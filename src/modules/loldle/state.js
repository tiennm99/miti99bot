/**
 * @file Game state in KV, keyed by "subject" (user in DM, chat in groups).
 *
 * One active round per subject at a time. Rounds are self-paced: players
 * can /loldle_new to abandon and reroll. Streak = consecutive wins.
 *
 * Key layout (inside module-prefixed store):
 *   game:<subject>   -> { target, guesses[], solved, giveup, startedAt }
 *   stats:<subject>  -> { played, wins, streak, bestStreak, lastResultAt }
 */

const MAX_GUESSES = 8;
// 7 days — a round can't linger forever, but is far longer than typical play.
const GAME_TTL_SECONDS = 60 * 60 * 24 * 7;

/** @param {number|string} subject */
const gameKey = (subject) => `game:${subject}`;
/** @param {number|string} subject */
const statsKey = (subject) => `stats:${subject}`;

/**
 * @typedef {object} GameState
 * @property {string} target — champion id
 * @property {Array<{champion:string, results:any[]}>} guesses
 * @property {boolean} solved
 * @property {boolean} [giveup]
 * @property {number} [startedAt] — epoch ms
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
      lastResultAt: null,
    }
  );
}

/**
 * Record a finished round (win/loss/giveup) and update streaks.
 * Streak increments on each win, resets to 0 on any non-win.
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
  s.lastResultAt = Date.now();
  await db.putJSON(statsKey(subject), s);
  return s;
}
