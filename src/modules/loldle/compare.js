/**
 * @file Classic-mode champion comparison against the raw loldle.net schema.
 * Pure functions, no DOM/React.
 *
 * Champion records use the shape emitted by scripts/scrape-loldle-data.js
 * (identical to loldle.net's JS bundle): gender is a string ("Male"), the
 * multi-value axes (positions, species, regions, range_type) are arrays,
 * and release_date is an ISO "YYYY-MM-DD" string.
 */

export const CLASSIC_ATTRIBUTES = [
  { key: "gender", label: "Gender", type: "exact" },
  { key: "species", label: "Species", type: "multi" },
  { key: "range_type", label: "Range", type: "multi" },
  { key: "resource", label: "Resource", type: "exact" },
  { key: "regions", label: "Region", type: "multi" },
  { key: "positions", label: "Lane", type: "multi" },
  { key: "release_date", label: "Year", type: "year" },
];

/**
 * Compare a guess champion against the target.
 * @param {Record<string, unknown>} guess
 * @param {Record<string, unknown>} target
 */
export function compareChampions(guess, target) {
  return CLASSIC_ATTRIBUTES.map((attr) => {
    const guessVal = guess[attr.key];
    const targetVal = target[attr.key];

    switch (attr.type) {
      case "exact":
        return {
          ...attr,
          guessValue: formatValue(guessVal),
          targetValue: formatValue(targetVal),
          result:
            String(guessVal ?? "").toLowerCase() === String(targetVal ?? "").toLowerCase()
              ? "correct"
              : "wrong",
        };
      case "multi":
        return {
          ...attr,
          guessValue: formatValue(guessVal),
          targetValue: formatValue(targetVal),
          result: compareMultiValue(guessVal, targetVal),
        };
      case "year":
        return {
          ...attr,
          guessValue: parseYear(guessVal) || "?",
          targetValue: parseYear(targetVal) || "?",
          ...compareYear(guessVal, targetVal),
        };
      default:
        return { ...attr, guessValue: guessVal, targetValue: targetVal, result: "wrong" };
    }
  });
}

function compareMultiValue(guess, target) {
  const guessSet = toSet(guess);
  const targetSet = toSet(target);

  if (guessSet.size === 0 && targetSet.size === 0) return "correct";
  if (guessSet.size === 0 || targetSet.size === 0) return "wrong";
  if (setsEqual(guessSet, targetSet)) return "correct";
  for (const val of guessSet) {
    if (targetSet.has(val)) return "partial";
  }
  return "wrong";
}

function parseYear(val) {
  if (!val) return 0;
  const m = String(val).match(/^(\d{4})/);
  return m ? Number(m[1]) : 0;
}

function compareYear(guess, target) {
  const g = parseYear(guess);
  const t = parseYear(target);
  if (!g || !t) return { result: "wrong" };
  if (g === t) return { result: "correct" };
  return { result: "wrong", direction: g < t ? "up" : "down" };
}

function toSet(val) {
  const arr = Array.isArray(val) ? val : String(val ?? "").split(",");
  return new Set(arr.map((s) => String(s).trim().toLowerCase()).filter(Boolean));
}

function setsEqual(a, b) {
  if (a.size !== b.size) return false;
  for (const v of a) if (!b.has(v)) return false;
  return true;
}

function formatValue(val) {
  if (val == null || val === "") return "—";
  if (Array.isArray(val)) return val.length === 0 ? "—" : val.join(", ");
  return String(val);
}
