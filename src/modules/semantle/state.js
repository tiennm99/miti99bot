/**
 * @file Game + stats persistence in KV, keyed by "subject"
 * (user id in DMs, chat id in groups — so a group shares one round).
 *
 * Target is stored lowercased so the case-insensitive equality check
 * is a single compare. Unlimited guesses — no MAX cap; rounds end only
 * on solve or giveup.
 *
 * Key layout (inside the module-prefixed store):
 *   game:<subject>   -> { target, startedAt, solved, guesses[] }
 *   stats:<subject>  -> { played, solved, totalGuesses, bestGuessCount, lastResultAt }
 */

// Long enough for any real session, short enough that stale rounds reclaim.
const GAME_TTL_SECONDS = 60 * 60 * 24 * 7;

const gameKey = (subject) => `game:${subject}`;
const statsKey = (subject) => `stats:${subject}`;

/**
 * @typedef {object} SemantleGuess
 * @property {string} word — raw user input (normalized)
 * @property {string} canonical — model's canonical form of the guess, lowercased
 * @property {number} similarity — cosine ∈ [-1, 1]
 */

/**
 * @typedef {object} SemantleGameState
 * @property {string} target — secret word (lowercased)
 * @property {number|null} startedAt — epoch ms; null until the first real guess
 * @property {boolean} solved
 * @property {SemantleGuess[]} guesses
 */

/**
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {number|string} subject
 * @returns {Promise<SemantleGameState|null>}
 */
export async function loadGame(db, subject) {
  return db.getJSON(gameKey(subject));
}

/**
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {number|string} subject
 * @param {SemantleGameState} state
 */
export async function saveGame(db, subject, state) {
  await db.putJSON(gameKey(subject), state, { expirationTtl: GAME_TTL_SECONDS });
}

/**
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {number|string} subject
 */
export async function clearGame(db, subject) {
  await db.delete(gameKey(subject));
}

/**
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {number|string} subject
 */
export async function loadStats(db, subject) {
  return (
    (await db.getJSON(statsKey(subject))) ?? {
      played: 0,
      solved: 0,
      totalGuesses: 0,
      bestGuessCount: null,
      lastResultAt: null,
    }
  );
}

/**
 * Record a finished round. `guessCount` counts scored guesses only — OOV
 * rejections never reached the state, so they're not represented here.
 *
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {number|string} subject
 * @param {{ solved: boolean, guessCount: number }} outcome
 */
export async function recordResult(db, subject, { solved, guessCount }) {
  const s = await loadStats(db, subject);
  s.played += 1;
  s.totalGuesses += guessCount;
  if (solved) {
    s.solved += 1;
    if (s.bestGuessCount === null || guessCount < s.bestGuessCount) {
      s.bestGuessCount = guessCount;
    }
  }
  s.lastResultAt = Date.now();
  await db.putJSON(statsKey(subject), s);
  return s;
}
