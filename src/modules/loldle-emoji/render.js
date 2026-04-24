/**
 * @file Render the emoji board — clue line + list of wrong guesses.
 */

import { escapeHtml } from "../../util/escape-html.js";

export function renderBoard(emojis, guesses, max) {
  const clue = `🎭 ${emojis}`;
  if (guesses.length === 0) {
    return `${clue}\n\nNo guesses yet. Reply with <code>/loldle_emoji &lt;champion&gt;</code>.`;
  }
  const lines = guesses.map((name) => `  • ${escapeHtml(name)}  ❌`).join("\n");
  return `${clue}\n\nGuesses (${guesses.length}/${max}):\n${lines}`;
}
