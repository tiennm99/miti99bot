/**
 * @file Guess normalization + shape validation (Vietnamese).
 *
 * Allows Unicode letters (including diacritics via combining marks) plus
 * single spaces between syllables for compound words (`con chó`,
 * `máy bay`). Rejects digits, punctuation, and underscores so the board
 * stays clean; the api-client handles the space→underscore conversion
 * internally when building ConceptNet URIs.
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
  return /^[\p{L}\p{M}]+(?: [\p{L}\p{M}]+)*$/u.test(word);
}
