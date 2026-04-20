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

// In a Telegram <pre> block, color-square emoji render at ~2 monospace cells
// wide, while ASCII letters render at 1 cell — so a plain "A B C" row never
// lines up under the markers. Fullwidth Latin (U+FF21..U+FF3A) is East Asian
// Width = Fullwidth, which renders at exactly 2 cells per character and
// matches the emoji column width 1-to-1.
const FULLWIDTH_OFFSET = 0xff21 - 0x41; // 'A' (0x41) → 'Ａ' (0xFF21)

function toFullwidthUpper(ch) {
  const code = ch.toUpperCase().charCodeAt(0);
  if (code >= 0x41 && code <= 0x5a) return String.fromCharCode(code + FULLWIDTH_OFFSET);
  return ch;
}

function rowPair(results) {
  const markers = results.map((r) => MARKER[r.result] ?? "⬜").join("");
  const letters = results.map((r) => toFullwidthUpper(r.letter)).join("");
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
