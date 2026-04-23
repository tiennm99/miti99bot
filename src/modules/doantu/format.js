/**
 * @file Display formatting helpers for similarity scores.
 *
 * phow2sim (PhoW2V word2vec) cosines already span a wide, game-friendly
 * range — unrelated pairs near 0.0, synonyms 0.6+ — so we map linearly:
 * `round(raw * 100)`, clamped to [0, 100]. Negative cosines (antonyms) clamp
 * to 0. No sigmoid, no magic constants.
 */

/** @param {number} rawCosine */
export function calibrate(rawCosine) {
  return Math.max(0, Math.min(100, rawCosine * 100));
}

/** @param {number} score — calibrated score in [0, 100] */
export function formatWarmth(score) {
  const pct = Math.round(score);
  return pct >= 100 ? "100" : String(pct).padStart(2, "0");
}

/** @param {number} score */
export function warmthEmoji(score) {
  if (score >= 90) return "🎯";
  if (score >= 70) return "🔥";
  if (score >= 40) return "🌡️";
  if (score >= 15) return "😐";
  return "🥶";
}
