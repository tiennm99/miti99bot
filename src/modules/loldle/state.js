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

// Default round length. The 7-axis grid leaks enough info per guess that 6
// is a fair target for typical play; admins can override per-subject via the
// hidden /loldle_setmax command (bounded by MAX_GUESSES_CAP).
const MAX_GUESSES = 6;
const MAX_GUESSES_CAP = 10;
// Upper bound for a round — long enough for any real session, short enough
// that stale KV entries get reclaimed automatically.
const GAME_TTL_SECONDS = 60 * 60 * 24 * 7;

const gameKey = (subject) => `game:${subject}`;
const statsKey = (subject) => `stats:${subject}`;
const configKey = (subject) => `config:${subject}`;

/**
 * @typedef {object} GameState
 * @property {string} target — hidden champion's championName
 * @property {string[]} guesses — championNames already tried this round
 * @property {number|null} startedAt — epoch ms; null until the first guess is
 *   submitted (viewing an empty board shouldn't start the clock)
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

/**
 * Remove the stored game so the next /loldle call creates a fresh round.
 * Used after win / loss / giveup — the new round's timer should start on the
 * player's next interaction, not at the moment the previous round ended.
 *
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {number|string} subject
 */
export async function clearGame(db, subject) {
  await db.delete(gameKey(subject));
}

export { MAX_GUESSES, MAX_GUESSES_CAP };

/**
 * Per-subject override for the round length, falling back to MAX_GUESSES.
 * Values outside [1, MAX_GUESSES_CAP] are ignored as if unset.
 *
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {number|string} subject
 * @returns {Promise<number>}
 */
export async function getMaxGuesses(db, subject) {
  const cfg = await db.getJSON(configKey(subject));
  const n = cfg?.maxGuesses;
  if (Number.isInteger(n) && n >= 1 && n <= MAX_GUESSES_CAP) return n;
  return MAX_GUESSES;
}

/**
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {number|string} subject
 * @param {number} n
 */
export async function setMaxGuesses(db, subject, n) {
  if (!Number.isInteger(n) || n < 1 || n > MAX_GUESSES_CAP) {
    throw new RangeError(`maxGuesses must be an integer in [1, ${MAX_GUESSES_CAP}]`);
  }
  await db.putJSON(configKey(subject), { maxGuesses: n });
}

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
