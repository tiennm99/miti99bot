/**
 * @file Classic-mode champion comparison — ported from tiennm99/loldle
 * (lib/classic-mode.js). Pure functions, no DOM/React.
 */

export const CLASSIC_ATTRIBUTES = [
  { key: "gender", label: "Gender", type: "exact" },
  { key: "genre", label: "Genre", type: "multi" },
  { key: "attackType", label: "Range", type: "exact" },
  { key: "resource", label: "Resource", type: "exact" },
  { key: "region", label: "Region", type: "exact" },
  { key: "lane", label: "Lane", type: "multi" },
  { key: "releaseDate", label: "Year", type: "year" },
];

/**
 * Compare a guess champion against the target.
 * @param {Record<string, unknown>} guess
 * @param {Record<string, unknown>} target
 */
export function compareChampions(guess, target) {
  return CLASSIC_ATTRIBUTES.map((attr) => {
    const guessVal = guess[attr.key] ?? "";
    const targetVal = target[attr.key] ?? "";

    switch (attr.type) {
      case "exact":
        return {
          ...attr,
          guessValue: formatValue(attr.key, guessVal),
          targetValue: formatValue(attr.key, targetVal),
          result:
            String(guessVal).toLowerCase() === String(targetVal).toLowerCase()
              ? "correct"
              : "wrong",
        };
      case "multi":
        return {
          ...attr,
          guessValue: formatValue(attr.key, guessVal),
          targetValue: formatValue(attr.key, targetVal),
          result: compareMultiValue(guessVal, targetVal),
        };
      case "year":
        return {
          ...attr,
          guessValue: guessVal || "?",
          targetValue: targetVal || "?",
          ...compareYear(guessVal, targetVal),
        };
      default:
        return { ...attr, guessValue: guessVal, targetValue: targetVal, result: "wrong" };
    }
  });
}

function compareMultiValue(guessStr, targetStr) {
  const guessSet = parseSet(guessStr);
  const targetSet = parseSet(targetStr);

  if (guessSet.size === 0 && targetSet.size === 0) return "correct";
  if (guessSet.size === 0 || targetSet.size === 0) return "wrong";
  if (setsEqual(guessSet, targetSet)) return "correct";
  for (const val of guessSet) {
    if (targetSet.has(val)) return "partial";
  }
  return "wrong";
}

function compareYear(guessYear, targetYear) {
  const g = Number(guessYear);
  const t = Number(targetYear);
  if (!g || !t) return { result: "wrong" };
  if (g === t) return { result: "correct" };
  return { result: "wrong", direction: g < t ? "up" : "down" };
}

function parseSet(str) {
  if (!str) return new Set();
  return new Set(
    String(str)
      .split(",")
      .map((s) => s.trim().toLowerCase())
      .filter(Boolean),
  );
}

function setsEqual(a, b) {
  if (a.size !== b.size) return false;
  for (const val of a) {
    if (!b.has(val)) return false;
  }
  return true;
}

function formatValue(key, value) {
  if (!value) return "—";
  const str = String(value);
  switch (key) {
    case "gender":
      return capitalize(str);
    case "attackType":
      return str === "close" ? "Melee" : "Ranged";
    case "region":
      return str.split("-").map(capitalize).join(" ");
    case "genre":
    case "lane":
      return str
        .split(",")
        .map((s) => capitalize(s.trim()))
        .join(", ");
    default:
      return str;
  }
}

function capitalize(str) {
  return str.charAt(0).toUpperCase() + str.slice(1);
}
