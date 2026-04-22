/**
 * @file Render a semantle board as a Telegram HTML monospace block.
 *
 * Rows sorted by similarity desc; capped at top 15 so the message stays
 * under Telegram's 4096-char limit even after hundreds of guesses. The
 * latest guess gets an arrow marker so it's easy to spot when sort order
 * shuffles it into the middle of the board.
 */

import { escapeHtml } from "../../util/escape-html.js";
import { formatWarmth, warmthEmoji } from "./format.js";

const MAX_ROWS = 15;
const LATEST_MARKER = "➡️";
const PLAIN_MARKER = "  ";

/** @typedef {import("./state.js").SemantleGuess} SemantleGuess */

/**
 * @param {SemantleGuess[]} guesses
 * @param {string|null} [latestCanonical]
 */
export function renderBoard(guesses, latestCanonical = null) {
  const count = guesses.length;
  const header = `🎯 Semantle — ${count} guess${count === 1 ? "" : "es"}`;
  if (count === 0) {
    return `${header}\n🆕 Round ready — reply with <code>/semantle &lt;word&gt;</code>.`;
  }

  const sorted = [...guesses].sort((a, b) => b.similarity - a.similarity).slice(0, MAX_ROWS);
  const wordWidth = Math.min(20, Math.max(...sorted.map((g) => g.canonical.length)));
  const rows = sorted.map((g, i) => {
    const marker = g.canonical === latestCanonical ? LATEST_MARKER : PLAIN_MARKER;
    const rank = String(i + 1).padStart(2);
    const warmth = formatWarmth(g.similarity).padStart(3);
    const word = escapeHtml(g.canonical.padEnd(wordWidth));
    return `${marker} ${rank}  ${warmth}  ${word} ${warmthEmoji(g.similarity)}`;
  });

  const hidden = count - sorted.length;
  const footer = hidden > 0 ? `\n…${hidden} older guess${hidden === 1 ? "" : "es"} hidden.` : "";
  return `${header}\n<pre>${rows.join("\n")}</pre>${footer}`;
}

/**
 * Single-line summary for the submitted guess.
 * @param {SemantleGuess} guess
 */
export function renderGuess(guess) {
  return `<code>${escapeHtml(guess.canonical)}</code> → ${formatWarmth(guess.similarity)} ${warmthEmoji(guess.similarity)}`;
}
