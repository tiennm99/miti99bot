/**
 * @file /trade_stats handler — portfolio summary with P&L breakdown.
 * Fetches live stock prices for each held asset.
 */

import { formatPnL, formatStock, formatVND } from "./format.js";
import { getPortfolio } from "./portfolio.js";
import { getStockPrice } from "./prices.js";

/** /trade_stats — show full portfolio valued in VND with P&L */
export async function handleStats(ctx, db) {
  const p = await getPortfolio(db, ctx.from?.id);

  const lines = ["📊 Portfolio Summary\n"];
  let totalValue = 0;

  // VND balance
  const vnd = p.currency.VND || 0;
  if (vnd > 0) {
    totalValue += vnd;
    lines.push(`VND: ${formatVND(vnd)}`);
  }

  // stock assets
  const assetEntries = Object.entries(p.assets);
  if (assetEntries.length > 0) {
    lines.push("\nStocks:");
    for (const [sym, qty] of assetEntries) {
      if (qty === 0) continue;
      let price;
      try {
        price = await getStockPrice(sym);
      } catch {
        price = null;
      }
      if (price == null) {
        lines.push(`  ${sym} x${formatStock(qty)} (no price)`);
        continue;
      }
      const val = qty * price;
      totalValue += val;
      lines.push(`  ${sym} x${formatStock(qty)} @ ${formatVND(price)} = ${formatVND(val)}`);
    }
  }

  lines.push(`\nTotal value: ${formatVND(totalValue)}`);
  lines.push(`Invested: ${formatVND(p.meta.invested)}`);
  lines.push(`P&L: ${formatPnL(totalValue, p.meta.invested)}`);
  await ctx.reply(lines.join("\n"));
}
