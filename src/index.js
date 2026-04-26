/**
 * @file fetch entry point for the Cloudflare Worker.
 *
 * Routes:
 *   GET  /         — "miti99bot ok" health check (unauthenticated).
 *   POST /webhook  — Telegram webhook. grammY validates the
 *                    X-Telegram-Bot-Api-Secret-Token header against
 *                    env.TELEGRAM_WEBHOOK_SECRET and replies 401 on mismatch.
 *   *              — 404.
 *
 * There is NO admin HTTP surface. `setWebhook` + `setMyCommands` run at
 * deploy time from `scripts/register.js`, not from the Worker.
 */

import { webhookCallback } from "grammy";
import { getBot, getRegistry } from "./bot.js";
import { dispatchScheduled } from "./modules/cron-dispatcher.js";
import { setLastCold } from "./util/request-context.js";
import { takeColdFlag } from "./util/timing.js";

/** @type {ReturnType<typeof webhookCallback> | null} */
let cachedWebhookHandler = null;

/**
 * @param {any} env
 */
async function getWebhookHandler(env) {
  if (cachedWebhookHandler) return cachedWebhookHandler;
  const bot = await getBot(env);
  cachedWebhookHandler = webhookCallback(bot, "cloudflare-mod", {
    secretToken: env.TELEGRAM_WEBHOOK_SECRET,
    // Default is 10s — too short for LLM calls (Gemma 4 cold-start can exceed).
    // Workers wall-clock limit is 30s; leave 5s headroom.
    timeoutMilliseconds: 25000,
  });
  return cachedWebhookHandler;
}

export default {
  /**
   * Cloudflare Cron Trigger handler.
   * Dispatches the scheduled event to all module cron handlers whose
   * schedule matches event.cron.
   *
   * @param {any} event — ScheduledEvent ({ cron: string, scheduledTime: number })
   * @param {any} env
   * @param {{ waitUntil: (p: Promise<any>) => void }} ctx
   */
  async scheduled(event, env, ctx) {
    try {
      const registry = await getRegistry(env);
      dispatchScheduled(event, env, ctx, registry);
    } catch (err) {
      console.error("scheduled handler failed", err);
    }
  },

  /**
   * @param {Request} request
   * @param {any} env
   * @param {any} _ctx
   */
  async fetch(request, env, _ctx) {
    // Capture cold-start flag first — must be before any await.
    // Stored in shared request-context module so dispatcher middleware can read it
    // without a circular import (index → bot → dispatcher → index).
    const coldMeta = takeColdFlag();
    setLastCold(coldMeta);

    const start = Date.now();
    const { pathname } = new URL(request.url);
    const method = request.method;

    // Per-request cold-start log for CF Observability soak analysis.
    console.log(
      JSON.stringify({
        event: "request",
        method,
        path: pathname,
        cold: coldMeta.cold,
        isolateAgeMs: coldMeta.isolateAgeMs,
        ts: start,
      }),
    );

    const response = await route(request, env, pathname);

    // Structured request log for Workers Observability dashboard.
    console.log(
      JSON.stringify({
        msg: "req",
        method,
        path: pathname,
        status: response.status,
        ms: Date.now() - start,
      }),
    );
    return response;
  },
};

/**
 * @param {Request} request
 * @param {any} env
 * @param {string} pathname
 */
async function route(request, env, pathname) {
  if (request.method === "GET" && pathname === "/") {
    return new Response("miti99bot ok", {
      status: 200,
      headers: { "content-type": "text/plain" },
    });
  }

  if (request.method === "POST" && pathname === "/webhook") {
    try {
      const handler = await getWebhookHandler(env);
      return await handler(request);
    } catch (err) {
      console.error("webhook handler failed", err);
      return new Response("internal error", { status: 500 });
    }
  }

  return new Response("not found", { status: 404 });
}
