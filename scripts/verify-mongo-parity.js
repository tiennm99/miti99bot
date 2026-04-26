#!/usr/bin/env node
/**
 * @file verify-mongo-parity — cross-check KV / D1 data against MongoDB Atlas.
 *
 * Per KV module: count via CF REST vs Mongo countDocuments (±1% tolerance),
 * then value-compare via SHA256. Full-scan when total < 10 000; random-sample
 * N = min(500, ceil(sqrt(N))) otherwise. expiresAt checked within ±5 min.
 * For trading_trades: full-scan on legacy_id, ts, user_id, symbol, qty.
 *
 * Flags: --dry-run (counts only, no value compare)
 * Required env: MONGODB_URI, CLOUDFLARE_ACCOUNT_ID, CLOUDFLARE_API_TOKEN,
 *               KV_NAMESPACE_ID, MODULES
 * Exit: 0 all-pass | 1 any failure
 */

import { execSync } from "node:child_process";
import {
  cfKvGet,
  cfKvList,
  closeMongoClient,
  countDiffRatio,
  getMongoClient,
  sampleStrategy,
  sha256,
  sleep,
} from "./lib/migration-helpers.js";

const DB_NAME = "miti99bot-db";
const EXPIRES_TOL_MS = 5 * 60 * 1000; // ±5 minutes
const dryRun = process.argv.includes("--dry-run");
const {
  MONGODB_URI,
  CLOUDFLARE_ACCOUNT_ID,
  CLOUDFLARE_API_TOKEN,
  KV_NAMESPACE_ID,
  MODULES: MODULES_ENV,
} = process.env;
const toCollName = (mod) => mod.replace(/-/g, "_");
const redactKey = (k) => {
  const c = k.indexOf(":");
  return `${c >= 0 ? k.slice(0, c + 1) : ""}sha256:${sha256(k).slice(0, 8)}…`;
};

function validateEnv() {
  const missing = [
    "MONGODB_URI",
    "CLOUDFLARE_ACCOUNT_ID",
    "CLOUDFLARE_API_TOKEN",
    "KV_NAMESPACE_ID",
    "MODULES",
  ].filter((k) => !process.env[k]);
  if (missing.length) {
    console.error(`[verify] Missing env vars: ${missing.join(", ")}`);
    process.exit(1);
  }
}

function queryD1(sql) {
  const cmd = `npx wrangler d1 execute ${DB_NAME} --remote --command "${sql.replace(/"/g, '\\"')}" --json`;
  const parsed = JSON.parse(execSync(cmd, { stdio: ["ignore", "pipe", "pipe"] }).toString());
  return Array.isArray(parsed) ? (parsed[0]?.results ?? []) : [];
}

function shuffle(arr) {
  const a = [...arr];
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [a[i], a[j]] = [a[j], a[i]];
  }
  return a;
}

/** Enumerate ALL key objects for a module prefix via paginated CF REST. */
async function allKvKeys(mod) {
  const keys = [];
  let cursor = null;
  do {
    const page = await cfKvList(
      CLOUDFLARE_ACCOUNT_ID,
      KV_NAMESPACE_ID,
      CLOUDFLARE_API_TOKEN,
      `${mod}:`,
      cursor,
    );
    keys.push(...page.keys);
    cursor = page.list_complete ? null : page.cursor;
    if (cursor) await sleep(250);
  } while (cursor);
  return keys;
}

/**
 * Compare one KV key against its Mongo doc.
 * Returns null on match, mismatch description string on failure.
 *
 * @param {import("mongodb").Collection} coll
 * @param {{ name: string, metadata?: { expiration?: number } }} keyObj
 * @returns {Promise<string|null>}
 */
async function compareKey(coll, keyObj) {
  const value = await cfKvGet(
    CLOUDFLARE_ACCOUNT_ID,
    KV_NAMESPACE_ID,
    CLOUDFLARE_API_TOKEN,
    keyObj.name,
  );
  await sleep(5);
  const doc = await coll.findOne({ _id: keyObj.name });
  if (!doc) return "missing in Mongo";
  if (sha256(value) !== sha256(doc.value ?? "")) return "value hash mismatch";
  const kvExpMs = keyObj.metadata?.expiration ? keyObj.metadata.expiration * 1000 : null;
  const mgExpMs = doc.expiresAt ? new Date(doc.expiresAt).getTime() : null;
  if ((kvExpMs === null) !== (mgExpMs === null)) return "expiresAt presence mismatch";
  if (kvExpMs !== null && Math.abs(kvExpMs - mgExpMs) > EXPIRES_TOL_MS)
    return `expiresAt diff ${Math.abs(kvExpMs - mgExpMs)}ms`;
  return null;
}

