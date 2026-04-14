/**
 * @file Command handler implementations for the trading module.
 * Each handler receives (ctx, db) — the grammY context and KV store.
 * Currently only VN stocks are supported. Crypto/gold/convert coming later.
 */

import { formatStock, formatVND } from "./format.js";
import {
  addAsset,
  addCurrency,
  deductAsset,
  deductCurrency,
  getPortfolio,
  savePortfolio,
} from "./portfolio.js";
import { getStockPrice } from "./prices.js";
import { comingSoonMessage, resolveSymbol } from "./symbols.js";

function uid(ctx) {
  return ctx.from?.id;
}

function parseArgs(ctx) {
  return (ctx.match || "").trim().split(/\s+/).filter(Boolean);
}

function usageReply(ctx, usage) {
  return ctx.reply(`Usage: ${usage}`);
}

/** /trade_topup <amount> — add VND to account */
export async function handleTopup(ctx, db) {
  const args = parseArgs(ctx);
  if (args.length < 1)
    return usageReply(ctx, "/trade_topup <amount>\nExample: /trade_topup 5000000");
  const amount = Number(args[0]);
  if (!Number.isFinite(amount) || amount <= 0)
    return ctx.reply("Amount must be a positive number.");

  const p = await getPortfolio(db, uid(ctx));
  addCurrency(p, "VND", amount);
  p.meta.invested += amount;
  await savePortfolio(db, uid(ctx), p);
  await ctx.reply(`Topped up ${formatVND(amount)}.\nBalance: ${formatVND(p.currency.VND)}`);
}

/** /trade_buy <amount> <symbol> — buy VN stock at market price */
export async function handleBuy(ctx, db) {
  const args = parseArgs(ctx);
  if (args.length < 2)
    return usageReply(ctx, "/trade_buy <qty> <TICKER>\nExample: /trade_buy 100 TCB");
  const amount = Number(args[0]);
  if (!Number.isFinite(amount) || amount <= 0)
    return ctx.reply("Amount must be a positive number.");
  if (!Number.isInteger(amount)) return ctx.reply("Stock quantities must be whole numbers.");

  const info = await resolveSymbol(db, args[1]);
  if (!info)
    return ctx.reply(`Unknown stock ticker "${args[1].toUpperCase()}".\n${comingSoonMessage()}`);

  let price;
  try {
    price = await getStockPrice(info.symbol);
  } catch {
    return ctx.reply("Could not fetch price. Try again later.");
  }
  if (price == null) return ctx.reply(`No price available for ${info.symbol}.`);

  const cost = amount * price;
  const p = await getPortfolio(db, uid(ctx));
  const result = deductCurrency(p, "VND", cost);
  if (!result.ok) {
    return ctx.reply(
      `Insufficient VND. Need ${formatVND(cost)}, have ${formatVND(result.balance)}.`,
    );
  }
  addAsset(p, info.symbol, amount);
  await savePortfolio(db, uid(ctx), p);
  await ctx.reply(
    `Bought ${formatStock(amount)} ${info.symbol} @ ${formatVND(price)}\nCost: ${formatVND(cost)}`,
  );
}

/** /trade_sell <amount> <symbol> — sell VN stock back to VND */
export async function handleSell(ctx, db) {
  const args = parseArgs(ctx);
  if (args.length < 2)
    return usageReply(ctx, "/trade_sell <qty> <TICKER>\nExample: /trade_sell 100 TCB");
  const amount = Number(args[0]);
  if (!Number.isFinite(amount) || amount <= 0)
    return ctx.reply("Amount must be a positive number.");
  if (!Number.isInteger(amount)) return ctx.reply("Stock quantities must be whole numbers.");

  const symbol = args[1].toUpperCase();
  const p = await getPortfolio(db, uid(ctx));
  const result = deductAsset(p, symbol, amount);
  if (!result.ok) return ctx.reply(`Insufficient ${symbol}. You have: ${formatStock(result.held)}`);

  let price;
  try {
    price = await getStockPrice(symbol);
  } catch {
    return ctx.reply("Could not fetch price. Try again later.");
  }
  if (price == null) return ctx.reply(`No price available for ${symbol}.`);

  const revenue = amount * price;
  addCurrency(p, "VND", revenue);
  await savePortfolio(db, uid(ctx), p);
  await ctx.reply(
    `Sold ${formatStock(amount)} ${symbol} @ ${formatVND(price)}\nRevenue: ${formatVND(revenue)}`,
  );
}

/** /trade_convert — disabled, coming soon */
export async function handleConvert(ctx) {
  await ctx.reply(`Currency exchange is not available yet.\n${comingSoonMessage()}`);
}
