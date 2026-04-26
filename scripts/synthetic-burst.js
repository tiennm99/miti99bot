#!/usr/bin/env node
/**
 * @file synthetic-burst — fire N parallel requests at the deployed Worker URL
 * to exercise the M0 Atlas connection cap before a live deploy.
 *
 * Each request is a synthetic Telegram webhook update POST that grammY will
 * route to the specified command handler. All requests hit the Worker
 * simultaneously (Promise.all) to maximise cold-isolate spawning.
 *
 * Usage:
 *   node scripts/synthetic-burst.js \
 *     --url https://miti99bot.workers.dev \
 *     --secret <X-Telegram-Bot-Api-Secret-Token> \
 *     [--n 20] \
 *     [--cmd /wordle]
 *
 * No unit tests for this script — it is network-touching by design and only
 * runs against a live deployed Worker. Tested manually pre-deploy.
 *
 * Abort guideline (debugger #2): if Atlas connection peak > 300/500 (60% cap),
 * do NOT proceed with live deploy. Check Atlas UI during the 60s after the burst.
 */

// ── CLI arg parsing ──────────────────────────────────────────────────────────

function parseArgs(argv) {
  const args = { n: 20, cmd: "/wordle" };
  for (let i = 2; i < argv.length; i++) {
    if (argv[i] === "--url") args.url = argv[++i];
    else if (argv[i] === "--secret") args.secret = argv[++i];
    else if (argv[i] === "--n") args.n = Number.parseInt(argv[++i], 10);
    else if (argv[i] === "--cmd") args.cmd = argv[++i];
  }
  return args;
}

// ── Synthetic Telegram update payload ───────────────────────────────────────

/**
 * Build a minimal but valid grammY-shaped Telegram Update object for a
 * bot_command message.
 *
 * @param {string} cmd - e.g. "/wordle"
 * @param {number} index - used to differentiate update_id + message_id values
 * @returns {object}
 */
function buildUpdate(cmd, index) {
  return {
    update_id: 100000 + index,
    message: {
      message_id: 200000 + index,
      from: { id: 1, is_bot: false, first_name: "Burst" },
      chat: { id: 1, type: "private" },
      date: Math.floor(Date.now() / 1000),
      text: cmd,
      entities: [{ type: "bot_command", offset: 0, length: cmd.length }],
    },
  };
}

// ── Single request ───────────────────────────────────────────────────────────

/**
 * POST one synthetic update to the Worker webhook endpoint.
 *
 * @param {string} url - Worker base URL
 * @param {string} secret - X-Telegram-Bot-Api-Secret-Token value
 * @param {object} update - Telegram Update payload
 * @param {number} index - request index for logging
 * @returns {Promise<{ index: number, status: number, ms: number, error?: string }>}
 */
async function sendUpdate(url, secret, update, index) {
  const t0 = Date.now();
  const endpoint = url.replace(/\/$/, "") + "/webhook";

  try {
    const res = await fetch(endpoint, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Telegram-Bot-Api-Secret-Token": secret,
      },
      body: JSON.stringify(update),
    });
    return { index, status: res.status, ms: Date.now() - t0 };
  } catch (err) {
    return { index, status: 0, ms: Date.now() - t0, error: err.message };
  }
}

// ── Main ─────────────────────────────────────────────────────────────────────

async function main() {
  const args = parseArgs(process.argv);

  if (!args.url || !args.secret) {
    console.error(
      "Usage: node scripts/synthetic-burst.js --url <url> --secret <token> [--n 20] [--cmd /wordle]",
    );
    process.exit(1);
  }

  const { url, secret, n, cmd } = args;

  console.log(`Burst: ${n} parallel requests → ${url}/webhook  cmd=${cmd}`);
  console.log("Starting at", new Date().toISOString());

  const requests = Array.from({ length: n }, (_, i) =>
    sendUpdate(url, secret, buildUpdate(cmd, i), i),
  );

  const results = await Promise.all(requests);

  // ── Summary ──────────────────────────────────────────────────────────────
  let ok = 0;
  let fail = 0;
  let totalMs = 0;
  const statusCounts = {};

  for (const r of results) {
    const statusKey = r.status === 0 ? "network-error" : String(r.status);
    statusCounts[statusKey] = (statusCounts[statusKey] ?? 0) + 1;
    totalMs += r.ms;
    if (r.status >= 200 && r.status < 300) ok++;
    else fail++;

    // Log individual result.
    const tag = r.error ? `ERROR(${r.error})` : `HTTP ${r.status}`;
    console.log(`  [${r.index}] ${tag}  ${r.ms}ms`);
  }

  const avgMs = Math.round(totalMs / n);
  const allMs = results.map((r) => r.ms).sort((a, b) => a - b);
  const p50 = allMs[Math.floor(allMs.length * 0.5)];
  const p95 = allMs[Math.floor(allMs.length * 0.95)];

  console.log("\n── Summary ───────────────────────────────────────────");
  console.log(`  Requests: ${n}  |  OK: ${ok}  |  Failed: ${fail}`);
  console.log(`  Status counts: ${JSON.stringify(statusCounts)}`);
  console.log(`  Latency — avg: ${avgMs}ms  p50: ${p50}ms  p95: ${p95}ms`);
  console.log("\nNext: check Atlas UI connection counter within 60s.");
  console.log("Abort if peak connections > 300/500 (60% of M0 cap).");
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
