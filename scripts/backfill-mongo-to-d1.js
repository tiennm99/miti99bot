#!/usr/bin/env node
/**
 * @file backfill-mongo-to-d1 — emergency reverse-backfill: MongoDB → Cloudflare D1.
 *
 * Reads trading_trades from Mongo (sorted by legacy_id) and writes them back
 * into D1 via `wrangler d1 execute --remote --file=<tmp.sql>`.
 *
 * Preserves legacy_id when present (written by phase-05 forward backfill).
 * When legacy_id is absent, generates sequential IDs from max(existing_d1_id)+1.
 *
 * Use this ONLY during a Stage-2 rollback (phase-07 debugger #14).
 * Operator MUST inform users that N days of Mongo-only writes will revert.
 *
 * Flags:
 *   --dry-run  Print SQL to stdout; do not execute wrangler.
 *   --force    Proceed even if D1 trading_trades already has rows.
 *
 * Required env (loaded via --env-file-if-exists=.env.deploy):
 *   MONGODB_URI
 *
 * Usage:
 *   node --env-file-if-exists=.env.deploy scripts/backfill-mongo-to-d1.js
 *   node --env-file-if-exists=.env.deploy scripts/backfill-mongo-to-d1.js --dry-run
 *   node --env-file-if-exists=.env.deploy scripts/backfill-mongo-to-d1.js --force
 */

