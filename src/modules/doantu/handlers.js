/**
 * @file Command handlers for the doantu module (Vietnamese semantle).
 *
 * Subject resolution mirrors loldle/semantle:
 *   private chat           → user id (per-user game)
 *   group/supergroup chat  → chat id (shared game)
 *
 * Commands (all protected — listed in /help but not pushed to the Telegram
 * native / menu):
 *   /doantu              → show the board (or lazy-start a round)
 *   /doantu <word>       → submit a guess
 *   /doantu_giveup       → reveal target (next /doantu auto-starts fresh)
 *   /doantu_stats        → show per-subject stats
 */

import { escapeHtml } from "../../util/escape-html.js";
import { UpstreamError } from "./api-client.js";
import { isValidShape, normalize } from "./lookup.js";
import { renderBoard, renderGuess } from "./render.js";
import { clearGame, loadGame, loadStats, recordResult, saveGame } from "./state.js";

const UPSTREAM_FAIL = "⚠️ Upstream hiccup — try again in a few seconds.";

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

function logFail(stage, err) {
  console.log(
    JSON.stringify({
      msg: "doantu_upstream_fail",
      stage,
      err: err instanceof UpstreamError ? { status: err.status, body: err.body } : String(err),
    }),
  );
}

async function startFreshGame(db, client, subject) {
  const picked = await client.randomWord();
  const target = String(picked?.word ?? "").toLowerCase();
  if (!target) throw new UpstreamError("empty target from randomWord");
  const fresh = { target, startedAt: null, solved: false, guesses: [] };
  await saveGame(db, subject, fresh);
  return fresh;
}

async function getOrInitGame(db, client, subject) {
  const existing = await loadGame(db, subject);
  if (existing && !existing.solved) return existing;
  return startFreshGame(db, client, subject);
}

export async function handleDoantu(ctx, { db, client }) {
  const subject = getSubject(ctx);
  if (subject == null) return ctx.reply("Cannot identify chat.");
  const arg = argAfterCommand(ctx.message?.text ?? "");
  let game;
  try {
    game = await getOrInitGame(db, client, subject);
  } catch (err) {
    logFail("random", err);
    return ctx.reply(UPSTREAM_FAIL);
  }
  if (!arg) return ctx.reply(renderBoard(game.guesses), { parse_mode: "HTML" });
  return submitGuess(ctx, { db, client }, subject, game, arg);
}

async function submitGuess(ctx, { db, client }, subject, game, arg) {
  const guess = normalize(arg);
  if (!isValidShape(guess)) {
    return ctx.reply("Please provide a Vietnamese word (letters + optional single spaces).");
  }
  // Fast-path dedup: exact text OR prior canonical. Avoids wasted API calls.
  if (game.guesses.some((g) => g.word === guess || g.canonical === guess)) {
    return ctx.reply(
      `🔁 <b>${escapeHtml(guess)}</b> was already guessed this round — try another word.`,
      { parse_mode: "HTML" },
    );
  }
  let res;
  try {
    res = await client.similarity(game.target, guess);
  } catch (err) {
    logFail("similarity", err);
    return ctx.reply(UPSTREAM_FAIL);
  }
  if (!res?.in_vocab_b || res.similarity == null) {
    return ctx.reply(`🤔 <code>${escapeHtml(guess)}</code> isn't in the vocabulary.`, {
      parse_mode: "HTML",
    });
  }

  const entry = {
    word: guess,
    canonical: String(res.canonical_b ?? guess).toLowerCase(),
    similarity: Number(res.similarity),
  };
  if (game.guesses.some((g) => g.canonical === entry.canonical)) {
    return ctx.reply(
      `🔁 <b>${escapeHtml(entry.canonical)}</b> was already guessed this round — try another word.`,
      { parse_mode: "HTML" },
    );
  }
  game.guesses.push(entry);
  if (game.startedAt === null) game.startedAt = Date.now();

  if (entry.canonical === game.target) {
    game.solved = true;
    const count = game.guesses.length;
    await recordResult(db, subject, { solved: true, guessCount: count });
    await clearGame(db, subject);
    const board = renderBoard(game.guesses, entry.canonical);
    return ctx.reply(`${board}\n✅ Solved in ${count} guess${count === 1 ? "" : "es"}!`, {
      parse_mode: "HTML",
    });
  }

  await saveGame(db, subject, game);
  const body = `${renderGuess(entry)}\n${renderBoard(game.guesses, entry.canonical)}`;
  return ctx.reply(body, { parse_mode: "HTML" });
}

export async function handleGiveup(ctx, { db }) {
  const subject = getSubject(ctx);
  if (subject == null) return ctx.reply("Cannot identify chat.");
  const game = await loadGame(db, subject);
  if (!game) {
    return ctx.reply("No active round. Send <code>/doantu</code> to start one.", {
      parse_mode: "HTML",
    });
  }
  await recordResult(db, subject, { solved: false, guessCount: game.guesses.length });
  await clearGame(db, subject);
  return ctx.reply(
    `🏳️ The target was <b>${escapeHtml(game.target)}</b>. Send <code>/doantu</code> for a new round.`,
    { parse_mode: "HTML" },
  );
}

export async function handleStats(ctx, { db }) {
  const subject = getSubject(ctx);
  if (subject == null) return ctx.reply("Cannot identify chat.");
  const s = await loadStats(db, subject);
  if (s.played === 0) return ctx.reply("No doantu games played yet.");
  const solveRate = Math.round((s.solved / s.played) * 100);
  const avgPerRound = s.played > 0 ? Math.round(s.totalGuesses / s.played) : "—";
  return ctx.reply(
    [
      "🇻🇳 <b>Đoán từ stats</b>",
      `Played: ${s.played}`,
      `Solved: ${s.solved} (${solveRate}%)`,
      `Total guesses: ${s.totalGuesses}`,
      `Fewest to solve: ${s.bestGuessCount ?? "—"}`,
      `Avg per round: ${avgPerRound}`,
    ].join("\n"),
    { parse_mode: "HTML" },
  );
}
