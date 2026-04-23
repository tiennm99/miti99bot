#!/usr/bin/env node
/**
 * @file register — post-deploy Telegram registration.
 *
 * Runs after `wrangler deploy` (chained via `npm run deploy`). Reads env from
 * `.env.deploy` (via `node --env-file`), imports the same module registry
 * code the Worker uses, derives the public-command list, and calls Telegram's
 * HTTP API to (1) register the webhook with a secret token and (2) publish
 * setMyCommands.
 *
 * Idempotent — safe to re-run. Supports `--dry-run` to preview payloads
 * without calling Telegram.
 *
 * Required env (in .env.deploy):
 *   TELEGRAM_BOT_TOKEN       — bot token from BotFather
 *   TELEGRAM_WEBHOOK_SECRET  — must match the value `wrangler secret put` set for the Worker
 *   WORKER_URL               — https://<worker-subdomain>.workers.dev (no trailing slash)
 *   MODULES                  — same comma-separated list as wrangler.toml [vars]
 */

import { buildRegistry, resetRegistry } from "../src/modules/registry.js";
import { stubAi, stubKv } from "./stub-kv.js";

const TELEGRAM_API = "https://api.telegram.org";

/**
 * @param {string} name
 * @returns {string}
 */
function requireEnv(name) {
  const v = process.env[name];
  if (!v || v.trim().length === 0) {
    console.error(`missing env: ${name}`);
    process.exit(1);
  }
  return v.trim();
}

/**
 * @param {string} token
 * @param {string} method
 * @param {unknown} body
 */
async function callTelegram(token, method, body) {
  const res = await fetch(`${TELEGRAM_API}/bot${token}/${method}`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(body),
  });
  /** @type {any} */
  let json = {};
  try {
    json = await res.json();
  } catch {
    // Telegram always returns JSON on documented endpoints; only leave `json`
    // empty if the transport itself died.
  }
  if (!res.ok || json.ok === false) {
    console.error(`${method} failed:`, res.status, json);
    process.exit(1);
  }
  return json;
}

async function main() {
  const token = requireEnv("TELEGRAM_BOT_TOKEN");
  const secret = requireEnv("TELEGRAM_WEBHOOK_SECRET");
  const workerUrl = requireEnv("WORKER_URL").replace(/\/$/, "");
  const modules = requireEnv("MODULES");
  const dryRun = process.argv.includes("--dry-run");

  // Build the registry against the same code the Worker uses. Stub KV
  // satisfies the binding so createStore() does not throw.
  resetRegistry();
  const reg = await buildRegistry({ MODULES: modules, KV: stubKv, AI: stubAi });

  const commands = [...reg.publicCommands.values()].map(({ cmd }) => ({
    command: cmd.name,
    description: cmd.description,
  }));

  const webhookBody = {
    url: `${workerUrl}/webhook`,
    secret_token: secret,
    allowed_updates: ["message", "edited_message", "callback_query"],
    drop_pending_updates: false,
  };
  const commandsBody = { commands };

  if (dryRun) {
    console.log("DRY RUN — not calling Telegram");
    // Redact the secret in the printed payload.
    console.log("setWebhook:", { ...webhookBody, secret_token: "<redacted>" });
    console.log("setMyCommands:", commandsBody);
    return;
  }

  await callTelegram(token, "setWebhook", webhookBody);
  await callTelegram(token, "setMyCommands", commandsBody);

  console.log(`ok — webhook: ${webhookBody.url}`);
  console.log(`ok — ${commands.length} public commands registered:`);
  for (const c of commands) console.log(`  /${c.command} — ${c.description}`);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
