/**
 * @file /trade_stats handler — portfolio summary with P&L breakdown.
 */

import { formatCrypto, formatCurrency, formatPnL, formatStock, formatVND } from "./format.js";
import { getPortfolio } from "./portfolio.js";
import { getPrices } from "./prices.js";

/** /trade_stats — show full portfolio valued in VND with P&L */
export async function handleStats(ctx, db) {
  const p = await getPortfolio(db, ctx.from?.id);
  let prices;
  try {
    prices = await getPrices(db);
  } catch {
    return ctx.reply("Could not fetch prices. Try again later.");
  }

  const lines = ["📊 Portfolio Summary\n"];
  let totalValue = 0;

  // currencies
  const currLines = [];
  for (const [cur, bal] of Object.entries(p.currency)) {
    if (bal === 0) continue;
    const rate = cur === "VND" ? 1 : (prices.forex?.[cur]?.mid ?? 0);
    const vndVal = bal * rate;
    totalValue += vndVal;
    currLines.push(
      cur === "VND"
        ? `  VND: ${formatVND(bal)}`
        : `  ${cur}: ${formatCurrency(bal, cur)} (~${formatVND(vndVal)})`,
    );
  }
  if (currLines.length) lines.push("Currency:", ...currLines);

  // asset categories
  for (const [catName, catLabel] of [
    ["stock", "Stocks"],
    ["crypto", "Crypto"],
    ["others", "Others"],
  ]) {
    const catLines = [];
    for (const [sym, qty] of Object.entries(p[catName])) {
      if (qty === 0) continue;
      const price = prices[catName]?.[sym];
      if (price == null) {
        catLines.push(`  ${sym}: ${qty} (no price)`);
        continue;
      }
      const val = qty * price;
      totalValue += val;
      const fmtQty = catName === "stock" ? formatStock(qty) : formatCrypto(qty);
      catLines.push(`  ${sym} x${fmtQty} @ ${formatVND(price)} = ${formatVND(val)}`);
    }
    if (catLines.length) lines.push(`\n${catLabel}:`, ...catLines);
  }

  lines.push(`\nTotal value: ${formatVND(totalValue)}`);
  lines.push(`Invested: ${formatVND(p.totalvnd)}`);
  lines.push(`P&L: ${formatPnL(totalValue, p.totalvnd)}`);
  await ctx.reply(lines.join("\n"));
}
