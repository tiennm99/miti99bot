/**
 * @file Guess normalization + shape validation.
 *
 * Keeps obviously-bad input from hitting the API. The /random endpoint
 * already filters its output to ASCII letters only, so any guess outside
 * that shape can never equal the target — fail fast.
 */

/** @param {string} raw */
export function normalize(raw) {
  if (typeof raw !== "string") return "";
  return raw.trim().replace(/\s+/g, " ").toLowerCase();
}

/** @param {string} word — must already be normalized */
export function isValidShape(word) {
  if (!word) return false;
  if (word.length > 64) return false;
  return /^[a-z]+$/.test(word);
}
