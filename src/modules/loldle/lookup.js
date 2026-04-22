/**
 * @file Champion name lookup — normalizes user input to a champion record.
 * Matches championName case/space/punct-insensitive. Falls back to prefix
 * match when unique.
 */

function normalize(s) {
  return String(s || "")
    .toLowerCase()
    .replace(/[^a-z0-9]/g, "");
}

/**
 * @param {Array<Record<string, any>>} champions
 * @param {string} input
 * @returns {Record<string, any> | null}
 */
export function findChampion(champions, input) {
  const q = normalize(input);
  if (!q) return null;

  for (const c of champions) {
    if (normalize(c.championName) === q) return c;
  }

  const prefixMatches = champions.filter((c) => normalize(c.championName).startsWith(q));
  return prefixMatches.length === 1 ? prefixMatches[0] : null;
}
