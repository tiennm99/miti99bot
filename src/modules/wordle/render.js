/**
 * @file Render wordle comparison results for Telegram.
 *
 * Uses the NYT Wordle share format — guess word on one line, colored marker
 * row below — so there's no cross-client column-alignment dependency:
 *
 *   CRANE
 *   🟩🟨⬜🟩🟩
 *
 *   🟩 correct · 🟨 partial · ⬜ wrong
 *
 * Output is plain text; no HTML parse mode required.
 */

const MARKER = { correct: "🟩", partial: "🟨", wrong: "⬜" };

function rowPair({ word, results }) {
  const markers = results.map((r) => MARKER[r.result] ?? "⬜").join("");
  return `${word.toUpperCase()}\n${markers}`;
}

/**
 * Render a single guess (word over colors).
 * @param {string} word — the submitted guess (lowercase a-z)
 * @param {ReturnType<import("./compare.js").compareWords>} results
 */
export function renderGuess(word, results) {
  return rowPair({ word, results });
}

/**
 * Render the full board (all prior guesses, blank-line separated).
 * @param {Array<{word:string, results: any[]}>} guesses
 */
export function renderBoard(guesses) {
  if (guesses.length === 0) return "No guesses yet. Reply with `/wordle <word>`.";
  return guesses.map((g) => rowPair(g)).join("\n\n");
}
