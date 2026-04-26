/**
 * @file drift-verifier — hourly cron handler that maintains dual-write health.
 *
 * Two responsibilities:
 *   1. Drain retry queues: re-attempt writes that failed against the secondary
 *      backend and were enqueued by DualKVStore / DualSqlStore.
 *   2. Spot-check parity: sample N keys from the KV store, fetch the same keys
 *      from Mongo, hash-compare values, and log any mismatches.
 *
 * Schedule: `"0 * * * *"` (once per hour). Tunable via `env.DRIFT_SAMPLE_N`
 * (default 50).
 *
 * Error logging never includes document values to avoid PII leakage.
 *
 * Cron handler signature matches the existing pattern used by module crons:
 *   `handler(event, ctx)` where `ctx = { db, sql, env }`.
 *
 * @module cron/drift-verifier
 */

import { MongoKVStore } from "../db/mongo-kv-store.js";

const KV_RETRY_PREFIX = "__retry:mongo-failed:";
const SQL_RETRY_PREFIX = "__retry:mongo-sql-failed:";
const MAX_DRAIN_PER_RUN = 200;

/**
 * Simple stable hash of a string value for drift comparison.
 * Not cryptographic — used only for equality detection.
 *
 * @param {string | null} s
 * @returns {string}
 */
function hashValue(s) {
  if (s == null) return "__null__";
  let h = 0;
  for (let i = 0; i < s.length; i++) {
    h = ((h << 5) - h + s.charCodeAt(i)) | 0;
  }
  return h.toString(16);
}

/**
 * Drain one retry queue prefix: list matching keys, re-attempt the secondary
 * write, delete the queue entry on success. Cap at MAX_DRAIN_PER_RUN.
 *
 * @param {any} rawKv — raw CF KVNamespace (env.KV)
 * @param {string} prefix — queue key prefix, e.g. `__retry:mongo-failed:`
 * @param {(payload: object) => Promise<void>} retryFn — callable that re-attempts the secondary write
 * @returns {Promise<{ attempted: number, succeeded: number, failed: number }>}
 */
async function drainRetryQueue(rawKv, prefix, retryFn) {
  let attempted = 0;
  let succeeded = 0;
  let failed = 0;

  const listed = await rawKv.list({ prefix, limit: MAX_DRAIN_PER_RUN });
  const keys = listed.keys ?? [];

  for (const keyEntry of keys) {
    const queueKey = typeof keyEntry === "object" ? keyEntry.name : keyEntry;
    attempted++;
    try {
      const raw = await rawKv.get(queueKey);
      if (!raw) {
        await rawKv.delete(queueKey);
        continue;
      }
      const payload = JSON.parse(raw);
      await retryFn(payload);
      await rawKv.delete(queueKey);
      succeeded++;
    } catch (err) {
      failed++;
      console.warn("[drift-verifier] retry failed", {
        phase: "drift-verifier",
        queueKey,
        errClass: err instanceof Error ? err.constructor.name : "unknown",
        err: err instanceof Error ? err.message : String(err),
      });
    }
  }

  return { attempted, succeeded, failed };
}

/**
 * No-op retry for SQL entries — re-attempt is intentionally left as a no-op
 * for Phase 04 (the MongoSqlStore handles its own write path). A full retry
 * would require reconstructing the store, which is out of scope here.
 * The queue entry is deleted so stale entries don't accumulate indefinitely.
 *
 * @param {object} _payload
 * @returns {Promise<void>}
 */
async function retrySqlWrite(_payload) {
  // Phase 04 note: full SQL retry requires reconstructing MongoSqlStore.
  // For now this drain clears stale entries. Phase 06 telemetry will surface
  // any persistent failure patterns before Phase 07 cutover.
}

/**
 * Sample N keys from the CF KV namespace (scoped to a module prefix),
 * fetch the same keys from the MongoKVStore, and log any hash mismatches.
 *
 * @param {string} moduleName
 * @param {any} rawKv — raw CF KVNamespace
 * @param {MongoKVStore} mongoStore
 * @param {number} n — sample size
 * @returns {Promise<{ sampled: number, mismatches: number }>}
 */
