/**
 * @file Classic-mode champion comparison.
 *
 * Champion records use loldle.net's raw schema: `gender` is a string
 * ("Male"/"Female"/"Other"), multi-value axes (positions, species, regions,
 * range_type) are arrays, and `release_date` is an ISO "YYYY-MM-DD" string.
 */

// Labels match loldle.net's classic-mode grid headers verbatim.
export const CLASSIC_ATTRIBUTES = [
  { key: "gender", label: "Gender", type: "exact" },
  { key: "species", label: "Species", type: "multi" },
  { key: "range_type", label: "Range type", type: "multi" },
  { key: "resource", label: "Resource", type: "exact" },
  { key: "regions", label: "Region(s)", type: "multi" },
  { key: "positions", label: "Position(s)", type: "multi" },
  { key: "release_date", label: "Release year", type: "year" },
];

/**
 * Compare a guess champion against the target, returning one row per attribute.
 * @param {Record<string, unknown>} guess
 * @param {Record<string, unknown>} target
 */
export function compareChampions(guess, target) {
  return CLASSIC_ATTRIBUTES.map((attr) => {
    const g = guess[attr.key];
    const t = target[attr.key];

    if (attr.type === "year") {
      return {
        ...attr,
        guessValue: parseYear(g) || "?",
        targetValue: parseYear(t) || "?",
        ...compareYear(g, t),
      };
    }

    const row = {
      ...attr,
      guessValue: formatValue(g),
      targetValue: formatValue(t),
    };
    row.result =
      attr.type === "exact"
        ? String(g ?? "").toLowerCase() === String(t ?? "").toLowerCase()
          ? "correct"
          : "wrong"
        : compareMultiValue(g, t);
    return row;
  });
}

function compareMultiValue(guess, target) {
  const a = toSet(guess);
  const b = toSet(target);
  if (a.size === 0 && b.size === 0) return "correct";
  if (a.size === 0 || b.size === 0) return "wrong";
  if (setsEqual(a, b)) return "correct";
  for (const v of a) if (b.has(v)) return "partial";
  return "wrong";
}

function parseYear(val) {
  const m = String(val ?? "").match(/^(\d{4})/);
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
