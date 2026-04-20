/**
 * @file Wordle letter-by-letter comparison.
 *
 * Two-pass marking to handle duplicate letters correctly:
 *   pass 1 — mark exact positional matches as "correct" and consume those
 *            slots from the target's available pool.
 *   pass 2 — for remaining guess letters, mark "partial" if the letter still
 *            exists in the pool (and consume it), else "wrong".
 *
 * Example: target "abbey", guess "babes"
 *   → b@0 partial, a@1 partial, b@2 correct, e@3 correct, s@4 wrong.
 */

export const WORD_LENGTH = 5;

/**
 * Compare a 5-letter guess to the 5-letter target.
 * Both inputs are assumed lowercase a–z and of length WORD_LENGTH.
 *
 * @param {string} guess
 * @param {string} target
 * @returns {Array<{letter: string, result: "correct"|"partial"|"wrong"}>}
 */
export function compareWords(guess, target) {
  const results = new Array(WORD_LENGTH);
  const pool = [];

  for (let i = 0; i < WORD_LENGTH; i++) {
    if (guess[i] === target[i]) {
      results[i] = { letter: guess[i], result: "correct" };
    } else {
      pool.push(target[i]);
    }
  }

  for (let i = 0; i < WORD_LENGTH; i++) {
    if (results[i]) continue;
    const idx = pool.indexOf(guess[i]);
    if (idx !== -1) {
      pool.splice(idx, 1);
      results[i] = { letter: guess[i], result: "partial" };
    } else {
      results[i] = { letter: guess[i], result: "wrong" };
    }
  }

  return results;
}
