/**
 * @file Display formatting helpers for similarity scores.
 *
 * BGE embeddings live in a narrow cone so raw cosines are compressed —
 * unrelated word pairs already score ~0.40-0.55, which reads as
 * misleadingly "warm" to the player. We remap raw cosine through a
 * normalized sigmoid so the displayed 0-100 score actually tracks
 * semantic closeness: unrelated → ≤30, related → 70+, near-identical → 90+.
 *
 * Hyperparameters tuned empirically for `@cf/baai/bge-m3`. If switching
 * models, re-measure random-pair cosines and retune CENTER/SCALE.
 */

const FLOOR = 0.4;
const CENTER = 0.6;
const SCALE = 8.0;

const sigmoid = (x) => 1 / (1 + Math.exp(-x));
const FLOOR_SIG = sigmoid(SCALE * (FLOOR - CENTER));
const ONE_SIG = sigmoid(SCALE * (1 - CENTER));
const SIG_RANGE = ONE_SIG - FLOOR_SIG;

/**
 * Map raw cosine ∈ [-1, 1] to a calibrated display score ∈ [0, 100].
 * @param {number} rawCosine
 */
export function calibrate(rawCosine) {
  if (rawCosine >= 1) return 100;
  if (rawCosine <= FLOOR) return 0;
  const s = sigmoid(SCALE * (rawCosine - CENTER));
  return Math.max(0, Math.min(100, ((s - FLOOR_SIG) / SIG_RANGE) * 100));
}

/**
 * Zero-padded integer percent, width 2 (e.g. "07", "54", "100").
 * @param {number} score — calibrated score in [0, 100]
 */
export function formatWarmth(score) {
  const pct = Math.round(score);
  return pct >= 100 ? "100" : String(pct).padStart(2, "0");
}

/**
 * Warmth emoji bucket. Thresholds operate on the CALIBRATED score,
 * not raw cosine.
 * @param {number} score
 */
export function warmthEmoji(score) {
  if (score >= 90) return "🎯";
  if (score >= 70) return "🔥";
  if (score >= 40) return "🌡️";
  if (score >= 15) return "😐";
  return "🥶";
}
