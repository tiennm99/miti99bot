/**
 * @file Game + stats persistence in KV, keyed by "subject"
 * (user id in DMs, chat id in groups — so a group shares one round).
 *
 * Key layout (inside the module-prefixed store):
 *   game:<subject>   -> { target, guesses, startedAt }
 *   stats:<subject>  -> { played, wins, streak, bestStreak }
 *
 * `guesses` is a string[] of championNames — comparison rows are recomputed
 * at render time, so the board always reflects the live champions.json.
 */

const MAX_GUESSES = 8;
// Upper bound for a round — long enough for any real session, short enough
// that stale KV entries get reclaimed automatically.
const GAME_TTL_SECONDS = 60 * 60 * 24 * 7;

const gameKey = (subject) => `game:${subject}`;
const statsKey = (subject) => `stats:${subject}`;

/**
 * @typedef {object} GameState
 * @property {string} target — hidden champion's championName
 * @property {string[]} guesses — championNames already tried this round
 * @property {number} startedAt — epoch ms
 */

/**
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {number|string} subject
 * @returns {Promise<GameState|null>}
 */
export async function loadGame(db, subject) {
  return db.getJSON(gameKey(subject));
}

/**
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {number|string} subject
 * @param {GameState} state
 */
export async function saveGame(db, subject, state) {
  await db.putJSON(gameKey(subject), state, { expirationTtl: GAME_TTL_SECONDS });
}

export { MAX_GUESSES };

/**
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {number|string} subject
 */
export async function loadStats(db, subject) {
  return (
    (await db.getJSON(statsKey(subject))) ?? {
      played: 0,
      wins: 0,
      streak: 0,
      bestStreak: 0,
    }
  );
}

/**
 * Record a finished round and update the streak. Streak increments on each
 * win and resets to 0 on any non-win.
 *
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {number|string} subject
 * @param {boolean} won
 */
export async function recordResult(db, subject, won) {
  const s = await loadStats(db, subject);
  s.played += 1;
  if (won) {
    s.wins += 1;
    s.streak += 1;
    if (s.streak > s.bestStreak) s.bestStreak = s.streak;
  } else {
    s.streak = 0;
  }
  await db.putJSON(statsKey(subject), s);
  return s;
}
