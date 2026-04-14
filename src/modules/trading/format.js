/**
 * @file Number formatters for trading module display.
 * Manual VND formatter avoids locale-dependent toLocaleString in CF Workers.
 */

/**
 * Format number as Vietnamese Dong — dot thousands separator, no decimals.
 * @param {number} n
 * @returns {string} e.g. "15.000.000 VND"
 */
export function formatVND(n) {
  const rounded = Math.round(n);
  const abs = Math.abs(rounded).toString();
  // insert dots every 3 digits from right
  let result = "";
  for (let i = 0; i < abs.length; i++) {
    if (i > 0 && (abs.length - i) % 3 === 0) result += ".";
    result += abs[i];
  }
  return `${rounded < 0 ? "-" : ""}${result} VND`;
}

/**
 * Format number as USD — 2 decimals, comma thousands.
 * @param {number} n
 * @returns {string} e.g. "$1,234.56"
 */
export function formatUSD(n) {
  const fixed = Math.abs(n).toFixed(2);
  const [intPart, decPart] = fixed.split(".");
  let result = "";
  for (let i = 0; i < intPart.length; i++) {
    if (i > 0 && (intPart.length - i) % 3 === 0) result += ",";
    result += intPart[i];
  }
  return `${n < 0 ? "-" : ""}$${result}.${decPart}`;
}

/**
 * Format crypto quantity — up to 8 decimals, trailing zeros stripped.
 * @param {number} n
 * @returns {string} e.g. "0.00125"
 */
export function formatCrypto(n) {
  return Number.parseFloat(n.toFixed(8)).toString();
}

/**
 * Format stock quantity — integer only.
 * @param {number} n
 * @returns {string}
 */
export function formatStock(n) {
  return Math.floor(n).toString();
}

/**
 * Format amount based on currency type.
 * @param {number} n
 * @param {string} currency — "VND" or "USD"
 * @returns {string}
 */
export function formatCurrency(n, currency) {
  if (currency === "VND") return formatVND(n);
  if (currency === "USD") return formatUSD(n);
  return `${n} ${currency}`;
}

/**
 * Format P&L line with absolute and percentage.
 * @param {number} currentValue
 * @param {number} invested
 * @returns {string}
 */
export function formatPnL(currentValue, invested) {
  const diff = currentValue - invested;
  const pct = invested > 0 ? ((diff / invested) * 100).toFixed(2) : "0.00";
  const sign = diff >= 0 ? "+" : "";
  return `${sign}${formatVND(diff)} (${sign}${pct}%)`;
}
