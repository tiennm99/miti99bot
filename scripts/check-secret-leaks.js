#!/usr/bin/env node
/**
 * @file check-secret-leaks — fails build if any source file logs a known secret.
 *
 * Catches the common foot-gun where a developer prints `env.MONGODB_URI` or
 * similar during debugging and forgets to remove the line before commit. We
 * are NOT trying to be a full SAST — we just block obvious `console.log` /
 * `console.error` sites that interpolate a secret env var.
 *
 * Wired into `npm run lint` so every PR / pre-deploy run catches it.
 *
 * Patterns checked (add more as new secrets are introduced):
 *   - MONGODB_URI            — Atlas connection string (Phase 01)
 *   - TELEGRAM_BOT_TOKEN     — bot token from BotFather
 *   - TELEGRAM_WEBHOOK_SECRET — gates incoming webhook traffic
 *   - ADMIN_TOKEN            — kept for defense-in-depth even though Phase 05
 *                              redesign removed admin routes; cheap to leave in.
 *
 * Detection scope: any of these tokens appearing on the SAME line as a
 * `console.<level>(...)`, `JSON.stringify(env)`, or `throw new Error(...env...)`.
 *
 * Exit codes:
 *   0  — no leaks found
 *   1  — at least one leak detected (prints file:line + offending line)
 */

import { existsSync, readFileSync, readdirSync, statSync } from "node:fs";
import { extname, join, resolve } from "node:path";

const PROJECT_ROOT = resolve(import.meta.dirname, "..");
const SCAN_DIRS = ["src", "scripts"];
const SCAN_EXTS = new Set([".js", ".mjs", ".ts"]);

const SECRETS = [
  "MONGODB_URI",
  "TELEGRAM_BOT_TOKEN",
  "TELEGRAM_WEBHOOK_SECRET",
  "ADMIN_TOKEN",
  // Phase 05: CF API creds used by backfill scripts — defense-in-depth.
  "CLOUDFLARE_API_TOKEN",
  "CLOUDFLARE_ACCOUNT_ID",
];

// A line is suspicious if it both names a secret AND looks like it's emitting
// the value (console.*, JSON.stringify(env...), throw with interpolation).
const EMIT_PATTERNS = [
  /\bconsole\.(log|info|warn|error|debug|trace)\b/,
  /\bJSON\.stringify\s*\(\s*env\b/,
  /\bthrow\s+new\s+\w*Error\b/,
];

/**
 * Walk a directory tree and yield absolute file paths matching SCAN_EXTS.
 *
 * @param {string} dir
 * @returns {string[]}
 */
function walk(dir) {
  if (!existsSync(dir)) return [];
  const out = [];
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry);
    const st = statSync(full);
    if (st.isDirectory()) {
      if (entry === "node_modules" || entry === ".wrangler") continue;
      out.push(...walk(full));
    } else if (SCAN_EXTS.has(extname(entry))) {
      out.push(full);
    }
  }
  return out;
}

/**
 * Scan one file. Pushes hits to `findings`.
 *
 * @param {string} file
 * @param {Array<{file: string, line: number, secret: string, snippet: string}>} findings
 */
function scanFile(file, findings) {
  // Don't flag this file (it lists the patterns) or .example files.
  if (file.endsWith("check-secret-leaks.js")) return;

  const content = readFileSync(file, "utf8");
  const lines = content.split("\n");
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    if (!EMIT_PATTERNS.some((p) => p.test(line))) continue;
    for (const secret of SECRETS) {
      if (line.includes(secret)) {
        findings.push({
          file,
          line: i + 1,
          secret,
          snippet: line.trim(),
        });
      }
    }
  }
}

function main() {
  /** @type {Array<{file: string, line: number, secret: string, snippet: string}>} */
  const findings = [];

  for (const dir of SCAN_DIRS) {
    const abs = join(PROJECT_ROOT, dir);
    for (const file of walk(abs)) scanFile(file, findings);
  }

  if (findings.length === 0) {
    console.log(`secret-leak check: 0 findings across ${SCAN_DIRS.join(", ")}`);
    return;
  }

  console.error(`secret-leak check: ${findings.length} finding(s)`);
  for (const f of findings) {
    const rel = f.file.replace(`${PROJECT_ROOT}/`, "");
    console.error(`  ${rel}:${f.line}  [${f.secret}]  ${f.snippet}`);
  }
  process.exit(1);
}

main();
