/**
 * @file Guess normalization + shape validation.
 *
 * Keeps obviously-bad input out of the VOCAB lookup and the embedding call.
 * The wordlist is ASCII-letter-only at build time, so any guess outside
 * that shape is guaranteed OOV — fail fast.
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
