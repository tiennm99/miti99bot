#!/usr/bin/env node
/**
 * @file analyze-soak — parse a CF Logs export and compute per-(cmd × cold/warm)
 * latency percentiles plus error counts for the 24h soak analysis.
 *
 * Usage:
 *   node scripts/analyze-soak.js --input soak-export.json [--commands /wordle,/loldle] [--output report.md]
 *
 * Input: CF Logs JSON export — one JSON object per line (NDJSON) OR a CSV export
 * where each row has at least a "message" column containing the raw JSON log string.
 *
 * Output (stdout + optional --output <md>):
 *   Markdown table: cmd | cold/warm | n | p50 | p95 | p99
 *   Error summary: dual-write secondary failures, mongo errors, CPU-time exceeded
 */

import { readFileSync, writeFileSync } from "node:fs";

// ── CLI arg parsing ──────────────────────────────────────────────────────────

function parseArgs(argv) {
  const args = {};
  for (let i = 2; i < argv.length; i++) {
    if (argv[i] === "--input") args.input = argv[++i];
    else if (argv[i] === "--commands") args.commands = argv[++i];
    else if (argv[i] === "--output") args.output = argv[++i];
  }
  return args;
}

// ── Percentile math ──────────────────────────────────────────────────────────

/**
 * Compute a percentile from a sorted numeric array (ascending).
 * Uses nearest-rank method.
 *
 * @param {number[]} sorted - must be sorted ascending
 * @param {number} p - percentile 0–100
 * @returns {number}
 */
export function percentile(sorted, p) {
  if (sorted.length === 0) return 0;
  if (sorted.length === 1) return sorted[0];
  const idx = Math.ceil((p / 100) * sorted.length) - 1;
  return sorted[Math.max(0, Math.min(idx, sorted.length - 1))];
}

// ── Log line parsing ─────────────────────────────────────────────────────────

/**
 * Try to extract a JSON object from a raw log line.
 * Handles NDJSON (line is the JSON) and CSV rows where the JSON lives inside
 * a quoted "message" column value.
 *
 * @param {string} line
 * @returns {object|null}
 */
export function parseLogLine(line) {
  const trimmed = line.trim();
  if (!trimmed) return null;

  // Try direct JSON parse (NDJSON format).
  if (trimmed.startsWith("{")) {
    try {
      return JSON.parse(trimmed);
    } catch {
      return null;
    }
  }

  // CSV: find the first {...} substring inside the line and parse it.
  const braceStart = trimmed.indexOf("{");
  const braceEnd = trimmed.lastIndexOf("}");
  if (braceStart !== -1 && braceEnd > braceStart) {
    try {
      return JSON.parse(trimmed.slice(braceStart, braceEnd + 1));
    } catch {
      return null;
    }
  }

  return null;
}

// ── Aggregation ──────────────────────────────────────────────────────────────

/**
 * @typedef {{ n: number, samples: number[] }} Bucket
 * @typedef {Map<string, Bucket>} BucketMap  key = "cmd|cold" | "cmd|warm"
 */

/**
 * Read all lines from a file path synchronously and return as array.
 *
 * @param {string} filePath
 * @returns {string[]}
 */
function readLines(filePath) {
  return readFileSync(filePath, "utf8").split("\n");
}

/**
 * Aggregate cmd_timing events from log lines into per-(cmd×cold/warm) buckets.
 *
 * @param {string[]} lines - raw log lines
 * @param {string[]|null} filterCmds - if set, only include these cmd names
 * @returns {{ buckets: BucketMap, errors: { dualWriteFail: number, mongoError: number, cpuExceeded: number } }}
 */
export function aggregateLines(lines, filterCmds) {
  /** @type {BucketMap} */
  const buckets = new Map();
  const errors = { dualWriteFail: 0, mongoError: 0, cpuExceeded: 0 };

  for (const line of lines) {
    // Count error patterns regardless of JSON structure.
    if (
      line.includes("dual-write:secondary:failed") ||
      line.includes("[dual-kv] secondary write failed")
    ) {
      errors.dualWriteFail++;
    }
    if (
      line.includes("MongoError") ||
      line.includes("mongo connection") ||
      line.includes("MongoNetworkError")
    ) {
      errors.mongoError++;
    }
    if (line.includes("Worker exceeded CPU time") || line.includes("cpu time exceeded")) {
      errors.cpuExceeded++;
    }

    const obj = parseLogLine(line);
    if (!obj || obj.event !== "cmd_timing") continue;

    const { cmd, total, cold } = obj;
    if (typeof cmd !== "string" || typeof total !== "number") continue;

    // Apply command filter.
    if (filterCmds && !filterCmds.includes(cmd)) continue;

    const bucket = cold ? "cold" : "warm";
    const key = `${cmd}|${bucket}`;
    if (!buckets.has(key)) buckets.set(key, { n: 0, samples: [] });
    const b = buckets.get(key);
    b.n++;
    b.samples.push(total);
  }

  return { buckets, errors };
}

// ── Report formatting ────────────────────────────────────────────────────────

/**
 * Build a markdown report string from aggregated data.
 *
 * @param {BucketMap} buckets
 * @param {{ dualWriteFail: number, mongoError: number, cpuExceeded: number }} errors
 * @returns {string}
 */
export function formatReport(buckets, errors) {
  const rows = [];

  for (const [key, bucket] of [...buckets.entries()].sort()) {
    const [cmd, coldWarm] = key.split("|");
    const sorted = [...bucket.samples].sort((a, b) => a - b);
    rows.push({
      cmd,
      coldWarm,
      n: bucket.n,
      p50: percentile(sorted, 50),
      p95: percentile(sorted, 95),
      p99: percentile(sorted, 99),
    });
  }

  const header = "| cmd | cold/warm | n | p50 | p95 | p99 |";
  const sep = "|-----|-----------|---|-----|-----|-----|";
  const dataRows = rows.map(
    (r) => `| ${r.cmd} | ${r.coldWarm} | ${r.n} | ${r.p50} | ${r.p95} | ${r.p99} |`,
  );

  const table = [header, sep, ...dataRows].join("\n");

  const errorSection = [
    "",
    "## Error Summary",
    `- Dual-write secondary failures: ${errors.dualWriteFail}`,
    `- Mongo connection errors: ${errors.mongoError}`,
    `- CPU time exceeded: ${errors.cpuExceeded}`,
  ].join("\n");

  return `## Soak Latency Report\n\n${table}\n${errorSection}\n`;
}

// ── Main ─────────────────────────────────────────────────────────────────────

async function main() {
  const args = parseArgs(process.argv);

  if (!args.input) {
    console.error(
      "Usage: node scripts/analyze-soak.js --input <path> [--commands /wordle,/loldle] [--output <md>]",
    );
    process.exit(1);
  }

  const filterCmds = args.commands ? args.commands.split(",").map((c) => c.trim()) : null;
  const lines = readLines(args.input);
  const { buckets, errors } = aggregateLines(lines, filterCmds);
  const report = formatReport(buckets, errors);

  process.stdout.write(report);

  if (args.output) {
    writeFileSync(args.output, report, "utf8");
    console.error(`\nReport written to ${args.output}`);
  }
}

// Run only when executed directly (not imported in tests).
const isMain = process.argv[1]?.endsWith("analyze-soak.js");
if (isMain) {
  main().catch((err) => {
    console.error(err);
    process.exit(1);
  });
}
