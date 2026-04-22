/**
 * @file Win-message text helpers — one-word reaction + elapsed-time format.
 */

/**
 * One-word reaction keyed by how many guesses it took to solve.
 *
 * @param {number} attempt — 1-indexed count of the guess that won (1..max)
 * @param {number} max — MAX_GUESSES (8 for loldle)
 * @returns {string}
 */
export function attemptFlavor(attempt, max) {
  if (attempt <= 1) return "First try!";
  if (attempt === 2) return "Sharp!";
  if (attempt >= max) return "Phew — last one!";
  if (attempt >= max - 2) return "Close call!";
  return "Nice.";
}

/**
 * Format an elapsed duration in ms as a compact human string.
 *   < 60s       → "42s"
 *   < 60min     → "3m 14s"
 *   otherwise   → "1h 12m"
 *
 * @param {number} ms
 * @returns {string}
 */
export function formatDuration(ms) {
  const total = Math.max(0, Math.round(ms / 1000));
  if (total < 60) return `${total}s`;
  const minutes = Math.floor(total / 60);
  const seconds = total % 60;
  if (minutes < 60) return seconds === 0 ? `${minutes}m` : `${minutes}m ${seconds}s`;
  const hours = Math.floor(minutes / 60);
  const remMin = minutes % 60;
  return remMin === 0 ? `${hours}h` : `${hours}h ${remMin}m`;
}