import { execSync } from "node:child_process";
import { rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { closeMongoClient, getMongoClient } from "./lib/migration-helpers.js";

const DB_NAME = "miti99bot-db";
const COLLECTION = "trading_trades";

const dryRun = process.argv.includes("--dry-run");
const force = process.argv.includes("--force");

// ─── Preflight ────────────────────────────────────────────────────────────────

function validateEnv() {
  const missingVars = [["MONGODB_URI", "Atlas connection string"]].filter(([k]) => !process.env[k]);
  if (missingVars.length) {
    for (const [k, desc] of missingVars)
      console.error(`[backfill-mongo-d1] Missing: ${k} (${desc})`);
    console.error("  Copy .env.deploy.example → .env.deploy and fill in values.");
    process.exit(1);
  }
}

// ─── D1 helpers ───────────────────────────────────────────────────────────────

/**
 * Execute a single SQL command against remote D1 and return parsed rows.
 *
 * @param {string} sql
 * @returns {any[]}
 */
function queryD1(sql) {
  const cmd = `npx wrangler d1 execute ${DB_NAME} --remote --command "${sql.replace(/"/g, '\\"')}" --json`;
  let stdout;
  try {
    stdout = execSync(cmd, { stdio: ["ignore", "pipe", "pipe"] }).toString();
  } catch (err) {
    const stderr = /** @type {any} */ (err).stderr?.toString() ?? "";
    throw new Error(
      `wrangler d1 execute failed:\n${stderr || /** @type {any} */ (err).stdout?.toString()}`,
    );
  }
  const parsed = JSON.parse(stdout);
  return Array.isArray(parsed) ? (parsed[0]?.results ?? []) : [];
}

/**
 * Execute a SQL file against remote D1 via wrangler.
 *
 * @param {string} filePath — absolute path to .sql file
 */
function executeD1File(filePath) {
  const cmd = `npx wrangler d1 execute ${DB_NAME} --remote --file="${filePath}" --json`;
  try {
    execSync(cmd, { stdio: ["ignore", "pipe", "pipe"] });
  } catch (err) {
    const stderr = /** @type {any} */ (err).stderr?.toString() ?? "";
    throw new Error(
      `wrangler d1 execute failed:\n${stderr || /** @type {any} */ (err).stdout?.toString()}`,
    );
  }
}

// ─── SQL generation ──────────────────────────────────────────────────────────

/**
 * Escape a SQL string value (single-quote doubling).
 *
 * @param {string|null|undefined} v
 * @returns {string} SQL literal (including surrounding quotes)
 */
export function sqlStr(v) {
  if (v == null) return "NULL";
  return `'${String(v).replace(/'/g, "''")}'`;
}

/**
 * Build an INSERT statement for one trading trade row.
 *
 * @param {{
 *   id: number,
 *   user_id: string,
 *   symbol: string,
 *   side: string,
 *   qty: number,
 *   price_vnd: number,
 *   ts: number
 * }} row
 * @returns {string}
 */
export function buildInsertSql(row) {
  return (
    `INSERT INTO ${COLLECTION} (id, user_id, symbol, side, qty, price_vnd, ts) VALUES (` +
    [
      row.id,
      sqlStr(row.user_id),
      sqlStr(row.symbol),
      sqlStr(row.side),
      row.qty,
      row.price_vnd,
      row.ts,
    ].join(", ") +
    ");"
  );
}

// ─── Main ─────────────────────────────────────────────────────────────────────

async function main() {
  validateEnv();

  if (dryRun) console.log("[backfill-mongo-d1] DRY RUN — SQL will be printed to stdout");

  const client = await getMongoClient(/** @type {string} */ (process.env.MONGODB_URI));
  const db = client.db();
  const coll = db.collection(COLLECTION);

  // Sort by legacy_id ascending so we maintain original D1 row order.
  const docs = await coll.find({}).sort({ legacy_id: 1 }).toArray();
  console.log(`[backfill-mongo-d1] Found ${docs.length} document(s) in Mongo ${COLLECTION}`);

  if (docs.length === 0) {
    console.log("[backfill-mongo-d1] Nothing to restore.");
    await closeMongoClient();
    return;
  }

  if (!dryRun) {
    // Pre-flight: abort if D1 already has rows unless --force.
    const existingRows = queryD1(`SELECT COUNT(*) AS cnt FROM ${COLLECTION}`);
    const existingCount = existingRows[0]?.cnt ?? 0;
    if (existingCount > 0 && !force) {
      console.error(
        `[backfill-mongo-d1] ABORT: D1 ${COLLECTION} already has ${existingCount} row(s).`,
      );
      console.error("  Run with --force to bypass (e.g. after wiping D1 manually).");
      await closeMongoClient();
      process.exit(1);
    }
    if (existingCount > 0 && force) {
      console.log(
        `[backfill-mongo-d1] --force: proceeding despite ${existingCount} existing D1 row(s).`,
      );
    }
  }

  // Determine starting ID for docs without a legacy_id.
  let nextId = 1;
  const hasLegacyIds = docs.some((d) => d.legacy_id != null);
  if (!dryRun && !hasLegacyIds) {
    // Fetch max id from D1 to avoid collisions.
    const maxRows = queryD1(`SELECT MAX(id) AS m FROM ${COLLECTION}`);
    const maxId = maxRows[0]?.m ?? 0;
    nextId = maxId + 1;
  }

  // Build SQL statements.
  /** @type {string[]} */
  const statements = [];
  for (const doc of docs) {
    const id = doc.legacy_id != null ? Number(doc.legacy_id) : nextId++;
    statements.push(
      buildInsertSql({
        id,
        user_id: String(doc.user_id ?? ""),
        symbol: String(doc.symbol ?? ""),
        side: String(doc.side ?? ""),
        qty: Number(doc.qty ?? 0),
        price_vnd: Number(doc.price_vnd ?? 0),
        ts: Number(doc.ts ?? 0),
      }),
    );
  }

  if (dryRun) {
    console.log("[backfill-mongo-d1] Generated SQL:");
    for (const stmt of statements) console.log(stmt);
    console.log(`[backfill-mongo-d1] DRY RUN complete — ${statements.length} statement(s).`);
    await closeMongoClient();
    return;
  }

  // Write SQL to a temp file and pipe through wrangler.
  const tmpFile = join(tmpdir(), `backfill-mongo-d1-${Date.now()}.sql`);
  try {
    writeFileSync(tmpFile, statements.join("\n"), "utf8");
    console.log(`[backfill-mongo-d1] Executing ${statements.length} INSERT(s) via wrangler...`);
    executeD1File(tmpFile);
    console.log(`[backfill-mongo-d1] Done — ${statements.length} row(s) restored to D1.`);
  } finally {
    try {
      rmSync(tmpFile);
    } catch {
      // Non-fatal cleanup.
    }
  }

  await closeMongoClient();
}

// Run only when invoked directly (not when imported by tests).
const isMain = process.argv[1]?.endsWith("backfill-mongo-to-d1.js");
if (isMain) {
  main().catch((err) => {
    console.error("[backfill-mongo-d1] Fatal:", /** @type {Error} */ (err).message ?? err);
    process.exit(1);
  });
}
