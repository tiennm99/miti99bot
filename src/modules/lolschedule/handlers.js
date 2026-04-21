/**
 * @file Command + cron handlers for lolschedule.
 *
 * Day boundaries are defined in ICT (UTC+7). Data comes from lolesports.com
 * via a cache-first fetcher; no cron pre-warm is needed because the upstream
 * API is rate-limit friendly. A daily cron also pushes today's schedule to a
 * configured chat.
 */

import { getEventsCached } from "./api-client.js";
import { renderToday, renderWeek } from "./format.js";

const ICT_OFFSET_MS = 7 * 60 * 60 * 1000;

// Top-tier league allowlist. The API returns every regional/academy league
// (135+ events/week); filtering keeps the reply under Telegram's 4096-char
// limit and focuses on what most viewers care about.
const MAJOR_LEAGUE_SLUGS = new Set([
  "lck",
  "lpl",
  "lec",
  "lcs",
  "worlds",
  "msi",
  "first_stand",
  "lcp",
  "cblol-brazil",
  "emea_masters",
]);

function filterMajor(events) {
  return events.filter((e) => MAJOR_LEAGUE_SLUGS.has(e?.league?.slug));
}

/** Start of the current ICT calendar day, expressed as a UTC `Date`. */
export function ictDayStart(nowMs = Date.now()) {
  const shifted = new Date(nowMs + ICT_OFFSET_MS);
  shifted.setUTCHours(0, 0, 0, 0);
  return new Date(shifted.getTime() - ICT_OFFSET_MS);
}

/** Add `days` days to a Date, preserving time-of-day. */
export function addDays(date, days) {
  return new Date(date.getTime() + days * 24 * 60 * 60 * 1000);
}

/**
 * @param {import("grammy").Context} ctx
 * @param {import("../../db/kv-store-interface.js").KVStore | null} db
 */
export async function handleToday(ctx, db) {
  if (!db) {
    await ctx.reply("lolschedule: storage unavailable");
    return;
  }
  const from = ictDayStart();
  const to = addDays(from, 1);
  try {
    const events = filterMajor(await getEventsCached(db, from, to));
    await ctx.reply(renderToday(events, from), { parse_mode: "HTML" });
  } catch (err) {
    console.log(JSON.stringify({ msg: "lolschedule_today_fail", err: String(err) }));
    await ctx.reply("Could not fetch today's matches. Try again later.");
  }
}

/**
 * @param {import("grammy").Context} ctx
 * @param {import("../../db/kv-store-interface.js").KVStore | null} db
 */
export async function handleWeek(ctx, db) {
  if (!db) {
    await ctx.reply("lolschedule: storage unavailable");
    return;
  }
  const from = ictDayStart();
  const to = addDays(from, 7);
  try {
    const events = filterMajor(await getEventsCached(db, from, to));
    await ctx.reply(renderWeek(events, from, to), { parse_mode: "HTML" });
  } catch (err) {
    console.log(JSON.stringify({ msg: "lolschedule_week_fail", err: String(err) }));
    await ctx.reply("Could not fetch this week's matches. Try again later.");
  }
}

/**
 * Send a Telegram HTML message directly via the Bot API. Used from the cron
 * path where the grammY bot context is not available.
 *
 * @param {string} token
 * @param {string|number} chatId
 * @param {string} text
 */
async function sendTelegramMessage(token, chatId, text) {
  const res = await fetch(`https://api.telegram.org/bot${token}/sendMessage`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      chat_id: chatId,
      text,
      parse_mode: "HTML",
      disable_web_page_preview: true,
    }),
  });
  if (!res.ok) {
    const body = await res.text().catch(() => "");
    throw new Error(`telegram sendMessage HTTP ${res.status}: ${body.slice(0, 300)}`);
  }
}

/**
 * Cron handler — pushes today's major-league schedule to `LOLSCHEDULE_CHAT_ID`.
 * Silently skips when the env var or token is missing, or when there are no
 * major-league matches today.
 *
 * @param {any} _event
 * @param {{ db: import("../../db/kv-store-interface.js").KVStore, env: any }} ctx
 */
export async function handleDailyPushCron(_event, ctx) {
  const { db, env } = ctx;
  const chatId = env?.LOLSCHEDULE_CHAT_ID;
  const token = env?.TELEGRAM_BOT_TOKEN;
  if (!chatId || !token) {
    console.log(
      JSON.stringify({
        msg: "lolschedule_cron_skip",
        reason: !chatId ? "no_chat_id" : "no_token",
      }),
    );
    return;
  }
  const from = ictDayStart();
  const to = addDays(from, 1);
  try {
    const events = filterMajor(await getEventsCached(db, from, to));
    if (events.length === 0) {
      console.log(JSON.stringify({ msg: "lolschedule_cron_empty" }));
      return;
    }
    await sendTelegramMessage(token, chatId, renderToday(events, from));
    console.log(JSON.stringify({ msg: "lolschedule_cron_sent", events: events.length }));
  } catch (err) {
    console.log(JSON.stringify({ msg: "lolschedule_cron_fail", err: String(err) }));
  }
}
