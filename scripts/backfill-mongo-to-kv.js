#!/usr/bin/env node
/**
 * @file backfill-mongo-to-kv — emergency reverse-backfill: MongoDB → Cloudflare KV.
 *
 * Reads each per-module Mongo collection and writes every non-expired doc back
 * into CF KV via the REST API with correct TTL derived from expiresAt.
 *
 * Use this ONLY during a Stage-2 rollback. Operator must inform users that
 * N days of Mongo-only writes will revert (phase-07 debugger #14).
 *
 * Flags:
 *   --dry-run        Log summary without writing to KV.
 *   --module <name>  Restore only a single module (default: all).
 *
 * Required env (loaded via --env-file-if-exists=.env.deploy):
 *   MONGODB_URI, CLOUDFLARE_ACCOUNT_ID, CLOUDFLARE_API_TOKEN,
 *   KV_NAMESPACE_ID, MODULES (comma-separated)
 *
 * Usage:
 *   node --env-file-if-exists=.env.deploy scripts/backfill-mongo-to-kv.js
 *   node --env-file-if-exists=.env.deploy scripts/backfill-mongo-to-kv.js --dry-run
 *   node --env-file-if-exists=.env.deploy scripts/backfill-mongo-to-kv.js --module wordle
 */

import { cfKvPut, closeMongoClient, getMongoClient, sleep } from "./lib/migration-helpers.js";

// ─── Config ───────────────────────────────────────────────────────────────────

const {
  MONGODB_URI,
  CLOUDFLARE_ACCOUNT_ID,
  CLOUDFLARE_API_TOKEN,
  KV_NAMESPACE_ID,
  MODULES: MODULES_ENV,
} = process.env;

const dryRun = process.argv.includes("--dry-run");
const moduleFlag = (() => {
  const idx = process.argv.indexOf("--module");
  return idx !== -1 ? process.argv[idx + 1] : null;
})();

/** Throttle: 50 ops/sec → 20 ms between writes. */
const THROTTLE_MS = 20;

/** Minimum TTL accepted by CF KV REST API. */
const MIN_TTL_SECS = 60;

/** Normalize module name to Mongo collection name (mirrors mongo-kv-store.js). */
const toCollName = (mod) => mod.replace(/-/g, "_");

// ─── Preflight ────────────────────────────────────────────────────────────────

function validateEnv() {
  const missing = [
    "MONGODB_URI",
    "CLOUDFLARE_ACCOUNT_ID",
    "CLOUDFLARE_API_TOKEN",
    "KV_NAMESPACE_ID",
    "MODULES",
  ].filter((k) => !process.env[k]);
  if (missing.length) {
    console.error(`[backfill-mongo-kv] Missing required env vars: ${missing.join(", ")}`);
    console.error("  Copy .env.deploy.example → .env.deploy and fill in values.");
    process.exit(1);
  }
}

// ─── TTL computation ─────────────────────────────────────────────────────────

/**
 * Compute CF KV expirationTtl (seconds from now) from an absolute expiresAt Date.
 * Returns null if the doc has no TTL (key should be persistent).
 * Returns undefined (sentinel) if the doc is already expired (key must be SKIPPED).
 *
 * @param {Date|null|undefined} expiresAt
 * @param {number} nowMs — Date.now() at call time (injectable for tests)
 * @returns {{ ttl: number }|null|"expired"}
 */
export function computeTtl(expiresAt, nowMs = Date.now()) {
  if (!expiresAt) return null; // no TTL — write as persistent
  const remainingMs = expiresAt.getTime() - nowMs;
  if (remainingMs <= 0) return "expired"; // already past expiry → skip
  const secs = Math.floor(remainingMs / 1000);
  return { ttl: Math.max(MIN_TTL_SECS, secs) };
}

// ─── Per-module restore ───────────────────────────────────────────────────────

/**
 * Restore one module's Mongo collection back into CF KV.
 *
 * @param {import("mongodb").Db} db
 * @param {string} mod
 * @returns {Promise<{restored: number, skipped: number, failed: number}>}
 */
async function restoreModule(db, mod) {
  const coll = db.collection(toCollName(mod));
  const docs = await coll.find({}).toArray();

  let restored = 0;
  let skipped = 0;
  let failed = 0;

  for (const doc of docs) {
    const key = /** @type {string} */ (doc._id);
    const value = /** @type {string} */ (doc.value ?? "");
    const ttlResult = computeTtl(doc.expiresAt ?? null);

    if (ttlResult === "expired") {
      skipped++;
      continue; // expired in Mongo → do not surface a stale key in KV
    }

    if (dryRun) {
      restored++;
      continue;
    }

    try {
      /** @type {{ expirationTtl?: number }} */
      const opts = ttlResult ? { expirationTtl: ttlResult.ttl } : {};
      await cfKvPut(
        /** @type {string} */ (CLOUDFLARE_ACCOUNT_ID),
        /** @type {string} */ (KV_NAMESPACE_ID),
        /** @type {string} */ (CLOUDFLARE_API_TOKEN),
        key,
        value,
        opts,
      );
      restored++;
      await sleep(THROTTLE_MS);
    } catch (err) {
      failed++;
      // Log key hash only — never log plaintext keys (may encode user IDs).
      const { sha256 } = await import("./lib/migration-helpers.js");
      console.error(
        `[${mod}] ERROR key_sha256=${sha256(String(key)).slice(0, 16)}: ${/** @type {Error} */ (err).message}`,
      );
    }
  }

  return { restored, skipped, failed };
}

// ─── Main ─────────────────────────────────────────────────────────────────────

async function main() {
  validateEnv();

  const allModules = /** @type {string} */ (MODULES_ENV)
    .split(",")
    .map((m) => m.trim())
    .filter(Boolean);
  const modules = moduleFlag ? [moduleFlag] : allModules;

  if (moduleFlag && !allModules.includes(moduleFlag)) {
    console.error(
      `[backfill-mongo-kv] Unknown module "${moduleFlag}". Available: ${allModules.join(", ")}`,
    );
    process.exit(1);
  }

  if (dryRun) console.log("[backfill-mongo-kv] DRY RUN — no writes to KV");
  console.log(`[backfill-mongo-kv] Modules: ${modules.join(", ")}`);

  const client = await getMongoClient(/** @type {string} */ (MONGODB_URI));
  const db = client.db();

  let totalFailed = 0;
  for (const mod of modules) {
    const { restored, skipped, failed } = await restoreModule(db, mod);
    const verb = dryRun ? "(dry-run)" : `${restored} restored, ${skipped} skipped (expired)`;
    console.log(`[${mod}] ${docs_label(restored + skipped + failed)} docs: ${verb}`);
    totalFailed += failed;
  }

  await closeMongoClient();

  if (totalFailed > 0) {
    console.error(
      `[backfill-mongo-kv] Completed with ${totalFailed} failed write(s). Check logs above.`,
    );
    process.exit(1);
  }
  console.log("[backfill-mongo-kv] All modules restored.");
}

/** @param {number} n @returns {string} */
function docs_label(n) {
  return `${n} doc${n !== 1 ? "s" : ""}`;
}

// Run only when invoked directly (not when imported by tests).
const isMain = process.argv[1]?.endsWith("backfill-mongo-to-kv.js");
if (isMain) {
  main().catch((err) => {
    console.error("[backfill-mongo-kv] Fatal:", /** @type {Error} */ (err).message ?? err);
    process.exit(1);
  });
}
