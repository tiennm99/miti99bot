#!/usr/bin/env node
/**
 * @file wipe-mongo — rollback helper: delete all documents from every backfill
 * collection in MongoDB Atlas.
 *
 * WARNING: THIS IS IRREVERSIBLE. Run only when you want to start the backfill
 * from scratch (e.g. after discovering a systematic mapping bug).
 *
 * Flags:
 *   --yes  Skip the interactive confirmation prompt (for CI / scripted rollback).
 *          Even with --yes, prints a loud warning so logs capture intent.
 *
 * Required env: MONGODB_URI, MODULES (comma-separated KV module names)
 *
 * Usage:
 *   node --env-file-if-exists=.env.deploy scripts/wipe-mongo.js
 *   node --env-file-if-exists=.env.deploy scripts/wipe-mongo.js --yes
 */

import { createInterface } from "node:readline";
import { closeMongoClient, getMongoClient } from "./lib/migration-helpers.js";

const { MONGODB_URI, MODULES: MODULES_ENV } = process.env;
const skipPrompt = process.argv.includes("--yes");

function validateEnv() {
  const needed = { MONGODB_URI, MODULES: MODULES_ENV };
  const missing = Object.entries(needed)
    .filter(([, v]) => !v)
    .map(([k]) => k);
  if (missing.length) {
    console.error(`[wipe-mongo] Missing required env vars: ${missing.join(", ")}`);
    console.error("  Copy .env.deploy.example to .env.deploy and fill in values.");
    process.exit(1);
  }
}

/** @param {string} question @returns {Promise<string>} */
function prompt(question) {
  const rl = createInterface({ input: process.stdin, output: process.stdout });
  return new Promise((resolve) => {
    rl.question(question, (answer) => {
      rl.close();
      resolve(answer);
    });
  });
}

async function main() {
  validateEnv();

  // Build the list of collections: one per KV module + trading_trades.
  const kvModules = MODULES_ENV.split(",")
    .map((m) => m.trim())
    .filter(Boolean);
  const collections = [
    ...kvModules.map((m) => m.replace(/-/g, "_")), // mirrors mongo-kv-store.js normalization
    "trading_trades",
  ];

  console.error("╔══════════════════════════════════════════════════════╗");
  console.error("║  WARNING: wipe-mongo will DELETE ALL DOCUMENTS from  ║");
  console.error("║  the Atlas database and CANNOT BE UNDONE.            ║");
  console.error("╚══════════════════════════════════════════════════════╝");
  console.log(`Collections to wipe (${collections.length}): ${collections.join(", ")}`);

  if (!skipPrompt) {
    const answer = await prompt("\nType CONFIRM to wipe Atlas database miti99bot: ");
    if (answer.trim() !== "CONFIRM") {
      console.log("Aborted — nothing was deleted.");
      process.exit(0);
    }
  } else {
    console.error("[wipe-mongo] --yes passed: skipping interactive prompt. Proceeding with wipe.");
  }

  const client = await getMongoClient(MONGODB_URI);
  const db = client.db();

  let totalDeleted = 0;
  for (const name of collections) {
    const result = await db.collection(name).deleteMany({});
    console.log(`[wipe-mongo] ${name}: deleted ${result.deletedCount} document(s)`);
    totalDeleted += result.deletedCount;
  }

  await closeMongoClient();
  console.log(`[wipe-mongo] Done — ${totalDeleted} total document(s) removed.`);
}

main().catch((err) => {
  console.error("[wipe-mongo] Fatal:", err.message ?? err);
  process.exit(1);
});
