/**
 * @file Display formatting helpers for similarity scores.
 *
 * Scores live in [-1, 1]. Display as signed percent (`+73`, `-04`) plus an
 * emoji bucket so the UX reads "warmer / colder" at a glance.
 */

/**
 * Signed, zero-padded percent: +73, -04, +00.
 * @param {number} similarity
 */
export function formatWarmth(similarity) {
  const pct = Math.round(similarity * 100);
  const sign = pct >= 0 ? "+" : "-";
  return `${sign}${String(Math.abs(pct)).padStart(2, "0")}`;
}

/**
 * Warmth emoji bucket. Thresholds are intentionally coarse — anything ≥ 0.6
 * is already "very close" in word2vec space.
 * @param {number} similarity
 */
export function warmthEmoji(similarity) {
  if (similarity >= 0.8) return "🎯";
  if (similarity >= 0.6) return "🔥";
  if (similarity >= 0.4) return "🌡️";
  if (similarity >= 0.2) return "😐";
  return "🥶";
}
