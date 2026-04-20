/**
 * @file Word pickers — deterministic daily seeding and fresh-random.
 *
 * Mirrors loldle/daily.js so both modules pick targets the same way.
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
 * @param {T[]} words
 * @param {string} [seed]
 * @returns {T}
 */
export function pickDaily(words, seed) {
  assertNonEmpty(words);
  const s = seed ?? todayUtc();
  return words[hash(s) % words.length];
}

/**
 * Uniformly random pick. `rng` defaults to Math.random — override for tests.
 * @template T
 * @param {T[]} words
 * @param {() => number} [rng]
 * @returns {T}
 */
export function pickRandom(words, rng = Math.random) {
  assertNonEmpty(words);
  return words[Math.floor(rng() * words.length)];
}

function assertNonEmpty(arr) {
  if (!Array.isArray(arr) || arr.length === 0) {
    throw new Error("picker: words array is empty");
  }
}
