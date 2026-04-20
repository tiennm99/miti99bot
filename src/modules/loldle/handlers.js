/**
 * @file Command handlers for loldle module.
 *
 * - /loldle              → show board / start puzzle
 * - /loldle <champion>   → submit a guess
 * - /loldle_giveup       → reveal answer, end today's game
 * - /loldle_stats        → show personal stats
 */

import championsData from "./champions-data.js";
import { compareChampions } from "./compare.js";
import { pickDaily, todayUtc } from "./daily.js";
import { findChampion } from "./lookup.js";
import { renderBoard, renderGuess } from "./render.js";
import { loadGame, loadStats, MAX_GUESSES, recordResult, saveGame } from "./state.js";

/** @type {Array<Record<string, any>>} */
const champions = championsData;

function argAfterCommand(text) {
  if (!text) return "";
  const idx = text.indexOf(" ");
  return idx === -1 ? "" : text.slice(idx + 1).trim();
}

async function getOrInitGame(db, userId, date) {
  const existing = await loadGame(db, userId, date);
  if (existing) return existing;
  const target = pickDaily(champions, date);
  const fresh = { target: target.id, guesses: [], solved: false };
  await saveGame(db, userId, date, fresh);
  return fresh;
}

/**
 * @param {import("grammy").Context} ctx
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 */
export async function handleLoldle(ctx, db) {
  const userId = ctx.from?.id;
  if (!userId) return ctx.reply("Cannot identify user.");
  const date = todayUtc();
  const arg = argAfterCommand(ctx.message?.text ?? "");

  const game = await getOrInitGame(db, userId, date);

  if (!arg) {
    const status = game.solved
      ? `🎉 Solved in ${game.guesses.length}/${MAX_GUESSES}.`
      : game.giveup
        ? `🏳️ Gave up. Answer was ${game.target}.`
        : `Guess ${game.guesses.length}/${MAX_GUESSES}. Use \`/loldle <champion>\`.`;
    return ctx.reply(`${status}\n\n${renderBoard(game.guesses)}`);
  }

  if (game.solved || game.giveup) {
    return ctx.reply(`Today's game is over. Answer was ${game.target}.`);
  }
  if (game.guesses.length >= MAX_GUESSES) {
    return ctx.reply(`Out of guesses. Answer was ${game.target}.`);
  }

  const guess = findChampion(champions, arg);
  if (!guess) return ctx.reply(`Champion not found: "${arg}".`);

  const target = champions.find((c) => c.id === game.target);
  const results = compareChampions(guess, target);
  game.guesses.push({ champion: guess.name, results });
  const won = guess.id === target.id;
  if (won) game.solved = true;
  await saveGame(db, userId, date, game);

  const reply = renderGuess(guess.name, results);
  if (won) {
    const s = await recordResult(db, userId, date, true);
    return ctx.reply(`${reply}\n\n🎉 Solved in ${game.guesses.length}/${MAX_GUESSES}! Streak: ${s.streak}.`);
  }
  if (game.guesses.length >= MAX_GUESSES) {
    await recordResult(db, userId, date, false);
    return ctx.reply(`${reply}\n\n❌ Out of guesses. Answer was ${target.name}.`);
  }
  return ctx.reply(`${reply}\n\nGuess ${game.guesses.length}/${MAX_GUESSES}.`);
}

/**
 * @param {import("grammy").Context} ctx
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 */
export async function handleGiveup(ctx, db) {
  const userId = ctx.from?.id;
  if (!userId) return ctx.reply("Cannot identify user.");
  const date = todayUtc();
  const game = await getOrInitGame(db, userId, date);
  if (game.solved) return ctx.reply(`Already solved today — ${game.target}.`);
  if (game.giveup) return ctx.reply(`Already gave up — ${game.target}.`);
  game.giveup = true;
  await saveGame(db, userId, date, game);
  await recordResult(db, userId, date, false);
  const target = champions.find((c) => c.id === game.target);
  return ctx.reply(`🏳️ Answer was ${target.name} — ${target.title}.`);
}

/**
 * @param {import("grammy").Context} ctx
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 */
export async function handleStats(ctx, db) {
  const userId = ctx.from?.id;
  if (!userId) return ctx.reply("Cannot identify user.");
  const s = await loadStats(db, userId);
  const winRate = s.played ? Math.round((s.wins / s.played) * 100) : 0;
  return ctx.reply(
    `📊 Loldle stats\n` +
      `Played: ${s.played}\n` +
      `Wins: ${s.wins} (${winRate}%)\n` +
      `Current streak: ${s.streak}\n` +
      `Best streak: ${s.bestStreak}`,
  );
}
