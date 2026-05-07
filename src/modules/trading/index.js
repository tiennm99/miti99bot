/**
 * @file Trading module entry — fake/paper trading with crypto, VN stocks, forex, gold.
 * Handlers live in handlers.js; this file wires them into the module system.
 */

import { handleBuy, handleConvert, handleSell, handleTopup } from "./handlers.js";
import { createHistoryHandler, recordTrade } from "./history.js";
import { trimTradesHandler } from "./retention.js";
import { handleStats } from "./stats-handler.js";

/** @type {import("../../db/kv-store-interface.js").KVStore | null} */
let db = null;

/** @type {import("../../db/sql-store-interface.js").SqlStore | null} */
let sql = null;

/**
 * Build an onTrade callback bound to the current sql store and userId.
 *
 * @param {number} userId
 * @returns {(trade: {symbol:string, side:"buy"|"sell", qty:number, priceVnd:number}) => Promise<void>}
 */
function makeOnTrade(userId) {
  return ({ symbol, side, qty, priceVnd }) =>
    recordTrade(sql, { userId, symbol, side, qty, priceVnd });
}

/** @type {import("../registry.js").BotModule} */
const tradingModule = {
  name: "trading",
  init: async ({ db: store, sql: sqlStore }) => {
    db = store;
    sql = sqlStore ?? null;
  },
  commands: [
    {
      name: "trade_topup",
      visibility: "public",
      description: "Top up VND to your trading account",
      handler: (ctx) => handleTopup(ctx, db),
    },
    {
      name: "trade_buy",
      visibility: "public",
      description: "Buy VN stock at market price",
      handler: (ctx) => handleBuy(ctx, db, makeOnTrade(ctx.from?.id)),
    },
    {
      name: "trade_sell",
      visibility: "public",
      description: "Sell VN stock back to VND",
      handler: (ctx) => handleSell(ctx, db, makeOnTrade(ctx.from?.id)),
    },
    {
      name: "trade_convert",
      visibility: "public",
      description: "Currency exchange (coming soon)",
      handler: (ctx) => handleConvert(ctx),
    },
    {
      name: "trade_stats",
      visibility: "public",
      description: "Show portfolio summary with P&L",
      handler: (ctx) => handleStats(ctx, db),
    },
    {
      name: "history",
      visibility: "public",
      description: "Show your last N trades (default 10, max 50)",
      // handler is created lazily so it picks up the sql value set in init().
      handler: (ctx) => createHistoryHandler(sql)(ctx),
    },
  ],
  crons: [
    {
      schedule: "0 17 * * *",
      name: "trim-trades",
      handler: (event, ctx) => trimTradesHandler(event, ctx),
    },
  ],
};

export default tradingModule;
