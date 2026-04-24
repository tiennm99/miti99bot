/**
 * @file Telegram-HTML renderers for the twentyq game.
 * All target/hint/text values are HTML-escaped before splicing.
 */

import { escapeHtml } from "../../util/escape-html.js";

const MAX_TURN_ROWS = 10;
const ANSWER_EMOJI = { yes: "✅", no: "❌" };

/** @typedef {import("./state.js").TwentyqGameState} TwentyqGameState */
/** @typedef {import("./state.js").TwentyqStats} TwentyqStats */
/** @typedef {import("./state.js").TwentyqTurn} TwentyqTurn */

/**
 * Initial game-start message: category + initial hint.
 * @param {TwentyqGameState} state
 */
export function formatIntro(state) {
  return [
    `🎯 I'm thinking of <b>a ${escapeHtml(state.category)}</b>.`,
    `Hint: ${escapeHtml(state.initialHint)}`,
    "",
    "Ask yes/no questions with <code>/twentyq is it ...?</code>",
  ].join("\n");
}

/**
 * Reply for one answered turn.
 * @param {{ turn: TwentyqTurn, solved: boolean, target: string, turnCount: number }} args
 */
export function formatTurnReply({ turn, solved, target, turnCount }) {
  if (solved) {
    return [
      `🎉 Correct! It was <b>${escapeHtml(target)}</b>.`,
      `Solved in ${turnCount} question${turnCount === 1 ? "" : "s"}.`,
    ].join("\n");
  }
  const emoji = ANSWER_EMOJI[turn.answer] ?? "❓";
  const head = turn.isGuess
    ? `${emoji} Not quite. Hint: ${escapeHtml(turn.hint)}`
    : `${emoji} ${turn.answer === "yes" ? "Yes" : "No"}. Hint: ${escapeHtml(turn.hint)}`;
  return head;
}

/**
 * Board view: initial hint + numbered Q/A list.
 * @param {TwentyqGameState} state
 */
export function formatBoard(state) {
  const header = `🎯 Category: <b>${escapeHtml(state.category)}</b>`;
  const intro = `Initial hint: ${escapeHtml(state.initialHint)}`;
  if (state.turns.length === 0) {
    return [header, intro, "", "<i>No questions yet — go ahead and ask one.</i>"].join("\n");
  }
  const recent = state.turns.slice(-MAX_TURN_ROWS);
  const startNo = state.turns.length - recent.length + 1;
  const lines = recent.map((t, i) => {
    const num = String(startNo + i).padStart(2);
    const ans = ANSWER_EMOJI[t.answer] ?? "❓";
    const q = escapeHtml(t.text);
    const h = escapeHtml(t.hint);
    return `${num}. ${ans} <b>${q}</b>\n     ${h}`;
  });
  const hidden = state.turns.length - recent.length;
  const footer = hidden > 0 ? `\n…${hidden} earlier turn${hidden === 1 ? "" : "s"} hidden.` : "";
  return [header, intro, "", lines.join("\n")].join("\n") + footer;
}

/**
 * Reveal-on-giveup message.
 * @param {TwentyqGameState} state
 */
export function formatGiveup(state) {
  return [
    `🏳️ Gave up. The answer was <b>${escapeHtml(state.target)}</b>.`,
    "Send <code>/twentyq</code> to start a fresh round.",
  ].join("\n");
}

/**
 * Stats summary.
 * @param {TwentyqStats} stats
 */
export function formatStats(stats) {
  if (stats.played === 0) return "No twentyq games played yet.";
  const solveRate = Math.round((stats.solved / stats.played) * 100);
  const avg = stats.played > 0 ? Math.round(stats.totalTurns / stats.played) : "—";
  return [
    "🎯 <b>Twentyq stats</b>",
    `Played: ${stats.played}`,
    `Solved: ${stats.solved} (${solveRate}%)`,
    `Total questions: ${stats.totalTurns}`,
    `Fewest to solve: ${stats.bestTurnCount ?? "—"}`,
    `Avg per round: ${avg}`,
  ].join("\n");
}