async function verifyModule(db, mod) {
  const coll = db.collection(toCollName(mod));
  const kvKeys = await allKvKeys(mod);
  const nKv = kvKeys.length;
  const nMongo = await coll.countDocuments();
  const ratio = countDiffRatio(nKv, nMongo);
  const countOk = ratio < 0.01;
  console.log(
    `[${mod}] KV=${nKv} Mongo=${nMongo} diff=${(ratio * 100).toFixed(2)}% ${countOk ? "OK" : "FAIL"}`,
  );
  if (!countOk)
    return { pass: false, mismatches: [`count diff ${(ratio * 100).toFixed(2)}% > 1%`] };
  if (dryRun) return { pass: true, mismatches: [] };

  const { fullScan, sampleSize } = sampleStrategy(nKv);
  const sample = fullScan ? kvKeys : shuffle(kvKeys).slice(0, sampleSize);
  console.log(`[${mod}] ${fullScan ? "full-scan" : `sample ${sampleSize}/${nKv}`}`);

  const mismatches = [];
  for (const keyObj of sample) {
    const m = await compareKey(coll, keyObj);
    if (m) mismatches.push(`${redactKey(keyObj.name)}: ${m}`);
  }
  console.log(
    `[${mod}] value compare: ${mismatches.length === 0 ? "PASS" : `${mismatches.length} mismatch(es)`}`,
  );
  return { pass: mismatches.length === 0, mismatches };
}

async function verifyTrading(db) {
  const coll = db.collection("trading_trades");
  const rows = queryD1(
    "SELECT id, user_id, symbol, side, qty, price_vnd, ts FROM trading_trades ORDER BY id",
  );
  const nMongo = await coll.countDocuments();
  const ratio = countDiffRatio(rows.length, nMongo);
  const countOk = ratio < 0.01;
  console.log(
    `[trading] D1=${rows.length} Mongo=${nMongo} diff=${(ratio * 100).toFixed(2)}% ${countOk ? "OK" : "FAIL"}`,
  );
  if (!countOk) return { pass: false, mismatches: [`count diff ${(ratio * 100).toFixed(2)}%`] };
  if (dryRun) return { pass: true, mismatches: [] };

  const mismatches = [];
  for (const row of rows) {
    const doc = await coll.findOne({ legacy_id: row.id });
    if (!doc) {
      mismatches.push(`legacy_id=${row.id} missing`);
      continue;
    }
    for (const f of ["user_id", "symbol", "qty", "ts"]) {
      if (String(doc[f]) !== String(row[f])) mismatches.push(`legacy_id=${row.id} ${f} mismatch`);
    }
  }
  console.log(
    `[trading] full-scan: ${mismatches.length === 0 ? "PASS" : `${mismatches.length} mismatch(es)`}`,
  );
  return { pass: mismatches.length === 0, mismatches };
}

async function main() {
  validateEnv();
  if (dryRun) console.log("[verify] DRY RUN — counts only");
  const modules = MODULES_ENV.split(",")
    .map((m) => m.trim())
    .filter(Boolean);
  const db = (await getMongoClient(MONGODB_URI)).db();

  const report = [];
  let anyFail = false;
  for (const mod of modules) {
    const r = await verifyModule(db, mod);
    report.push({ mod, ...r });
    if (!r.pass) anyFail = true;
  }
  const tr = await verifyTrading(db);
  report.push({ mod: "trading", ...tr });
  if (!tr.pass) anyFail = true;

  await closeMongoClient();
  console.log("\n── Parity Report ─────────────────────────────");
  for (const r of report) {
    console.log(`  ${r.pass ? "PASS" : "FAIL"}  ${r.mod}`);
    for (const m of r.mismatches) console.log(`       mismatch: ${m}`);
  }
  console.log("──────────────────────────────────────────────");
  if (anyFail) {
    console.error("[verify] FAILED.");
    process.exit(1);
  }
  console.log("[verify] All modules PASS.");
}

main().catch((err) => {
  console.error("[verify] Fatal:", err.message ?? err);
  process.exit(1);
});
