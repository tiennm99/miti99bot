/**
 * @file Render a doantu board as a Telegram HTML monospace block.
 * Mirrors semantle/render.js; header text is the only visible change.
 */

import { escapeHtml } from "../../util/escape-html.js";
import { calibrate, formatWarmth, warmthEmoji } from "./format.js";

const MAX_ROWS = 15;
const LATEST_MARKER = "➡️";
const PLAIN_MARKER = "  ";

/** @typedef {import("./state.js").DoantuGuess} DoantuGuess */

/**
 * @param {DoantuGuess[]} guesses
 * @param {string|null} [latestCanonical]
 */
export function renderBoard(guesses, latestCanonical = null) {
  const count = guesses.length;
  const header = `🇻🇳 Đoán từ — ${count} guess${count === 1 ? "" : "es"}`;
  if (count === 0) {
    return `${header}\n🆕 Round ready — reply with <code>/doantu &lt;word&gt;</code>.`;
  }

  const sorted = [...guesses].sort((a, b) => b.similarity - a.similarity).slice(0, MAX_ROWS);
  const wordWidth = Math.min(20, Math.max(...sorted.map((g) => g.canonical.length)));
  const rows = sorted.map((g, i) => {
    const score = Math.round(calibrate(g.similarity));
    const marker = g.canonical === latestCanonical ? LATEST_MARKER : PLAIN_MARKER;
    const rank = String(i + 1).padStart(2);
    const warmth = formatWarmth(score).padStart(3);
    const word = escapeHtml(g.canonical.padEnd(wordWidth));
    return `${marker} ${rank}  ${warmth}  ${word} ${warmthEmoji(score)}`;
  });

  const hidden = count - sorted.length;
  const footer = hidden > 0 ? `\n…${hidden} older guess${hidden === 1 ? "" : "es"} hidden.` : "";
  return `${header}\n<pre>${rows.join("\n")}</pre>${footer}`;
}

/** @param {DoantuGuess} guess */
export function renderGuess(guess) {
  const score = Math.round(calibrate(guess.similarity));
  return `<code>${escapeHtml(guess.canonical)}</code> → ${formatWarmth(score)} ${warmthEmoji(score)}`;
}
