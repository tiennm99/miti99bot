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
import { getForexBidAsk, getPrice } from "./prices.js";
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
  p.totalvnd += amount;
  await savePortfolio(db, uid(ctx), p);
  await ctx.reply(`Topped up ${formatVND(amount)}.\nBalance: ${formatVND(p.currency.VND)}`);
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

/** /trade_convert <amount> <from> <to> — with bid/ask spread */
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

  let rates;
  try {
    rates = await getForexBidAsk(db, "USD");
  } catch {
    return ctx.reply("Could not fetch forex rate. Try again later.");
  }
  if (!rates) return ctx.reply("Forex rate unavailable. Try again later.");

  const p = await getPortfolio(db, uid(ctx));
  const result = deductCurrency(p, from, amount);
  if (!result.ok)
    return ctx.reply(`Insufficient ${from}. Balance: ${formatCurrency(result.balance, from)}`);

  // buy = bank buys USD (you sell USD → VND), sell = bank sells USD (you buy USD → pay VND)
  let converted;
  let rateUsed;
  if (from === "VND" && to === "USD") {
    // you're buying USD from bank → bank sells at higher price
    converted = amount / rates.sell;
    rateUsed = rates.sell;
  } else {
    // you're selling USD to bank → bank buys at lower price
    converted = amount * rates.buy;
    rateUsed = rates.buy;
  }

  addCurrency(p, to, converted);
  await savePortfolio(db, uid(ctx), p);
  const spread = (((rates.sell - rates.buy) / rates.buy) * 100).toFixed(2);
  await ctx.reply(
    `Converted ${formatCurrency(amount, from)} → ${formatCurrency(converted, to)}\nRate: ${formatVND(rateUsed)}/USD (buy: ${formatVND(rates.buy)}, sell: ${formatVND(rates.sell)}, spread: ${spread}%)`,
  );
}
