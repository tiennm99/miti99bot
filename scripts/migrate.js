#!/usr/bin/env node
/**
 * @file migrate — custom D1 migration runner for per-module SQL files.
 *
 * Discovers all `src/modules/*/ migrations; /*.sql` files, sorts them
 * deterministically (by `{moduleName}/{filename}`), then applies each NEW
 * migration via `wrangler d1 execute miti99bot-db --remote --file=<path>`.
 *
 * Applied migrations are tracked in a `_migrations(name TEXT PRIMARY KEY,
 * applied_at INTEGER)` table in the D1 database itself.
 *
 * Flags:
 *   --dry-run   Print the migration plan without executing anything.
 *   --local     Apply against local dev D1 (omits --remote flag).
 *
 * Usage:
 *   node scripts/migrate.js
 *   node scripts/migrate.js --dry-run
 *   node scripts/migrate.js --local
 */

import { execSync } from "node:child_process";
import { existsSync, readdirSync } from "node:fs";
import { basename, join, resolve } from "node:path";

const DB_NAME = "miti99bot-db";
const PROJECT_ROOT = resolve(import.meta.dirname, "..");
const MODULES_DIR = join(PROJECT_ROOT, "src", "modules");

const dryRun = process.argv.includes("--dry-run");
const local = process.argv.includes("--local");
const remoteFlag = local ? "" : "--remote";

/**
 * Run a wrangler d1 execute command and return stdout.
 *
 * @param {string} sql — inline SQL string (used for bootstrap queries)
 * @param {string} [file] — path to a .sql file (mutually exclusive with sql)
 * @returns {string}
 */
function wranglerExecute(sql, file) {
  const target = file ? `--file="${file}"` : `--command="${sql.replace(/"/g, '\\"')}"`;
  const cmd = `npx wrangler d1 execute ${DB_NAME} ${remoteFlag} ${target} --json`;
  try {
    return execSync(cmd, { stdio: ["ignore", "pipe", "pipe"] }).toString();
  } catch (err) {
    const stderr = err.stderr?.toString() ?? "";
    const stdout = err.stdout?.toString() ?? "";
    throw new Error(`wrangler error:\n${stderr || stdout}`);
  }
}

/**
 * Ensure the _migrations table exists.
 */
function bootstrapMigrationsTable() {
  const sql =
    "CREATE TABLE IF NOT EXISTS _migrations (name TEXT PRIMARY KEY, applied_at INTEGER NOT NULL);";
  wranglerExecute(sql);
}

/**
 * Fetch already-applied migration names from D1.
 *
 * @returns {Set<string>}
 */
function getAppliedMigrations() {
  const out = wranglerExecute("SELECT name FROM _migrations;");
  /** @type {any[]} */
  let parsed = [];
  try {
    const json = JSON.parse(out);
    // wrangler --json wraps results in an array of result objects
    parsed = Array.isArray(json) ? (json[0]?.results ?? []) : [];
  } catch {
    // If the table is freshly created it may return empty JSON — treat as empty.
  }
  return new Set(parsed.map((r) => r.name));
}

/**
 * Record a migration as applied.
 *
 * @param {string} name
 */
function recordMigration(name) {
  const sql = `INSERT INTO _migrations (name, applied_at) VALUES ('${name}', ${Date.now()});`;
  wranglerExecute(sql);
}

/**
 * Discover all migration files as { name, absPath } sorted deterministically.
 * name = "{moduleName}/{filename}" — used as the unique migration key.
 *
 * @returns {Array<{name: string, absPath: string}>}
 */
function discoverMigrations() {
  if (!existsSync(MODULES_DIR)) return [];

  /** @type {Array<{name: string, absPath: string}>} */
  const found = [];

  for (const entry of readdirSync(MODULES_DIR, { withFileTypes: true })) {
    if (!entry.isDirectory()) continue;
    const migrationsDir = join(MODULES_DIR, entry.name, "migrations");
    if (!existsSync(migrationsDir)) continue;

    for (const file of readdirSync(migrationsDir).sort()) {
      if (!file.endsWith(".sql")) continue;
      found.push({
        name: `${entry.name}/${file}`,
        absPath: join(migrationsDir, file),
      });
    }
  }

  // Sort by the composite name so ordering is deterministic across modules.
  found.sort((a, b) => a.name.localeCompare(b.name));
  return found;
}

async function main() {
  const all = discoverMigrations();

  if (all.length === 0) {
    console.log("No migration files found — nothing to do.");
    return;
  }

  if (dryRun) {
    console.log(`DRY RUN — would apply up to ${all.length} migration(s):`);
    for (const m of all) console.log(`  ${m.name}`);
    return;
  }

  bootstrapMigrationsTable();
  const applied = getAppliedMigrations();

  const pending = all.filter((m) => !applied.has(m.name));

  if (pending.length === 0) {
    console.log(`All ${all.length} migration(s) already applied.`);
    return;
  }

  console.log(`Applying ${pending.length} pending migration(s)...`);

  for (const migration of pending) {
    console.log(`  → ${migration.name}`);
    wranglerExecute(null, migration.absPath);
    recordMigration(migration.name);
    console.log("    ✓ applied");
  }

  console.log("Done.");
}

main().catch((err) => {
  console.error(err.message ?? err);
  process.exit(1);
});
