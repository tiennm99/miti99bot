/**
 * @file Trading module entry — fake/paper trading with crypto, VN stocks, forex, gold.
 * Handlers live in handlers.js; this file wires them into the module system.
 */

import { handleBuy, handleConvert, handleSell, handleTopup } from "./handlers.js";
import { handleStats } from "./stats-handler.js";

/** @type {import("../../db/kv-store-interface.js").KVStore | null} */
let db = null;

/** @type {import("../registry.js").BotModule} */
const tradingModule = {
  name: "trading",
  init: async ({ db: store }) => {
    db = store;
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
      description: "Buy crypto/stock/gold at market price",
      handler: (ctx) => handleBuy(ctx, db),
    },
    {
      name: "trade_sell",
      visibility: "public",
      description: "Sell holdings back to VND",
      handler: (ctx) => handleSell(ctx, db),
    },
    {
      name: "trade_convert",
      visibility: "public",
      description: "Convert between currencies (bid/ask spread)",
      handler: (ctx) => handleConvert(ctx, db),
    },
    {
      name: "trade_stats",
      visibility: "public",
      description: "Show portfolio summary with P&L",
      handler: (ctx) => handleStats(ctx, db),
    },
  ],
};

export default tradingModule;
