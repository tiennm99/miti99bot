/**
 * @file Loldle command handlers.
 *
 * Subject resolution:
 *   private chat → user id (per-user game)
 *   group/supergroup chat → chat id (shared game — everyone plays together)
 *
 * Round lifecycle: a round is created lazily on the first /loldle call after
 * the previous round ended, and the timer (`startedAt`) only starts when the
 * player submits their first actual guess — viewing an empty board gives no
 * hints, so it shouldn't count against the clock.
 *
 * Commands:
 *   /loldle              → show the current board (or start a round)
 *   /loldle <champion>   → submit a guess
 *   /loldle_giveup       → reveal the answer and end the round
 *   /loldle_stats        → show wins / streak for the current subject
 */

import { escapeHtml } from "../../util/escape-html.js";
import championsData from "./champions.json" with { type: "json" };
import { compareChampions } from "./compare.js";
import { attemptFlavor, formatDuration } from "./flavor.js";
import { findChampion } from "./lookup.js";
import { renderBoard, renderGuess } from "./render.js";
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
import { GIVEUP_STICKERS, LOSE_STICKERS, WIN_STICKERS, pickSticker } from "./stickers.js";

/** @type {Array<Record<string, any>>} */
const champions = championsData;

const NEW_ROUND_HINT =
  "🆕 Send <code>/loldle</code> or <code>/loldle &lt;champion&gt;</code> to start a new round.";

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

function pickRandomChampion() {
  return champions[Math.floor(Math.random() * champions.length)];
}

function findByName(name) {
  return champions.find((c) => c.championName === name);
}

/** Recompute comparison rows for each stored guess against the current target. */
function rehydrateGuesses(game) {
  const target = findByName(game.target);
  if (!target) return [];
  const rows = [];
  for (const name of game.guesses) {
    const guess = findByName(name);
    if (!guess) continue;
    rows.push({ champion: name, results: compareChampions(guess, target) });
  }
  return rows;
}

async function startFreshGame(db, subject) {
  const target = pickRandomChampion();
  // startedAt stays null until the player actually submits their first guess —
  // seeing an empty board gives no hints, so the round hasn't really begun.
  const fresh = { target: target.championName, guesses: [], startedAt: null };
  await saveGame(db, subject, fresh);
  return fresh;
}

async function getOrInitGame(db, subject, maxGuesses) {
  const existing = await loadGame(db, subject);
  if (existing && existing.guesses.length < maxGuesses) return existing;
  return startFreshGame(db, subject);
}

/** Send a sticker, swallowing errors so a bad file_id never blocks the reply. */
async function trySendSticker(ctx, pool) {
  const sticker = pickSticker(pool);
  if (!sticker) return;
  try {
    await ctx.replyWithSticker(sticker);
  } catch {
    // Invalid file_id or transient Telegram error — the outcome text is what matters.
  }
}

/**
 * @param {import("grammy").Context} ctx
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 */
export async function handleLoldle(ctx, db) {
  const subject = getSubject(ctx);
  if (subject == null) return ctx.reply("Cannot identify chat.");
  const arg = argAfterCommand(ctx.message?.text ?? "");

  const maxGuesses = await getMaxGuesses(db, subject);
  const game = await getOrInitGame(db, subject, maxGuesses);

  if (!arg) {
    const header = `Guess ${game.guesses.length}/${maxGuesses}. Use <code>/loldle &lt;champion&gt;</code>.`;
    const board = renderBoard(rehydrateGuesses(game));
    return ctx.reply(`${header}\n\n${board}`, { parse_mode: "HTML" });
  }

  const guess = findChampion(champions, arg);
  if (!guess) return ctx.reply(`Champion not found: "${arg}".`);

  if (game.guesses.includes(guess.championName)) {
    return ctx.reply(
      `🔁 <b>${escapeHtml(guess.championName)}</b> was already guessed this round — try another champion.`,
      { parse_mode: "HTML" },
    );
  }

  const target = findByName(game.target);
  // champions.json can be refreshed between rounds — an active target may disappear.
  if (!target) {
    await clearGame(db, subject);
    return ctx.reply(`Champion data was updated since this round started. ${NEW_ROUND_HINT}`, {
      parse_mode: "HTML",
    });
  }

  const results = compareChampions(guess, target);
  // Stamp the clock on the first guess — prior /loldle views don't count.
  if (game.startedAt == null) game.startedAt = Date.now();
  game.guesses.push(guess.championName);
  const won = guess.championName === target.championName;

  const reply = renderGuess(guess.championName, results);
  const elapsed = formatDuration(Date.now() - game.startedAt);
  const champ = escapeHtml(target.championName);

  if (won) {
    const s = await recordResult(db, subject, true);
    await clearGame(db, subject);
    await trySendSticker(ctx, WIN_STICKERS);
    const flavor = attemptFlavor(game.guesses.length, maxGuesses);
    return ctx.reply(
      `${reply}\n\n🎉 ${flavor} ${champ}\n⏱ ${elapsed} · 🔥 Streak: ${s.streak} (${game.guesses.length}/${maxGuesses})\n${NEW_ROUND_HINT}`,
      { parse_mode: "HTML" },
    );
  }

  if (game.guesses.length >= maxGuesses) {
    await recordResult(db, subject, false);
    await clearGame(db, subject);
    await trySendSticker(ctx, LOSE_STICKERS);
    return ctx.reply(`${reply}\n\n❌ Out of guesses. Answer was ${champ}.\n${NEW_ROUND_HINT}`, {
      parse_mode: "HTML",
    });
  }

  await saveGame(db, subject, game);
  return ctx.reply(`${reply}\n\nGuess ${game.guesses.length}/${maxGuesses}.`, {
    parse_mode: "HTML",
  });
}

/**
 * Hidden command — set the per-subject MAX_GUESSES override (1..MAX_GUESSES_CAP).
 * Takes effect on the next round; in-progress rounds keep the old value.
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
    return ctx.reply(`Usage: /loldle_setmax <1-${MAX_GUESSES_CAP}>`);
  }
  await setMaxGuesses(db, subject, n);
  return ctx.reply(`✅ Loldle max guesses set to ${n} (applies to the next round).`);
}

/**
 * @param {import("grammy").Context} ctx
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 */
export async function handleGiveup(ctx, db) {
  const subject = getSubject(ctx);
  if (subject == null) return ctx.reply("Cannot identify chat.");
  const existing = await loadGame(db, subject);
  // No active round — nothing to give up on.
  if (!existing) {
    return ctx.reply(`No active round. ${NEW_ROUND_HINT}`, { parse_mode: "HTML" });
  }
  await recordResult(db, subject, false);
  const target = findByName(existing.target);
  await clearGame(db, subject);
  await trySendSticker(ctx, GIVEUP_STICKERS);
  const answer = target ? escapeHtml(target.championName) : escapeHtml(existing.target);
  return ctx.reply(`🏳️ Answer was ${answer}.\n${NEW_ROUND_HINT}`, { parse_mode: "HTML" });
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
