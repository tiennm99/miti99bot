#!/usr/bin/env node
/**
 * @file backfill-kv-to-mongo — copy historical KV data into MongoDB Atlas.
 *
 * Uses the CF KV REST API to enumerate keys per module, then upserts each
 * key into the corresponding Mongo collection using $setOnInsert (skip-if-exists)
 * so live dual-write data written after Phase 04 is never overwritten.
 *
 * Flags:
 *   --dry-run        List + count without writing to Mongo.
 *   --module <name>  Backfill only a single module (default: all).
 *
 * Required env (loaded via --env-file-if-exists=.env.deploy):
 *   MONGODB_URI, CLOUDFLARE_ACCOUNT_ID, CLOUDFLARE_API_TOKEN,
 *   KV_NAMESPACE_ID, MODULES (comma-separated)
 *
 * Usage:
 *   node --env-file-if-exists=.env.deploy scripts/backfill-kv-to-mongo.js
 *   node --env-file-if-exists=.env.deploy scripts/backfill-kv-to-mongo.js --dry-run
 *   node --env-file-if-exists=.env.deploy scripts/backfill-kv-to-mongo.js --module wordle
 */

import {
  cfKvGet,
  cfKvList,
  clearCheckpoint,
  closeMongoClient,
  getMongoClient,
  loadCheckpoint,
  saveCheckpoint,
  sleep,
} from "./lib/migration-helpers.js";

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

/** Normalize module name to MongoDB collection name (mirrors mongo-kv-store.js). */
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
    console.error(`[backfill-kv] Missing required env vars: ${missing.join(", ")}`);
    console.error("  Copy .env.deploy.example → .env.deploy and fill in values.");
    process.exit(1);
  }
}

// ─── Per-module backfill ──────────────────────────────────────────────────────

/**
 * Backfill one module from KV into Mongo.
 *
 * @param {import("mongodb").Db} db
 * @param {string} mod — module name (e.g. "wordle", "loldle-emoji")
 * @returns {Promise<{copied: number, skipped: number, failed: number}>}
 */
async function backfillModule(db, mod) {
  const coll = db.collection(toCollName(mod));
  const prefix = `${mod}:`;
  const cp = loadCheckpoint(mod);
  let cursor = cp?.cursor ?? null;
  let page = 0;
  let copied = 0;
  let skipped = 0;
  let failed = 0;

  if (cp)
    console.log(
      `[${mod}] resuming from checkpoint (cursor=${cursor ? cursor.slice(0, 12) : "start"})`,
    );

  let hasMore = true;
  while (hasMore) {
    page++;
    const {
      keys,
      cursor: nextCursor,
      list_complete,
    } = await cfKvList(
      CLOUDFLARE_ACCOUNT_ID,
      KV_NAMESPACE_ID,
      CLOUDFLARE_API_TOKEN,
      prefix,
      cursor,
    );

    for (const keyObj of keys) {
      const key = keyObj.name;
      // expiresAt: KV metadata.expiration is unix-seconds → convert to JS Date.
      const expSecs = keyObj.metadata?.expiration;
      const expiresAt = expSecs ? new Date(expSecs * 1000) : undefined;

      if (dryRun) {
        copied++; // count as "would copy" in dry-run
        continue;
      }

      try {
        const value = await cfKvGet(
          CLOUDFLARE_ACCOUNT_ID,
          KV_NAMESPACE_ID,
          CLOUDFLARE_API_TOKEN,
          key,
        );

        const doc = { value };
        if (expiresAt) doc.expiresAt = expiresAt;

        // $setOnInsert: skip-if-exists — preserves any newer Mongo state from dual-write.
        const result = await coll.updateOne({ _id: key }, { $setOnInsert: doc }, { upsert: true });
        if (result.upsertedCount > 0) {
          copied++;
        } else {
          skipped++;
        }
        await sleep(20); // throttle: ~50 ops/sec to stay within Atlas M0 headroom
      } catch (err) {
        failed++;
        console.error(`[${mod}] ERROR key="${key.slice(0, 40)}…": ${err.message}`);
      }
    }

    // Checkpoint after each page so a crash doesn't lose progress.
    saveCheckpoint(mod, { cursor: nextCursor, lastKey: keys.at(-1)?.name ?? null });

    const verb = dryRun ? "(dry-run)" : `${copied} copied, ${skipped} skipped, ${failed} failed`;
    console.log(`[${mod}] page ${page}: ${keys.length} keys ${verb}`);

    cursor = nextCursor;
    hasMore = !list_complete && !!cursor;

    // Brief pause between pages to respect CF REST rate limits (1200 req/5 min).
    if (hasMore) await sleep(250);
  }

  if (!dryRun) clearCheckpoint(mod);
  return { copied, skipped, failed };
}

// ─── Main ─────────────────────────────────────────────────────────────────────

async function main() {
  validateEnv();

  const allModules = MODULES_ENV.split(",")
    .map((m) => m.trim())
    .filter(Boolean);
  const modules = moduleFlag ? [moduleFlag] : allModules;

  if (moduleFlag && !allModules.includes(moduleFlag)) {
    console.error(
      `[backfill-kv] Unknown module "${moduleFlag}". Available: ${allModules.join(", ")}`,
    );
    process.exit(1);
  }

  if (dryRun) console.log("[backfill-kv] DRY RUN — no writes will be made");
  console.log(`[backfill-kv] Modules: ${modules.join(", ")}`);

  const client = await getMongoClient(MONGODB_URI);
  const db = client.db();

  let totalFailed = 0;
  for (const mod of modules) {
    const { copied, skipped, failed } = await backfillModule(db, mod);
    console.log(`[${mod}] DONE — ${copied} copied, ${skipped} skipped, ${failed} failed`);
    totalFailed += failed;
  }

  await closeMongoClient();
  if (totalFailed > 0) {
    console.error(`[backfill-kv] Completed with ${totalFailed} failed key(s). Check logs above.`);
    process.exit(1);
  }
  console.log("[backfill-kv] All modules complete.");
}

main().catch((err) => {
  console.error("[backfill-kv] Fatal:", err.message ?? err);
  process.exit(1);
});
