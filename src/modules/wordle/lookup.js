/**
 * @file Wordle word validation — normalizes user input and checks membership.
 *
 * Normalization: lowercase, strip non-a-z. Valid words are exactly 5 letters
 * (matching WORD_LENGTH) and present in the provided dictionary. The dictionary
 * is checked via a Set built once per call-site (see makeWordSet below).
 */

import { WORD_LENGTH } from "./compare.js";

/**
 * Lowercase + strip anything that isn't a–z.
 * @param {string} input
 */
export function normalizeWord(input) {
  return String(input || "")
    .toLowerCase()
    .replace(/[^a-z]/g, "");
}

/**
 * Build a Set from an array of words for O(1) membership checks.
 * @param {string[]} words
 */
export function makeWordSet(words) {
  return new Set(words);
}

/**
 * Validate a guess against the dictionary.
 *
 * Returns a discriminated result so the caller can tell *why* validation failed:
 *   - "empty"   — input was blank or stripped to nothing.
 *   - "length"  — normalized word is not WORD_LENGTH letters.
 *   - "unknown" — right length, but not in the dictionary.
 *
 * @param {Set<string>} wordSet
 * @param {string} input
 * @returns {{ok: true, word: string} | {ok: false, reason: "empty"|"length"|"unknown", word: string}}
 */
export function validateGuess(wordSet, input) {
  const word = normalizeWord(input);
  if (!word) return { ok: false, reason: "empty", word };
  if (word.length !== WORD_LENGTH) return { ok: false, reason: "length", word };
  if (!wordSet.has(word)) return { ok: false, reason: "unknown", word };
  return { ok: true, word };
}
