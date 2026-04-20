/**
 * @file Champion pickers — deterministic daily seeding and fresh-random.
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
 * Deterministic pick seeded by date (or any string).
 * @template T
 * @param {T[]} champions
 * @param {string} [seed]
 * @returns {T}
 */
export function pickDaily(champions, seed) {
  assertNonEmpty(champions);
  const s = seed ?? todayUtc();
  return champions[hash(s) % champions.length];
}

/**
 * Uniformly random pick. `rng` defaults to Math.random — override for tests.
 * @template T
 * @param {T[]} champions
 * @param {() => number} [rng]
 * @returns {T}
 */
export function pickRandom(champions, rng = Math.random) {
  assertNonEmpty(champions);
  return champions[Math.floor(rng() * champions.length)];
}

function assertNonEmpty(arr) {
  if (!Array.isArray(arr) || arr.length === 0) {
    throw new Error("picker: champions array is empty");
  }
}
