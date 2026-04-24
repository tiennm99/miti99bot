/**
 * @file Command handlers for the twentyq module.
 *
 * Subject resolution mirrors loldle/doantu:
 *   private chat           → user id (per-user game)
 *   group/supergroup chat  → chat id (shared game)
 *
 * Commands:
 *   /twentyq              → show board (or auto-start a fresh round)
 *   /twentyq <text>       → ask a yes/no question OR submit a guess
 *   /twentyq_giveup       → reveal target + end round
 *   /twentyq_stats        → show per-subject stats
 */

import { UpstreamError, judge } from "./ai-client.js";
import { formatBoard, formatGiveup, formatIntro, formatStats, formatTurnReply } from "./render.js";
import { getRandomSeed } from "./seeds.js";
import { clearGame, loadGame, loadStats, recordResult, saveGame } from "./state.js";
import { validateQuestion } from "./validate-input.js";

const UPSTREAM_FAIL = "⚠️ AI service hiccup — try again in a few seconds.";
const NO_ROUND = "No active round. Send <code>/twentyq</code> to start one.";

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

function startFreshGame() {
  const seed = getRandomSeed();
  return {
    category: seed.category,
    target: seed.target,
    initialHint: seed.initialHint,
    startedAt: Date.now(),
    solved: false,
    turns: [],
  };
}

function logFail(stage, err) {
  console.log(
    JSON.stringify({
      msg: "twentyq_upstream_fail",
      stage,
      err:
        err instanceof UpstreamError
          ? { name: err.name, status: err.status, cause: String(err.cause) }
          : String(err),
    }),
  );
}

export async function handleTwentyq(ctx, { db, env }) {
  const subject = getSubject(ctx);
  if (subject == null) return ctx.reply("Cannot identify chat.");
  const arg = argAfterCommand(ctx.message?.text ?? "");

  let game = await loadGame(db, subject);
  // Solved games linger until next /twentyq → start a fresh round transparently.
  if (game?.solved) {
    await clearGame(db, subject);
    game = null;
  }

  if (!game) {
    game = startFreshGame();
    await saveGame(db, subject, game);
    if (!arg) return ctx.reply(formatIntro(game), { parse_mode: "HTML" });
    // Fresh round + immediate question — show intro then process turn.
    await ctx.reply(formatIntro(game), { parse_mode: "HTML" });
    return submitTurn(ctx, { db, env }, subject, game, arg);
  }

  if (!arg) return ctx.reply(formatBoard(game), { parse_mode: "HTML" });
  return submitTurn(ctx, { db, env }, subject, game, arg);
}

async function submitTurn(ctx, { db, env }, subject, game, raw) {
  const v = validateQuestion(raw);
  if (!v.ok) return ctx.reply(v.reason, { parse_mode: "HTML" });

  const lower = v.normalized.toLowerCase();
  if (game.turns.some((t) => t.text.toLowerCase() === lower)) {
    return ctx.reply("🔁 You already asked that exact question — try a new angle.");
  }

  let result;
  try {
    result = await judge(env, game, v.normalized);
  } catch (err) {
    logFail("judge", err);
    return ctx.reply(UPSTREAM_FAIL);
  }

  const turn = {
    text: v.normalized,
    isGuess: result.is_guess,
    answer: result.answer,
    hint: result.hint,
    ts: Date.now(),
  };
  game.turns.push(turn);

  const won = turn.isGuess && turn.answer === "yes";
  if (won) {
    game.solved = true;
    const turnCount = game.turns.length;
    await recordResult(db, subject, { solved: true, turnCount });
    await clearGame(db, subject);
    return ctx.reply(formatTurnReply({ turn, solved: true, target: game.target, turnCount }), {
      parse_mode: "HTML",
    });
  }

  await saveGame(db, subject, game);
  return ctx.reply(
    formatTurnReply({ turn, solved: false, target: game.target, turnCount: game.turns.length }),
    { parse_mode: "HTML" },
  );
}

export async function handleGiveup(ctx, { db }) {
  const subject = getSubject(ctx);
  if (subject == null) return ctx.reply("Cannot identify chat.");
  const game = await loadGame(db, subject);
  if (!game) return ctx.reply(NO_ROUND, { parse_mode: "HTML" });
  await recordResult(db, subject, { solved: false, turnCount: game.turns.length });
  await clearGame(db, subject);
  return ctx.reply(formatGiveup(game), { parse_mode: "HTML" });
}

export async function handleStats(ctx, { db }) {
  const subject = getSubject(ctx);
  if (subject == null) return ctx.reply("Cannot identify chat.");
  const stats = await loadStats(db, subject);
  return ctx.reply(formatStats(stats), { parse_mode: "HTML" });
}
