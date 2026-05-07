/**
 * @file loldle-quote command handlers. Binary right/wrong; 6 guesses.
 * Subject resolution and round lifecycle mirror loldle-emoji.
 */

import { escapeHtml } from "../../util/escape-html.js";
import { findChampion } from "./lookup.js";
import quotesData from "./quotes.json" with { type: "json" };
import { renderBoard } from "./render.js";
import {
  MAX_GUESSES_CAP,
  clearGame,
  getMaxGuesses,
  loadGame,
  loadStats,
  recordResult,
  saveGame,
  setMaxGuesses,
} from "./state.js";

const POOL = quotesData.filter((c) => c.quote && c.quote.trim().length > 0);

const NEW_ROUND_HINT =
  "🆕 Send <code>/loldle_quote</code> or <code>/loldle_quote &lt;champion&gt;</code> to start a new round.";

function getSubject(ctx) {
  const type = ctx.chat?.type;
  if (type === "group" || type === "supergroup") return ctx.chat.id;
  return ctx.from?.id ?? null;
}

function argAfterCommand(text) {
  if (!text) return "";
  const idx = text.indexOf(" ");
  return idx === -1 ? "" : text.slice(idx + 1).trim();
}

function pickRandom() {
  return POOL[Math.floor(Math.random() * POOL.length)];
}

function findByName(name) {
  return POOL.find((c) => c.championName === name);
}

async function startFreshGame(db, subject) {
  const target = pickRandom();
  const fresh = { target: target.championName, guesses: [], startedAt: null };
  await saveGame(db, subject, fresh);
  return fresh;
}

async function getOrInitGame(db, subject, maxGuesses) {
  const existing = await loadGame(db, subject);
  if (existing && existing.guesses.length < maxGuesses) return existing;
  return startFreshGame(db, subject);
}

export async function handleQuote(ctx, db) {
  const subject = getSubject(ctx);
  if (subject == null) return ctx.reply("Cannot identify chat.");
  const arg = argAfterCommand(ctx.message?.text ?? "");

  const maxGuesses = await getMaxGuesses(db, subject);
  const game = await getOrInitGame(db, subject, maxGuesses);
  const target = findByName(game.target);
  if (!target) {
    await clearGame(db, subject);
    return ctx.reply(`Quote data was updated since this round started. ${NEW_ROUND_HINT}`, {
      parse_mode: "HTML",
    });
  }

  if (!arg) {
    return ctx.reply(renderBoard(target.quote, game.guesses, maxGuesses), {
      parse_mode: "HTML",
    });
  }

  const guess = findChampion(POOL, arg);
  if (!guess) return ctx.reply(`Champion not found: "${arg}".`);

  if (game.guesses.includes(guess.championName)) {
    return ctx.reply(
      `🔁 <b>${escapeHtml(guess.championName)}</b> was already guessed this round — try another champion.`,
      { parse_mode: "HTML" },
    );
  }

  if (game.startedAt == null) game.startedAt = Date.now();
  game.guesses.push(guess.championName);
  const won = guess.championName === target.championName;
  const answer = escapeHtml(target.championName);

  if (won) {
    const s = await recordResult(db, subject, true);
    await clearGame(db, subject);
    return ctx.reply(
      `🎉 Nailed it! <b>${answer}</b> — solved in ${game.guesses.length}/${maxGuesses}\n🔥 Streak: ${s.streak}\n${NEW_ROUND_HINT}`,
      { parse_mode: "HTML" },
    );
  }

  if (game.guesses.length >= maxGuesses) {
    await recordResult(db, subject, false);
    await clearGame(db, subject);
    return ctx.reply(
      `${renderBoard(target.quote, game.guesses, maxGuesses)}\n\n❌ Out of guesses. Answer: <b>${answer}</b>.\n${NEW_ROUND_HINT}`,
      { parse_mode: "HTML" },
    );
  }

  await saveGame(db, subject, game);
  return ctx.reply(
    `${renderBoard(target.quote, game.guesses, maxGuesses)}\n\n❌ Not <b>${escapeHtml(guess.championName)}</b>. Guess ${game.guesses.length}/${maxGuesses}.`,
    { parse_mode: "HTML" },
  );
}

/**
 * Hidden — set the per-subject MAX_GUESSES override (1..MAX_GUESSES_CAP).
 *
 * @param {import("grammy").Context} ctx
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 */
export async function handleSetMax(ctx, db) {
  const subject = getSubject(ctx);
  if (subject == null) return ctx.reply("Cannot identify chat.");
  const arg = argAfterCommand(ctx.message?.text ?? "");
  const n = Number.parseInt(arg, 10);
  if (!Number.isInteger(n) || n < 1 || n > MAX_GUESSES_CAP) {
    return ctx.reply(`Usage: /loldle_quote_setmax <1-${MAX_GUESSES_CAP}>`);
  }
  await setMaxGuesses(db, subject, n);
  return ctx.reply(`✅ Loldle quote max guesses set to ${n} (applies to the next round).`);
}

export async function handleGiveup(ctx, db) {
  const subject = getSubject(ctx);
  if (subject == null) return ctx.reply("Cannot identify chat.");
  const existing = await loadGame(db, subject);
  if (!existing) {
    return ctx.reply(`No active round. ${NEW_ROUND_HINT}`, { parse_mode: "HTML" });
  }
  await recordResult(db, subject, false);
  await clearGame(db, subject);
  return ctx.reply(`🏳️ Answer: <b>${escapeHtml(existing.target)}</b>.\n${NEW_ROUND_HINT}`, {
    parse_mode: "HTML",
  });
}

export async function handleStats(ctx, db) {
  const subject = getSubject(ctx);
  if (subject == null) return ctx.reply("Cannot identify chat.");
  const s = await loadStats(db, subject);
  const winRate = s.played ? Math.round((s.wins / s.played) * 100) : 0;
  const scope = ctx.chat?.type === "private" ? "your" : "group";
  return ctx.reply(
    `📊 Loldle Quote ${scope} stats\n` +
      `Played: ${s.played}\n` +
      `Wins: ${s.wins} (${winRate}%)\n` +
      `Current streak: ${s.streak}\n` +
      `Best streak: ${s.bestStreak}`,
  );
}
