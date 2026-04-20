/**
 * @file Daily puzzle seeding — deterministic per UTC date.
 * Uses djb2 hash of YYYY-MM-DD to pick champion index.
 */

/** UTC date string YYYY-MM-DD. */
export function todayUtc(now = new Date()) {
  return now.toISOString().slice(0, 10);
}

/** djb2 string hash. */
function hash(str) {
  let h = 5381;
  for (let i = 0; i < str.length; i++) {
    h = (h * 33) ^ str.charCodeAt(i);
  }
  return h >>> 0;
}

/**
 * Pick today's target champion deterministically.
 * @template T
 * @param {T[]} champions
 * @param {string} [seed] — defaults to today's UTC date
 * @returns {T}
 */
export function pickDaily(champions, seed) {
  if (!Array.isArray(champions) || champions.length === 0) {
    throw new Error("pickDaily: champions array is empty");
  }
  const s = seed ?? todayUtc();
  return champions[hash(s) % champions.length];
}
