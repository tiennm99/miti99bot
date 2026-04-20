/**
 * @file Render comparison results as a Telegram-friendly plain-text grid.
 * Avoids HTML to keep escaping trivial; uses emoji markers:
 *   ✅ correct · 🟨 partial · ❌ wrong · ⬆ direction-up · ⬇ direction-down
 */

const MARKER = { correct: "✅", partial: "🟨", wrong: "❌" };

/**
 * Render a single guess row.
 * @param {string} championName
 * @param {ReturnType<import("./compare.js").compareChampions>} results
 */
export function renderGuess(championName, results) {
  const header = `🎯 ${championName}`;
  const lines = results.map((r) => {
    const mark = MARKER[r.result] ?? "❌";
    let arrow = "";
    if (r.key === "releaseDate" && r.result !== "correct") {
      if (r.direction === "up") arrow = " ⬆";
      else if (r.direction === "down") arrow = " ⬇";
    }
    return `${mark} ${r.label}: ${r.guessValue}${arrow}`;
  });
  return [header, ...lines].join("\n");
}

/**
 * Render the current board = all prior guesses stacked.
 * @param {Array<{champion:string, results: any[]}>} guesses
 */
export function renderBoard(guesses) {
  if (guesses.length === 0) return "No guesses yet. Reply with `/loldle <champion>`.";
  return guesses.map((g) => renderGuess(g.champion, g.results)).join("\n\n");
}
