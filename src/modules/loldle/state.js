/**
 * @file Per-user daily game state in KV.
 * Key layout (inside module-prefixed store):
 *   game:<userId>:<yyyy-mm-dd>   -> { target, guesses[], solved, giveup }
 *   stats:<userId>               -> { played, wins, streak, bestStreak, lastDate }
 */

const MAX_GUESSES = 8;

/** @param {number} userId @param {string} date */
const gameKey = (userId, date) => `game:${userId}:${date}`;
/** @param {number} userId */
const statsKey = (userId) => `stats:${userId}`;

/**
 * @typedef {object} GameState
 * @property {string} target — champion id
 * @property {Array<{champion:string, results:any[]}>} guesses
 * @property {boolean} solved
 * @property {boolean} [giveup]
 */

/**
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {number} userId
 * @param {string} date
 * @returns {Promise<GameState|null>}
 */
export async function loadGame(db, userId, date) {
  return db.getJSON(gameKey(userId, date));
}

/**
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {number} userId
 * @param {string} date
 * @param {GameState} state
 */
export async function saveGame(db, userId, date, state) {
  // Expire after 3 days — stats are the only long-lived record.
  await db.putJSON(gameKey(userId, date), state, { expirationTtl: 60 * 60 * 24 * 3 });
}

export { MAX_GUESSES };

/**
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {number} userId
 */
export async function loadStats(db, userId) {
  return (
    (await db.getJSON(statsKey(userId))) ?? {
      played: 0,
      wins: 0,
      streak: 0,
      bestStreak: 0,
      lastDate: null,
    }
  );
}

/**
 * Record a finished game and update streaks.
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {number} userId
 * @param {string} date
 * @param {boolean} won
 */
export async function recordResult(db, userId, date, won) {
  const s = await loadStats(db, userId);
  s.played += 1;
  if (won) {
    s.wins += 1;
    const prev = s.lastDate;
    const yesterday = prevDate(date);
    s.streak = prev === yesterday || prev === date ? s.streak + (prev === date ? 0 : 1) : 1;
    if (s.streak > s.bestStreak) s.bestStreak = s.streak;
  } else {
    s.streak = 0;
  }
  s.lastDate = date;
  await db.putJSON(statsKey(userId), s);
  return s;
}

function prevDate(ymd) {
  const d = new Date(`${ymd}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() - 1);
  return d.toISOString().slice(0, 10);
}
