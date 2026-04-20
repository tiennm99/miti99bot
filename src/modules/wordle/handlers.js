/**
 * @file Command handlers for wordle module.
 *
 * Subject resolution mirrors loldle:
 *   private chat → user id (per-user game)
 *   group/supergroup chat → chat id (shared game)
 *
 * Commands:
 *   /wordle              → show board / start puzzle
 *   /wordle <word>       → submit a guess
 *   /wordle_new          → abandon current round (counts as giveup) + start fresh
 *   /wordle_giveup       → reveal answer, end current round
 *   /wordle_stats        → show stats
 */

import { WORD_LENGTH, compareWords } from "./compare.js";
import { pickRandom } from "./daily.js";
import { makeWordSet, validateGuess } from "./lookup.js";
import { renderBoard, renderGuess } from "./render.js";
import { MAX_GUESSES, loadGame, loadStats, recordResult, saveGame } from "./state.js";
import wordsData from "./words-data.js";

/** @type {string[]} */
const words = wordsData;
const wordSet = makeWordSet(words);

/**
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

async function getOrInitGame(db, subject) {
  const existing = await loadGame(db, subject);
  if (existing) return existing;
  return startFreshGame(db, subject);
}

async function startFreshGame(db, subject) {
  const target = pickRandom(words);
  const fresh = { target, guesses: [], solved: false, startedAt: Date.now() };
  await saveGame(db, subject, fresh);
  return fresh;
}

function rejectReason(reason) {
  if (reason === "empty") return `Please provide a ${WORD_LENGTH}-letter word.`;
  if (reason === "length") return `Word must be exactly ${WORD_LENGTH} letters.`;
  return "Not in the word list.";
}

/**
 * @param {import("grammy").Context} ctx
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 */
export async function handleWordle(ctx, db) {
  const subject = getSubject(ctx);
  if (subject == null) return ctx.reply("Cannot identify chat.");
  const arg = argAfterCommand(ctx.message?.text ?? "");

  const game = await getOrInitGame(db, subject);

  if (!arg) {
    const header = game.solved
      ? `🎉 Solved in ${game.guesses.length}/${MAX_GUESSES}. /wordle_new for another.`
      : game.giveup
        ? `🏳️ Gave up. Answer was ${game.target.toUpperCase()}. /wordle_new for another.`
        : `Guess ${game.guesses.length}/${MAX_GUESSES}. Use \`/wordle <word>\`.`;
    return ctx.reply(`${header}\n\n${renderBoard(game.guesses)}`);
  }

  if (isFinished(game)) {
    return ctx.reply(
      `Current round is over. Use /wordle_new to start another. Answer was ${game.target.toUpperCase()}.`,
    );
  }

  const validated = validateGuess(wordSet, arg);
  if (!validated.ok) return ctx.reply(rejectReason(validated.reason));

  const results = compareWords(validated.word, game.target);
  game.guesses.push({ word: validated.word, results });
  const won = validated.word === game.target;
  if (won) game.solved = true;
  await saveGame(db, subject, game);

  const reply = renderGuess(results);
  if (won) {
    const s = await recordResult(db, subject, true);
    return ctx.reply(
      `${reply}\n\n🎉 Solved in ${game.guesses.length}/${MAX_GUESSES}! Streak: ${s.streak}. /wordle_new for another.`,
    );
  }
  if (game.guesses.length >= MAX_GUESSES) {
    await recordResult(db, subject, false);
    return ctx.reply(
      `${reply}\n\n❌ Out of guesses. Answer was ${game.target.toUpperCase()}. /wordle_new to retry.`,
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
    prelude = `🏳️ Previous round abandoned (auto-giveup). Answer was ${prior.target.toUpperCase()}.\n\n`;
  }

  await startFreshGame(db, subject);
  return ctx.reply(`${prelude}🆕 New round started. Use \`/wordle <word>\` to guess.`);
}

/**
 * @param {import("grammy").Context} ctx
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 */
export async function handleGiveup(ctx, db) {
  const subject = getSubject(ctx);
  if (subject == null) return ctx.reply("Cannot identify chat.");
  const game = await getOrInitGame(db, subject);
  if (game.solved) return ctx.reply(`Already solved — ${game.target.toUpperCase()}.`);
  if (game.giveup) return ctx.reply(`Already gave up — ${game.target.toUpperCase()}.`);
  game.giveup = true;
  await saveGame(db, subject, game);
  await recordResult(db, subject, false);
  return ctx.reply(`🏳️ Answer was ${game.target.toUpperCase()}. /wordle_new for another.`);
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
    `📊 Wordle ${scope} stats\n` +
      `Played: ${s.played}\n` +
      `Wins: ${s.wins} (${winRate}%)\n` +
      `Current streak: ${s.streak}\n` +
      `Best streak: ${s.bestStreak}`,
  );
}
