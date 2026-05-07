/**
 * @file Game + stats persistence for loldle-ability. Adds `slot` field vs
 * emoji/quote so the SAME ability icon persists across guesses in a round.
 */

const MAX_GUESSES = 5;
const MAX_GUESSES_CAP = 10;
const GAME_TTL_SECONDS = 60 * 60 * 24 * 7;

const gameKey = (subject) => `game:${subject}`;
const statsKey = (subject) => `stats:${subject}`;
const configKey = (subject) => `config:${subject}`;

export { MAX_GUESSES, MAX_GUESSES_CAP };

export async function getMaxGuesses(db, subject) {
  const cfg = await db.getJSON(configKey(subject));
  const n = cfg?.maxGuesses;
  if (Number.isInteger(n) && n >= 1 && n <= MAX_GUESSES_CAP) return n;
  return MAX_GUESSES;
}

export async function setMaxGuesses(db, subject, n) {
  if (!Number.isInteger(n) || n < 1 || n > MAX_GUESSES_CAP) {
    throw new RangeError(`maxGuesses must be an integer in [1, ${MAX_GUESSES_CAP}]`);
  }
  await db.putJSON(configKey(subject), { maxGuesses: n });
}

export async function loadGame(db, subject) {
  return db.getJSON(gameKey(subject));
}

export async function saveGame(db, subject, state) {
  await db.putJSON(gameKey(subject), state, { expirationTtl: GAME_TTL_SECONDS });
}

export async function clearGame(db, subject) {
  await db.delete(gameKey(subject));
}

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
