/**
 * @file history — D1-backed trade record + /history command for the trading module.
 *
 * Exports:
 *   recordTrade(sql, opts)   — fire-and-forget insert; logs + swallows on failure.
 *   listTrades(sql, userId, limit) — newest-first query; returns [] when sql null.
 *   formatTradesHtml(trades) — HTML-escaped compact list for Telegram HTML mode.
 *   createHistoryHandler(sql) — grammY command handler factory.
 */

import { escapeHtml } from "../../util/escape-html.js";

/** @typedef {import("../../types.js").Trade} Trade */
/** @typedef {import("../../db/sql-store-interface.js").SqlStore} SqlStore */

const TABLE = "trading_trades";
const DEFAULT_LIMIT = 10;
const MAX_LIMIT = 50;

/**
 * Insert a trade row. Silently skips when sql is null (no D1 binding).
 * Failure is logged but never re-thrown — portfolio KV is source of truth.
 *
 * @param {SqlStore | null} sql
 * @param {{ userId: number, symbol: string, side: "buy"|"sell", qty: number, priceVnd: number }} opts
 * @returns {Promise<void>}
 */
export async function recordTrade(sql, { userId, symbol, side, qty, priceVnd }) {
  if (sql === null) {
    console.warn("[trading/history] recordTrade skipped — no D1 binding");
    return;
  }
  try {
    await sql.run(
      `INSERT INTO ${TABLE} (user_id, symbol, side, qty, price_vnd, ts) VALUES (?, ?, ?, ?, ?, ?)`,
      userId,
      symbol,
      side,
      qty,
      priceVnd,
      Date.now(),
    );
  } catch (err) {
    console.error("[trading/history] recordTrade failed:", err);
  }
}

/**
 * Fetch the most recent trades for a user, newest first.
 * Returns [] when sql is null or the table is empty.
 *
 * @param {SqlStore | null} sql
 * @param {number} userId
 * @param {number} limit — clamped to [1, 50].
 * @returns {Promise<Trade[]>}
 */
export async function listTrades(sql, userId, limit) {
  if (sql === null) return [];
  const n = Math.max(1, Math.min(MAX_LIMIT, limit));
  const rows = await sql.all(
    `SELECT id, user_id, symbol, side, qty, price_vnd, ts FROM ${TABLE} WHERE user_id = ? ORDER BY ts DESC LIMIT ?`,
    userId,
    n,
  );
  // Map snake_case DB columns → camelCase Trade objects.
  return rows.map((r) => ({
    id: r.id,
    userId: r.user_id,
    symbol: r.symbol,
    side: r.side,
    qty: r.qty,
    priceVnd: r.price_vnd,
    ts: r.ts,
  }));
}

/**
 * Render a trade list as Telegram HTML.
 * Symbols are HTML-escaped to prevent injection.
 *
 * @param {Trade[]} trades
 * @returns {string}
 */
export function formatTradesHtml(trades) {
  if (trades.length === 0) return "No trades recorded yet.";

  const header = "<b>Trade History</b>\n";
  const lines = trades.map((t) => {
    const side = t.side === "buy" ? "BUY " : "SELL";
    const sym = escapeHtml(t.symbol);
    const price = t.priceVnd.toLocaleString("vi-VN");
    const date = new Date(t.ts).toISOString().slice(0, 16).replace("T", " ");
    return `${side} <b>${t.qty}</b> <code>${sym}</code> @ ${price} VND  <i>${date}</i>`;
  });

  return header + lines.join("\n");
}

/**
 * Factory that returns a grammY command handler for /history [n].
 *
 * Parses optional N from ctx.match (default 10, clamped 1–50).
 * Replies with HTML trade list.
 *
 * @param {SqlStore | null} sql
 * @returns {(ctx: any) => Promise<void>}
 */
export function createHistoryHandler(sql) {
  return async (ctx) => {
    const userId = ctx.from?.id;
    if (!userId) return ctx.reply("Could not identify user.");

    const raw = Number.parseInt((ctx.match || "").trim(), 10);
    // Invalid / zero / negative → default; > MAX_LIMIT → clamp inside listTrades.
    const n = Number.isFinite(raw) && raw > 0 ? raw : DEFAULT_LIMIT;

    const trades = await listTrades(sql, userId, n);
    const html = formatTradesHtml(trades);
    await ctx.reply(html, { parse_mode: "HTML" });
  };
}
