/**
 * @file Game + stats persistence in KV, keyed by "subject"
 * (user id in DMs, chat id in groups). Mirrors doantu/state.js shape;
 * `turns[]` instead of `guesses[]` because each entry holds Q + A + hint.
 *
 * Key layout (inside the module-prefixed store):
 *   game:<subject>   -> { category, target, initialHint, startedAt, solved, turns[] }
 *   stats:<subject>  -> { played, solved, totalTurns, bestTurnCount, lastResultAt }
 */

const GAME_TTL_SECONDS = 60 * 60 * 24 * 7;

const gameKey = (subject) => `game:${subject}`;
const statsKey = (subject) => `stats:${subject}`;

/**
 * @typedef {object} TwentyqTurn
 * @property {string} text       — raw user input (trimmed)
 * @property {boolean} isGuess   — model classified as a final-guess attempt
 * @property {"yes"|"no"} answer
 * @property {string} hint
 * @property {number} ts         — Date.now() when recorded
 */

/**
 * @typedef {object} TwentyqGameState
 * @property {string} category
 * @property {string} target            — lowercased
 * @property {string} initialHint
 * @property {number|null} startedAt
 * @property {boolean} solved
 * @property {TwentyqTurn[]} turns
 */

/**
 * @typedef {object} TwentyqStats
 * @property {number} played
 * @property {number} solved
 * @property {number} totalTurns
 * @property {number|null} bestTurnCount
 * @property {number|null} lastResultAt
 */

/**
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {number|string} subject
 * @returns {Promise<TwentyqGameState|null>}
 */
export async function loadGame(db, subject) {
  return db.getJSON(gameKey(subject));
}

/**
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {number|string} subject
 * @param {TwentyqGameState} state
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
 * @returns {Promise<TwentyqStats>}
 */
export async function loadStats(db, subject) {
  return (
    (await db.getJSON(statsKey(subject))) ?? {
      played: 0,
      solved: 0,
      totalTurns: 0,
      bestTurnCount: null,
      lastResultAt: null,
    }
  );
}

/**
 * Record a finished round. `turnCount` counts scored Q&A turns only —
 * validator-rejected inputs and repeat-question dedups never reach state.
 *
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {number|string} subject
 * @param {{ solved: boolean, turnCount: number }} outcome
 */
export async function recordResult(db, subject, { solved, turnCount }) {
  const s = await loadStats(db, subject);
  s.played += 1;
  s.totalTurns += turnCount;
  if (solved) {
    s.solved += 1;
    if (s.bestTurnCount === null || turnCount < s.bestTurnCount) {
      s.bestTurnCount = turnCount;
    }
  }
  s.lastResultAt = Date.now();
  await db.putJSON(statsKey(subject), s);
  return s;
}
