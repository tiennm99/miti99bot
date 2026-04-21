/**
 * @file Render comparison results as a monospace-aligned table.
 *
 * Output uses Telegram HTML parse mode wrapped in <pre> so columns line up
 * in Telegram's fixed-width font. The label column auto-widths based on the
 * longest label in the block, so new attributes drop in without re-tuning.
 *
 * Markers:
 *   🎯 the guessed champion (name row header)
 *   ✅ correct · 🟨 partial · ❌ wrong · ⬆️ / ⬇️ direction hint for year
 */

import { escapeHtml } from "../../util/escape-html.js";

const MARKER = { correct: "✅", partial: "🟨", wrong: "❌" };
const ARROW = { up: "⬆️", down: "⬇️" };
const NAME_LABEL = "Name";
const NAME_MARKER = "🎯";

/**
 * Build the label/value rows for a single guess (name row + one row per attribute).
 * @param {string} championName
 * @param {ReturnType<import("./compare.js").compareChampions>} results
 * @returns {Array<{marker: string, label: string, value: string}>}
 */
function buildRows(championName, results) {
  const rows = [{ marker: NAME_MARKER, label: NAME_LABEL, value: championName.toUpperCase() }];
  for (const r of results) {
    const marker = MARKER[r.result] ?? MARKER.wrong;
    let value = String(r.guessValue ?? "");
    if (r.key === "releaseDate" && r.result !== "correct" && r.direction) {
      const arrow = ARROW[r.direction];
      if (arrow) value = `${value} ${arrow}`;
    }
    rows.push({ marker, label: r.label, value });
  }
  return rows;
}

/**
 * Render one or more row-groups as a single aligned monospace block. Label
 * column width = max label length across ALL groups, so stacked guesses on a
 * board line up with each other.
 * @param {Array<Array<{marker: string, label: string, value: string}>>} rowGroups
 */
function formatRowGroups(rowGroups) {
  const width = Math.max(...rowGroups.flat().map((r) => r.label.length));
  const blocks = rowGroups.map((rows) =>
    rows.map((r) => `${r.marker} ${r.label.padEnd(width)} ${escapeHtml(r.value)}`).join("\n"),
  );
  return `<pre>${blocks.join("\n\n")}</pre>`;
}

/**
 * Render a single guess row-group.
 * @param {string} championName
 * @param {ReturnType<import("./compare.js").compareChampions>} results
 */
export function renderGuess(championName, results) {
  return formatRowGroups([buildRows(championName, results)]);
}

/**
 * Render the current board — all prior guesses stacked in one aligned block.
 * @param {Array<{champion: string, results: any[]}>} guesses
 */
export function renderBoard(guesses) {
  if (guesses.length === 0) {
    return "No guesses yet. Reply with <code>/loldle &lt;champion&gt;</code>.";
  }
  return formatRowGroups(guesses.map((g) => buildRows(g.champion, g.results)));
}
