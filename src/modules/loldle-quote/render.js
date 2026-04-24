/**
 * @file Render the quote board — italic quote block + list of wrong guesses.
 * Quote text is HTML-escaped before wrapping in <i> so stray `<`, `&`, or
 * apostrophes from the data source can't break Telegram's HTML parse mode.
 */

import { escapeHtml } from "../../util/escape-html.js";

export function renderBoard(quote, guesses, max) {
  const clue = `🎭 <i>${escapeHtml(quote)}</i>`;
  if (guesses.length === 0) {
    return `${clue}\n\nNo guesses yet. Reply with <code>/loldle_quote &lt;champion&gt;</code>.`;
  }
  const lines = guesses.map((name) => `  • ${escapeHtml(name)}  ❌`).join("\n");
  return `${clue}\n\nGuesses (${guesses.length}/${max}):\n${lines}`;
}
