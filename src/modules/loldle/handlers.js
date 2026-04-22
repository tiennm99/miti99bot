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
 *   /loldle_giveup       → reveal answer, end round (a fresh round auto-starts)
 *   /loldle_stats        → show stats (per-user in DM, per-group in groups)
 *
 * A finished round (solved, gave up, or out of guesses) is immediately
 * replaced by a fresh round, so the user can just keep playing.
 */

import { escapeHtml } from "../../util/escape-html.js";
import championsData from "./champions-data.js";
import { compareChampions } from "./compare.js";
import { pickRandom } from "./daily.js";
import { attemptFlavor, formatDuration } from "./flavor.js";
import { findChampion } from "./lookup.js";
import { renderBoard, renderGuess } from "./render.js";
import { MAX_GUESSES, loadGame, loadStats, recordResult, saveGame } from "./state.js";
import { GIVEUP_STICKERS, LOSE_STICKERS, WIN_STICKERS, pickSticker } from "./stickers.js";

/** @type {Array<Record<string, any>>} */
const champions = championsData;

// Sent inside HTML parse_mode replies — must be HTML-safe.
// `<champion>` as a literal tag would make Telegram reject the whole message.
const NEW_ROUND_HINT = "🆕 New round started. Use <code>/loldle &lt;champion&gt;</code> to guess.";

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
 * A previously-finished round is discarded and replaced with a fresh one so
 * the game auto-continues without needing a manual "new round" command.
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {number} subject
 */
async function getOrInitGame(db, subject) {
  const existing = await loadGame(db, subject);
  if (existing && !isFinished(existing)) return existing;
  return startFreshGame(db, subject);
}

async function startFreshGame(db, subject) {
  const target = pickRandom(champions);
  const fresh = {
    target: target.id,
    guesses: [],
    solved: false,
    giveup: false,
    startedAt: Date.now(),
  };
  await saveGame(db, subject, fresh);
  return fresh;
}

/**
 * Send a random sticker from the pool, swallowing errors so a rotten file_id
 * (Telegram rejection) never blocks the follow-up text reply.
 *
 * @param {import("grammy").Context} ctx
 * @param {readonly string[]} pool
 */
async function trySendSticker(ctx, pool) {
  const sticker = pickSticker(pool);
  if (!sticker) return;
  try {
    await ctx.replyWithSticker(sticker);
  } catch {
    // Ignore — the outcome text reply is what matters. An invalid file_id
    // or transient Telegram error must not derail the game flow.
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

  const game = await getOrInitGame(db, subject);

  if (!arg) {
    const header = `Guess ${game.guesses.length}/${MAX_GUESSES}. Use <code>/loldle &lt;champion&gt;</code>.`;
    return ctx.reply(`${header}\n\n${renderBoard(game.guesses)}`, { parse_mode: "HTML" });
  }

  const guess = findChampion(champions, arg);
  if (!guess) return ctx.reply(`Champion not found: "${arg}".`);

  if (game.guesses.some((g) => g.champion === guess.name)) {
    return ctx.reply(
      `🔁 <b>${escapeHtml(guess.name)}</b> was already guessed this round — try another champion.`,
      { parse_mode: "HTML" },
    );
  }

  const target = champions.find((c) => c.id === game.target);
  // champions.json can be refreshed between rounds — an active target may disappear.
  if (!target) {
    await startFreshGame(db, subject);
    return ctx.reply(
      "Champion data was updated since this round started. Starting a fresh round — try again.",
    );
  }
  const results = compareChampions(guess, target);
  game.guesses.push({ champion: guess.name, results });
  const won = guess.id === target.id;
  if (won) game.solved = true;
  await saveGame(db, subject, game);

  const reply = renderGuess(guess.name, results);
  const elapsed = formatDuration(Date.now() - (game.startedAt ?? Date.now()));
  const champ = `${escapeHtml(target.name)} — ${escapeHtml(target.title)}`;

  if (won) {
    const s = await recordResult(db, subject, true);
    await startFreshGame(db, subject);
    await trySendSticker(ctx, WIN_STICKERS);
    const flavor = attemptFlavor(game.guesses.length, MAX_GUESSES);
    return ctx.reply(
      `${reply}\n\n🎉 ${flavor} ${champ}\n⏱ ${elapsed} · 🔥 Streak: ${s.streak} (${game.guesses.length}/${MAX_GUESSES})\n${NEW_ROUND_HINT}`,
      { parse_mode: "HTML" },
    );
  }
  if (game.guesses.length >= MAX_GUESSES) {
    await recordResult(db, subject, false);
    await startFreshGame(db, subject);
    await trySendSticker(ctx, LOSE_STICKERS);
    return ctx.reply(`${reply}\n\n❌ Out of guesses. Answer was ${champ}.\n${NEW_ROUND_HINT}`, {
      parse_mode: "HTML",
    });
  }
  return ctx.reply(`${reply}\n\nGuess ${game.guesses.length}/${MAX_GUESSES}.`, {
    parse_mode: "HTML",
  });
}

/**
 * @param {import("grammy").Context} ctx
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 */
export async function handleGiveup(ctx, db) {
  const subject = getSubject(ctx);
  if (subject == null) return ctx.reply("Cannot identify chat.");
  const game = await getOrInitGame(db, subject);
  // getOrInitGame guarantees the returned game is unfinished, so mark + record.
  game.giveup = true;
  await saveGame(db, subject, game);
  await recordResult(db, subject, false);
  const target = champions.find((c) => c.id === game.target);
  await startFreshGame(db, subject);
  await trySendSticker(ctx, GIVEUP_STICKERS);
  const answer = target
    ? `${escapeHtml(target.name)} — ${escapeHtml(target.title)}`
    : escapeHtml(game.target);
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
