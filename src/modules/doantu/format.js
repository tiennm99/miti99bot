/**
 * @file Display formatting helpers for similarity scores.
 * Identical to semantle/format.js — score calibration is language-agnostic
 * because bge-m3 runs on both modules with the same cosine distribution.
 */

const FLOOR = 0.4;
const CENTER = 0.6;
const SCALE = 8.0;

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