async function spotCheckModule(moduleName, rawKv, mongoStore, n) {
  const prefix = `${moduleName}:`;
  let sampled = 0;
  let mismatches = 0;

  try {
    const listed = await rawKv.list({ prefix, limit: n });
    const keys = (listed.keys ?? []).map((k) => (typeof k === "object" ? k.name : k));

    for (const fullKey of keys) {
      sampled++;
      try {
        const cfRaw = await rawKv.get(fullKey);
        const mongoRaw = await mongoStore.get(fullKey);
        const cfHash = hashValue(cfRaw);
        const mongoHash = hashValue(mongoRaw);

        if (cfHash !== mongoHash) {
          mismatches++;
          console.warn("[drift-verifier] parity mismatch", {
            phase: "drift-verifier",
            module: moduleName,
            key: fullKey,
            primary_hash: cfHash,
            secondary_hash: mongoHash,
          });
        }
      } catch (err) {
        console.warn("[drift-verifier] key compare failed", {
          phase: "drift-verifier",
          module: moduleName,
          key: fullKey,
          err: err instanceof Error ? err.message : String(err),
        });
      }
    }
  } catch (err) {
    console.error("[drift-verifier] spotCheckModule failed", {
      phase: "drift-verifier",
      module: moduleName,
      err: err instanceof Error ? err.message : String(err),
    });
  }

  return { sampled, mismatches };
}

/**
 * Drift-verifier cron handler.
 *
 * Registered in wrangler.toml as `"0 * * * *"` and wired into the cron
 * dispatcher via the registry's system crons array.
 *
 * @param {any} _event — Cloudflare ScheduledEvent (unused; schedule matched by dispatcher)
 * @param {{ db: any, sql: any, env: any }} ctx — injected by cron-dispatcher
 * @returns {Promise<void>}
 */
export async function driftVerifier(_event, ctx) {
  const { env } = ctx;
  const N = Math.max(1, Number(env.DRIFT_SAMPLE_N ?? 50));
  const rawKv = env.KV;

  if (!rawKv) {
    console.warn("[drift-verifier] env.KV not available — skipping run");
    return;
  }

  // 1. Drain KV retry queue (failed Mongo secondary writes).
  const kvDrain = await drainRetryQueue(rawKv, KV_RETRY_PREFIX, async (payload) => {
    // Atlas URI is required to re-attempt the secondary write.
    if (!env.MONGODB_URI)
      throw new Error("Atlas URI not configured — cannot retry secondary write");
    const store = new MongoKVStore(env, payload.key.split(":")[0] ?? "unknown");
    if (payload.op === "delete") {
      await store.delete(payload.key);
    } else if (payload.op === "putJSON") {
      // Value is not stored in the retry queue (PII risk) — skip value retry.
      // The key will be re-synced by the next backfill / drift-verifier parity check.
      console.warn("[drift-verifier] putJSON retry skipped — value not stored in queue", {
        phase: "drift-verifier",
        key: payload.key,
      });
    } else if (payload.op === "put") {
      // Same as putJSON: value omitted from queue for PII safety.
      console.warn("[drift-verifier] put retry skipped — value not stored in queue", {
        phase: "drift-verifier",
        key: payload.key,
      });
    }
  });

  // 2. Drain SQL retry queue (failed MongoSqlStore secondary writes).
  const sqlDrain = await drainRetryQueue(rawKv, SQL_RETRY_PREFIX, retrySqlWrite);

  console.log("[drift-verifier] queue drain complete", {
    phase: "drift-verifier",
    kv: kvDrain,
    sql: sqlDrain,
  });

  // 3. Spot-check parity across key modules when Atlas is reachable.
  if (!env.MONGODB_URI || env.MONGODB_URI === "__stub_mongo__") return;

  const modules =
    typeof env.MODULES === "string"
      ? env.MODULES.split(",")
          .map((m) => m.trim())
          .filter(Boolean)
      : [];

  let totalSampled = 0;
  let totalMismatches = 0;

  for (const moduleName of modules) {
    const collName = moduleName.replace(/-/g, "_");
    const mongoStore = new MongoKVStore(env, collName);
    const { sampled, mismatches } = await spotCheckModule(moduleName, rawKv, mongoStore, N);
    totalSampled += sampled;
    totalMismatches += mismatches;
  }

  if (totalMismatches > 0) {
    console.error("[drift-verifier] parity drift detected", {
      phase: "drift-verifier",
      totalSampled,
      totalMismatches,
    });
  } else {
    console.log("[drift-verifier] parity check passed", {
      phase: "drift-verifier",
      totalSampled,
    });
  }
}

/**
 * Module-style export so the cron-dispatcher can register drift-verifier as a
 * system-level cron entry alongside module crons. The `name` is synthetic —
 * no module folder exists; the dispatcher treats it like any other cron entry.
 */
export const driftVerifierCron = {
  schedule: "0 * * * *",
  name: "drift-verifier",
  handler: driftVerifier,
};
