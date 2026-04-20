/**
 * @file Command handlers for loldle module.
 *
 * Subject resolution:
 *   private chat → user id (per-user game)
 *   group/supergroup chat → chat id (shared game — all members play together)
 *
 * Commands:
 *   /loldle              → show board / start puzzle
 *   /loldle <champion>   → submit a guess
 *   /loldle_new          → abandon current round (counts as giveup) + start fresh
 *   /loldle_giveup       → reveal answer, end current round
 *   /loldle_stats        → show stats (per-user in DM, per-group in groups)
 */

import championsData from "./champions-data.js";
import { compareChampions } from "./compare.js";
import { pickRandom } from "./daily.js";
import { findChampion } from "./lookup.js";
import { renderBoard, renderGuess } from "./render.js";
import { loadGame, loadStats, MAX_GUESSES, recordResult, saveGame } from "./state.js";

/** @type {Array<Record<string, any>>} */
const champions = championsData;

/**
 * Returns the stable subject identifier for the current chat.
 * In private chat: user id. In groups: chat id (shared across all members).
 * @param {import("grammy").Context} ctx
 * @returns {number|null}
 */
function getSubject(ctx) {
  const type = ctx.chat?.type;
  if (type === "private") return ctx.from?.id ?? null;
  if (type === "group" || type === "supergroup") return ctx.chat.id;
  return ctx.from?.id ?? null;
}

function argAfterCommand(text) {
  if (!text) return "";
  const idx = text.indexOf(" ");
  return idx === -1 ? "" : text.slice(idx + 1).trim();
}

function isFinished(game) {
  return game.solved || game.giveup || game.guesses.length >= MAX_GUESSES;
}

/**
 * Load existing round, or create + persist a fresh random one.
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {number} subject
 */
async function getOrInitGame(db, subject) {
  const existing = await loadGame(db, subject);
  if (existing) return existing;
  return startFreshGame(db, subject);
}

async function startFreshGame(db, subject) {
  const target = pickRandom(champions);
  const fresh = { target: target.id, guesses: [], solved: false, startedAt: Date.now() };
  await saveGame(db, subject, fresh);
  return fresh;
}

/**
 * @param {import("grammy").Context} ctx
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 */
export async function handleLoldle(ctx, db) {
  const subject = getSubject(ctx);
  if (subject == null) return ctx.reply("Cannot identify chat.");
  const arg = argAfterCommand(ctx.message?.text ?? "");

  const game = await getOrInitGame(db, subject);

  if (!arg) {
    const header = game.solved
      ? `🎉 Solved in ${game.guesses.length}/${MAX_GUESSES}. /loldle_new for another.`
      : game.giveup
        ? `🏳️ Gave up. Answer was ${game.target}. /loldle_new for another.`
        : `Guess ${game.guesses.length}/${MAX_GUESSES}. Use \`/loldle <champion>\`.`;
    return ctx.reply(`${header}\n\n${renderBoard(game.guesses)}`);
  }

  if (isFinished(game)) {
    return ctx.reply(
      `Current round is over. Use /loldle_new to start another. Answer was ${game.target}.`,
    );
  }

  const guess = findChampion(champions, arg);
  if (!guess) return ctx.reply(`Champion not found: "${arg}".`);

  const target = champions.find((c) => c.id === game.target);
  const results = compareChampions(guess, target);
  game.guesses.push({ champion: guess.name, results });
  const won = guess.id === target.id;
  if (won) game.solved = true;
  await saveGame(db, subject, game);

  const reply = renderGuess(guess.name, results);
  if (won) {
    const s = await recordResult(db, subject, true);
    return ctx.reply(
      `${reply}\n\n🎉 Solved in ${game.guesses.length}/${MAX_GUESSES}! Streak: ${s.streak}. /loldle_new for another.`,
    );
  }
  if (game.guesses.length >= MAX_GUESSES) {
    await recordResult(db, subject, false);
    return ctx.reply(
      `${reply}\n\n❌ Out of guesses. Answer was ${target.name}. /loldle_new to retry.`,
    );
  }
  return ctx.reply(`${reply}\n\nGuess ${game.guesses.length}/${MAX_GUESSES}.`);
}

/**
 * @param {import("grammy").Context} ctx
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 */
export async function handleNew(ctx, db) {
  const subject = getSubject(ctx);
  if (subject == null) return ctx.reply("Cannot identify chat.");

  const prior = await loadGame(db, subject);
  let prelude = "";
  if (prior && !isFinished(prior)) {
    await recordResult(db, subject, false);
    const prev = champions.find((c) => c.id === prior.target);
    prelude = `🏳️ Previous round abandoned (auto-giveup). Answer was ${prev?.name ?? prior.target}.\n\n`;
  }

  await startFreshGame(db, subject);
  return ctx.reply(`${prelude}🆕 New round started. Use \`/loldle <champion>\` to guess.`);
}

/**
 * @param {import("grammy").Context} ctx
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 */
export async function handleGiveup(ctx, db) {
  const subject = getSubject(ctx);
  if (subject == null) return ctx.reply("Cannot identify chat.");
  const game = await getOrInitGame(db, subject);
  if (game.solved) return ctx.reply(`Already solved — ${game.target}.`);
  if (game.giveup) return ctx.reply(`Already gave up — ${game.target}.`);
  game.giveup = true;
  await saveGame(db, subject, game);
  await recordResult(db, subject, false);
  const target = champions.find((c) => c.id === game.target);
  return ctx.reply(`🏳️ Answer was ${target.name} — ${target.title}. /loldle_new for another.`);
}

/**
 * @param {import("grammy").Context} ctx
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 */
export async function handleStats(ctx, db) {
  const subject = getSubject(ctx);
  if (subject == null) return ctx.reply("Cannot identify chat.");
  const s = await loadStats(db, subject);
  const winRate = s.played ? Math.round((s.wins / s.played) * 100) : 0;
  const scope = ctx.chat?.type === "private" ? "your" : "group";
  return ctx.reply(
    `📊 Loldle ${scope} stats\n` +
      `Played: ${s.played}\n` +
      `Wins: ${s.wins} (${winRate}%)\n` +
      `Current streak: ${s.streak}\n` +
      `Best streak: ${s.bestStreak}`,
  );
}
