/**
 * @file Render wordle comparison results as a Telegram monospace grid.
 *
 * Each guess renders as two lines — colored markers over the upper-case
 * letters — wrapped in an HTML <pre> block so Telegram renders it in
 * monospace and the emoji column widths line up with the letter columns.
 *
 *   🟩 correct · 🟨 partial · ⬜ wrong
 *
 * Output strings are intended to be sent with `parse_mode: "HTML"`. All
 * game content is validated `[a-z]` so no HTML escaping is needed inside
 * the grid; only callers embedding user-controlled text around the grid
 * need to escape it.
 */

const MARKER = { correct: "🟩", partial: "🟨", wrong: "⬜" };

// Alignment under Telegram <pre>: the color markers are emoji; to guarantee
// the letter row is the same width, render letters as emoji too. Unicode
// block "Enclosed Alphanumeric Supplement" has emoji-class capitals at
// U+1F170..FF189 (🅰🅱🅲..🆉 — "Negative Squared Latin Capital Letter").
// Because both rows draw from the emoji font, column widths match 1-to-1
// on every client, regardless of the monospace font shipped with it.
const EMOJI_A = 0x1f170;
const ASCII_A = 0x41;

function toEmojiLetter(ch) {
  const code = ch.toUpperCase().charCodeAt(0);
  if (code >= ASCII_A && code <= 0x5a) {
    return String.fromCodePoint(EMOJI_A + (code - ASCII_A));
  }
  return ch;
}

function rowPair(results) {
  const markers = results.map((r) => MARKER[r.result] ?? "⬜").join("");
  const letters = results.map((r) => toEmojiLetter(r.letter)).join("");
  return `${markers}\n${letters}`;
}

/**
 * Render a single guess row as an HTML <pre> block.
 * @param {ReturnType<import("./compare.js").compareWords>} results
 */
export function renderGuess(results) {
  return `<pre>${rowPair(results)}</pre>`;
}

/**
 * Render the full board (all prior guesses, blank-line separated) as a
 * single HTML <pre> block, so all rows share one monospace code box.
 * @param {Array<{word:string, results: any[]}>} guesses
 */
export function renderBoard(guesses) {
  if (guesses.length === 0) return "No guesses yet. Reply with <code>/wordle &lt;word&gt;</code>.";
  return `<pre>${guesses.map((g) => rowPair(g.results)).join("\n\n")}</pre>`;
}
