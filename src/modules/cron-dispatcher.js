/**
 * @file cron-dispatcher — dispatches a Cloudflare scheduled event to all
 * matching module cron handlers.
 *
 * Design:
 *  - Iterates registry.crons, filters by event.cron === entry.schedule.
 *  - Wraps each handler invocation in try/catch so one failure cannot block
 *    others (equivalent to Promise.allSettled fan-out via ctx.waitUntil).
 *  - ctx.waitUntil is fire-and-forget from Workers' perspective; we wrap in
 *    an async IIFE so errors are caught and logged rather than silently lost.
 */

import { createSqlStore } from "../db/create-sql-store.js";
import { createStore } from "../db/create-store.js";

/**
 * @param {any} event — Cloudflare ScheduledEvent (has .cron string).
 * @param {any} env
 * @param {{ waitUntil: (p: Promise<any>) => void }} ctx
 * @param {import("./registry.js").Registry} registry
 */
export function dispatchScheduled(event, env, ctx, registry) {
  const matching = registry.crons.filter((entry) => entry.schedule === event.cron);

  for (const entry of matching) {
    const handlerCtx = {
      db: createStore(entry.module.name, env),
      sql: createSqlStore(entry.module.name, env),
      env,
    };

    ctx.waitUntil(
      (async () => {
        try {
          await entry.handler(event, handlerCtx);
        } catch (err) {
          console.error(
            `[cron] handler "${entry.name}" (module "${entry.module.name}", schedule "${entry.schedule}") failed:`,
            err,
          );
        }
      })(),
    );
  }
}
