/**
 * @file Command handler implementations for the trading module.
 * Each handler receives (ctx, db) — the grammY context and KV store.
 */

import { formatCrypto, formatCurrency, formatStock, formatVND } from "./format.js";
import {
  addAsset,
  addCurrency,
  deductAsset,
  deductCurrency,
  getPortfolio,
  savePortfolio,
} from "./portfolio.js";
import { getForexRate, getPrice } from "./prices.js";
import { CURRENCIES, getSymbol, listSymbols } from "./symbols.js";

function uid(ctx) {
  return ctx.from?.id;
}

function parseArgs(ctx) {
  return (ctx.match || "").trim().split(/\s+/).filter(Boolean);
}

function usageReply(ctx, usage) {
  return ctx.reply(`Usage: ${usage}`);
}

/** /trade_topup <amount> [currency=VND] */
export async function handleTopup(ctx, db) {
  const args = parseArgs(ctx);
  if (args.length < 1) return usageReply(ctx, "/trade_topup <amount> [VND|USD]");
  const amount = Number(args[0]);
  if (!Number.isFinite(amount) || amount <= 0)
    return ctx.reply("Amount must be a positive number.");
  const currency = (args[1] || "VND").toUpperCase();
  if (!CURRENCIES.has(currency))
    return ctx.reply(`Unsupported currency. Use: ${[...CURRENCIES].join(", ")}`);

  const p = await getPortfolio(db, uid(ctx));
  addCurrency(p, currency, amount);
  if (currency === "VND") {
    p.totalvnd += amount;
  } else {
    const rate = await getForexRate(db, currency);
    if (rate == null) return ctx.reply("Could not fetch forex rate. Try again later.");
    p.totalvnd += amount * rate;
  }
  await savePortfolio(db, uid(ctx), p);
  const bal = p.currency[currency];
  await ctx.reply(
    `Topped up ${formatCurrency(amount, currency)}.\nBalance: ${formatCurrency(bal, currency)}`,
  );
}

/** /trade_buy <amount> <symbol> */
export async function handleBuy(ctx, db) {
  const args = parseArgs(ctx);
  if (args.length < 2)
    return usageReply(ctx, "/trade_buy <amount> <SYMBOL>\nExample: /trade_buy 0.01 BTC");
  const amount = Number(args[0]);
  if (!Number.isFinite(amount) || amount <= 0)
    return ctx.reply("Amount must be a positive number.");
  const info = getSymbol(args[1]);
  if (!info) return ctx.reply(`Unknown symbol.\n${listSymbols()}`);
  if (info.category === "stock" && !Number.isInteger(amount))
    return ctx.reply("Stock quantities must be whole numbers.");

  let price;
  try {
    price = await getPrice(db, info.symbol);
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
  const qty = info.category === "stock" ? formatStock(amount) : formatCrypto(amount);
  await ctx.reply(`Bought ${qty} ${info.symbol} @ ${formatVND(price)}\nCost: ${formatVND(cost)}`);
}

/** /trade_sell <amount> <symbol> */
export async function handleSell(ctx, db) {
  const args = parseArgs(ctx);
  if (args.length < 2)
    return usageReply(ctx, "/trade_sell <amount> <SYMBOL>\nExample: /trade_sell 0.01 BTC");
  const amount = Number(args[0]);
  if (!Number.isFinite(amount) || amount <= 0)
    return ctx.reply("Amount must be a positive number.");
  const info = getSymbol(args[1]);
  if (!info) return ctx.reply(`Unknown symbol.\n${listSymbols()}`);
  if (info.category === "stock" && !Number.isInteger(amount))
    return ctx.reply("Stock quantities must be whole numbers.");

  const p = await getPortfolio(db, uid(ctx));
  const result = deductAsset(p, info.symbol, amount);
  if (!result.ok) {
    const qty = info.category === "stock" ? formatStock(result.held) : formatCrypto(result.held);
    return ctx.reply(`Insufficient ${info.symbol}. You have: ${qty}`);
  }

  let price;
  try {
    price = await getPrice(db, info.symbol);
  } catch {
    return ctx.reply("Could not fetch price. Try again later.");
  }
  if (price == null) return ctx.reply(`No price available for ${info.symbol}.`);

  const revenue = amount * price;
  addCurrency(p, "VND", revenue);
  await savePortfolio(db, uid(ctx), p);
  const qty = info.category === "stock" ? formatStock(amount) : formatCrypto(amount);
  await ctx.reply(
    `Sold ${qty} ${info.symbol} @ ${formatVND(price)}\nRevenue: ${formatVND(revenue)}`,
  );
}

/** /trade_convert <amount> <from> <to> */
export async function handleConvert(ctx, db) {
  const args = parseArgs(ctx);
  if (args.length < 3)
    return usageReply(
      ctx,
      "/trade_convert <amount> <FROM> <TO>\nExample: /trade_convert 100 USD VND",
    );
  const amount = Number(args[0]);
  if (!Number.isFinite(amount) || amount <= 0)
    return ctx.reply("Amount must be a positive number.");
  const from = args[1].toUpperCase();
  const to = args[2].toUpperCase();
  if (!CURRENCIES.has(from) || !CURRENCIES.has(to))
    return ctx.reply(`Supported currencies: ${[...CURRENCIES].join(", ")}`);
  if (from === to) return ctx.reply("Cannot convert to the same currency.");

  let fromRate;
  let toRate;
  try {
    [fromRate, toRate] = await Promise.all([getForexRate(db, from), getForexRate(db, to)]);
  } catch {
    return ctx.reply("Could not fetch forex rate. Try again later.");
  }
  if (fromRate == null || toRate == null)
    return ctx.reply("Forex rate unavailable. Try again later.");

  const p = await getPortfolio(db, uid(ctx));
  const result = deductCurrency(p, from, amount);
  if (!result.ok)
    return ctx.reply(`Insufficient ${from}. Balance: ${formatCurrency(result.balance, from)}`);
  const converted = (amount * fromRate) / toRate;
  addCurrency(p, to, converted);
  await savePortfolio(db, uid(ctx), p);
  await ctx.reply(`Converted ${formatCurrency(amount, from)} → ${formatCurrency(converted, to)}`);
}
