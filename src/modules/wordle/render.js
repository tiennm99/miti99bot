/**
 * @file Render wordle comparison results as a Telegram-friendly grid.
 *   🟩 correct · 🟨 partial · ⬜ wrong
 *
 * Each guess shows two lines: the colored markers and the upper-case letters
 * underneath so the player can read the word at a glance.
 */

const MARKER = { correct: "🟩", partial: "🟨", wrong: "⬜" };

/**
 * Render a single guess row (two lines: markers + letters).
 * @param {ReturnType<import("./compare.js").compareWords>} results
 */
export function renderGuess(results) {
  const markers = results.map((r) => MARKER[r.result] ?? "⬜").join("");
  const letters = results.map((r) => ` ${r.letter.toUpperCase()} `).join("");
  return `${markers}\n${letters}`;
}

/**
 * Render all prior guesses stacked.
 * @param {Array<{word:string, results: any[]}>} guesses
 */
export function renderBoard(guesses) {
  if (guesses.length === 0) return "No guesses yet. Reply with `/wordle <word>`.";
  return guesses.map((g) => renderGuess(g.results)).join("\n\n");
}
