/**
 * @file /trade_stats handler — portfolio summary with P&L breakdown.
 *
 * Price fetches are issued in parallel with Promise.allSettled so a portfolio
 * holding N stocks only waits for the slowest fetch, not the sum. Without this
 * 10+ symbols would serially stack KBS latency and can blow Cloudflare's
 * subrequest budget.
 */

import { formatPnL, formatStock, formatVND } from "./format.js";
import { getPortfolio } from "./portfolio.js";
import { getStockPrice } from "./prices.js";

/** /trade_stats — show full portfolio valued in VND with P&L */
export async function handleStats(ctx, db) {
  const id = ctx.from?.id;
  if (id == null) {
    return ctx.reply("Cannot identify user — /trade_stats needs a sender.");
  }
  const p = await getPortfolio(db, id);

  const lines = ["📊 Portfolio Summary\n"];
  let totalValue = 0;

  const vnd = p.currency.VND || 0;
  if (vnd > 0) {
    totalValue += vnd;
    lines.push(`VND: ${formatVND(vnd)}`);
  }

  const held = Object.entries(p.assets).filter(([, qty]) => qty !== 0);
  if (held.length > 0) {
    lines.push("\nStocks:");
    const prices = await Promise.allSettled(held.map(([sym]) => getStockPrice(sym)));
    for (let i = 0; i < held.length; i++) {
      const [sym, qty] = held[i];
      const settled = prices[i];
      const price = settled.status === "fulfilled" ? settled.value : null;
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
