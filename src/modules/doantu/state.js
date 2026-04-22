/**
 * @file Game + stats persistence in KV, keyed by "subject"
 * (user id in DMs, chat id in groups). Target stored lowercased;
 * guesses are append-only until solve, giveup, or fresh-start.
 *
 * Key layout (inside the module-prefixed store):
 *   game:<subject>   -> { target, startedAt, solved, guesses[] }
 *   stats:<subject>  -> { played, solved, totalGuesses, bestGuessCount, lastResultAt }
 */

const GAME_TTL_SECONDS = 60 * 60 * 24 * 7;

const gameKey = (subject) => `game:${subject}`;
const statsKey = (subject) => `stats:${subject}`;

/**
 * @typedef {object} DoantuGuess
 * @property {string} word — raw user input (normalized)
 * @property {string} canonical — canonical form, lowercased
 * @property {number} similarity
 */

/**
 * @typedef {object} DoantuGameState
 * @property {string} target
 * @property {number|null} startedAt
 * @property {boolean} solved
 * @property {DoantuGuess[]} guesses
 */

/**
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {number|string} subject
 * @returns {Promise<DoantuGameState|null>}
 */
export async function loadGame(db, subject) {
  return db.getJSON(gameKey(subject));
}

/**
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {number|string} subject
 * @param {DoantuGameState} state
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
 * Record a finished round. `guessCount` counts scored guesses only —
 * OOV rejections never reach the state and don't increment.
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
