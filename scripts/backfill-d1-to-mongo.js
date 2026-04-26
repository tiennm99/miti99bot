#!/usr/bin/env node
/**
 * @file backfill-d1-to-mongo — copy trading_trades from D1 into MongoDB Atlas.
 *
 * Uses `npx wrangler d1 execute --remote --json` (mirrors scripts/migrate.js pattern)
 * to read all rows, then insertMany in batches of 100 into the `trading_trades`
 * Mongo collection.
 *
 * Flags:
 *   --dry-run  Count rows + log first 5 without writing.
 *   --force    Bypass pre-flight abort when trading_trades collection is non-empty.
 *
 * Required env (loaded via --env-file-if-exists=.env.deploy):
 *   MONGODB_URI
 *
 * Usage:
 *   node --env-file-if-exists=.env.deploy scripts/backfill-d1-to-mongo.js
 *   node --env-file-if-exists=.env.deploy scripts/backfill-d1-to-mongo.js --dry-run
 *   node --env-file-if-exists=.env.deploy scripts/backfill-d1-to-mongo.js --force
 */

import { execSync } from "node:child_process";
import { ObjectId } from "mongodb";
import { closeMongoClient, getMongoClient, sleep } from "./lib/migration-helpers.js";

const DB_NAME = "miti99bot-db";
const COLLECTION = "trading_trades";
const BATCH_SIZE = 100;
const BATCH_SLEEP_MS = 100; // brief pause between batches

const dryRun = process.argv.includes("--dry-run");
const force = process.argv.includes("--force");

// ─── Preflight ────────────────────────────────────────────────────────────────

function validateEnv() {
  const missingVars = [["MONGODB_URI", "Atlas connection string"]].filter(([k]) => !process.env[k]);
  if (missingVars.length) {
    for (const [k, desc] of missingVars) console.error(`[backfill-d1] Missing: ${k} (${desc})`);
    console.error("  Copy .env.deploy.example to .env.deploy and fill in values.");
    process.exit(1);
  }
}

// ─── D1 query ─────────────────────────────────────────────────────────────────

/**
 * Execute a SQL query against the remote D1 database via wrangler.
 * Mirrors the wranglerExecute pattern in scripts/migrate.js.
 *
 * @param {string} sql
 * @returns {any[]} rows from D1
 */
function queryD1(sql) {
  const cmd = `npx wrangler d1 execute ${DB_NAME} --remote --command "${sql.replace(/"/g, '\\"')}" --json`;
  let stdout;
  try {
    stdout = execSync(cmd, { stdio: ["ignore", "pipe", "pipe"] }).toString();
  } catch (err) {
    const stderr = err.stderr?.toString() ?? "";
    throw new Error(`wrangler d1 execute failed:\n${stderr || err.stdout?.toString()}`);
  }
  // wrangler --json wraps results: [{results: [...], success: bool}]
  const parsed = JSON.parse(stdout);
  const results = Array.isArray(parsed) ? (parsed[0]?.results ?? []) : [];
  return results;
}

// ─── Row mapping ──────────────────────────────────────────────────────────────

/**
 * Map a D1 row to a Mongo document.
 * Preserves legacy_id so round-trip verification can match records.
 *
 * @param {{ id: number, user_id: string, symbol: string, side: string,
 *           qty: number, price_vnd: number, ts: number }} row
 * @returns {object}
 */
function rowToDoc(row) {
  return {
    _id: new ObjectId(),
    legacy_id: row.id,
    user_id: row.user_id,
    symbol: row.symbol,
    side: row.side,
    qty: row.qty,
    price_vnd: row.price_vnd,
    ts: row.ts,
  };
}

// ─── Main ─────────────────────────────────────────────────────────────────────

async function main() {
  validateEnv();

  if (dryRun) console.log("[backfill-d1] DRY RUN — no writes will be made");

  // Fetch all rows from D1.
  console.log("[backfill-d1] Querying D1 trading_trades...");
  const rows = queryD1(
    "SELECT id, user_id, symbol, side, qty, price_vnd, ts FROM trading_trades ORDER BY id",
  );
  console.log(`[backfill-d1] Found ${rows.length} rows in D1`);

  if (dryRun) {
    console.log("[backfill-d1] Sample (first 5 rows):");
    for (const row of rows.slice(0, 5)) {
      // Log structural info only — no PII values printed.
      console.log(`  id=${row.id} symbol=${row.symbol} side=${row.side} ts=${row.ts}`);
    }
    console.log("[backfill-d1] DRY RUN complete. No writes made.");
    return;
  }

  const client = await getMongoClient(process.env.MONGODB_URI);
  const db = client.db();
  const coll = db.collection(COLLECTION);

  // Pre-flight: abort if collection already has data (prevents double-insert).
  const existingCount = await coll.countDocuments();
  if (existingCount > 0 && !force) {
    console.error(`[backfill-d1] ABORT: ${COLLECTION} already has ${existingCount} document(s).`);
    console.error("  Run with --force to bypass this check (e.g. after wipe-mongo).");
    await closeMongoClient();
    process.exit(1);
  }
  if (existingCount > 0 && force) {
    console.log(`[backfill-d1] --force: proceeding despite ${existingCount} existing doc(s).`);
  }

  // Insert in batches of BATCH_SIZE.
  let inserted = 0;
  for (let i = 0; i < rows.length; i += BATCH_SIZE) {
    const batch = rows.slice(i, i + BATCH_SIZE).map(rowToDoc);
    await coll.insertMany(batch);
    inserted += batch.length;
    console.log(`[backfill-d1] Inserted ${inserted}/${rows.length}`);
    if (i + BATCH_SIZE < rows.length) await sleep(BATCH_SLEEP_MS);
  }

  await closeMongoClient();
  console.log(`[backfill-d1] Done — ${inserted} documents inserted into ${COLLECTION}.`);
}

main().catch((err) => {
  console.error("[backfill-d1] Fatal:", err.message ?? err);
  process.exit(1);
});
