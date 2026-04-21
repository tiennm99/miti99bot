/**
 * @file /lol_today and /lol_week command handlers.
 *
 * Day boundaries are defined in ICT (UTC+7), converted to the UTC literals
 * that Leaguepedia's cargo query expects. Errors are surfaced as a short reply
 * — we never throw through grammY.
 */

import { CACHE_TTL_TODAY_SEC, CACHE_TTL_WEEK_SEC, getCachedMatches } from "./api-client.js";
import { renderToday, renderWeek } from "./format.js";

const ICT_OFFSET_MS = 7 * 60 * 60 * 1000;

/**
 * Start of the current ICT calendar day, expressed as a UTC `Date`.
 * @param {number} [nowMs]
 */
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
    const rows = await getCachedMatches(db, from, to, CACHE_TTL_TODAY_SEC);
    await ctx.reply(renderToday(rows, from), { parse_mode: "HTML" });
  } catch (err) {
    console.log(
      JSON.stringify({ msg: "lolschedule_today_fail", err: String(err) }),
    );
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
    const rows = await getCachedMatches(db, from, to, CACHE_TTL_WEEK_SEC);
    await ctx.reply(renderWeek(rows, from, to), { parse_mode: "HTML" });
  } catch (err) {
    console.log(
      JSON.stringify({ msg: "lolschedule_week_fail", err: String(err) }),
    );
    await ctx.reply("Could not fetch this week's matches. Try again later.");
  }
}
