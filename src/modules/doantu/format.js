/**
 * @file Display formatting helpers for similarity scores.
 *
 * Tuned for phow2sim (PhoW2V word2vec): its cosine distribution is wider
 * than bge-m3's narrow transformer cone that semantle/format.js calibrates
 * against. Typical word2vec ranges:
 *   unrelated pairs:     0.00–0.15
 *   loosely related:     0.15–0.30
 *   clearly related:     0.30–0.55
 *   synonyms/same-topic: 0.55–0.80
 *   identical:           1.00
 * Mapping: raw 0.10 → 0, 0.30 → ~25, 0.40 → ~45, 0.50 → ~60, 0.70 → ~85, 1.00 → 100.
 */

const FLOOR = 0.1;
const CENTER = 0.4;
const SCALE = 6.0;

const sigmoid = (x) => 1 / (1 + Math.exp(-x));
const FLOOR_SIG = sigmoid(SCALE * (FLOOR - CENTER));
const ONE_SIG = sigmoid(SCALE * (1 - CENTER));
const SIG_RANGE = ONE_SIG - FLOOR_SIG;

/** @param {number} rawCosine */
export function calibrate(rawCosine) {
  if (rawCosine >= 1) return 100;
  if (rawCosine <= FLOOR) return 0;
  const s = sigmoid(SCALE * (rawCosine - CENTER));
  return Math.max(0, Math.min(100, ((s - FLOOR_SIG) / SIG_RANGE) * 100));
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
