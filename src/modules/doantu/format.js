/**
 * @file Display formatting helpers for similarity scores.
 * Identical to semantle/format.js — score display is language-agnostic.
 */

/** @param {number} similarity */
export function formatWarmth(similarity) {
  const pct = Math.round(similarity * 100);
  const sign = pct >= 0 ? "+" : "-";
  return `${sign}${String(Math.abs(pct)).padStart(2, "0")}`;
}

/** @param {number} similarity */
export function warmthEmoji(similarity) {
  if (similarity >= 0.8) return "🎯";
  if (similarity >= 0.6) return "🔥";
  if (similarity >= 0.4) return "🌡️";
  if (similarity >= 0.2) return "😐";
  return "🥶";
}
