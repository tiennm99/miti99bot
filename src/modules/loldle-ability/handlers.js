/**
 * @file loldle-ability command handlers.
 *
 * Binary right/wrong — 5 guesses. Subject resolution mirrors other loldle
 * modes. Round state persists `{target, slot, guesses, startedAt}` so the
 * SAME ability icon shows across all turns until the round ends.
 */

import { escapeHtml } from "../../util/escape-html.js";
import abilitiesData from "./abilities.json" with { type: "json" };
import { findChampion } from "./lookup.js";
import { MAX_GUESSES, clearGame, loadGame, loadStats, recordResult, saveGame } from "./state.js";

const POOL = abilitiesData.filter((c) => c.abilities && c.abilities.length > 0);

const NEW_ROUND_HINT =
  "🆕 Send <code>/loldle_ability</code> or <code>/loldle_ability &lt;champion&gt;</code> to start a new round.";

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

function pickAbility(champ) {
  return champ.abilities[Math.floor(Math.random() * champ.abilities.length)];
}

function getAbilityBySlot(champ, slot) {
  return champ.abilities.find((a) => a.slot === slot);
}

async function startFreshGame(db, subject) {
  const target = pickRandom();
  const ability = pickAbility(target);
  const fresh = {
    target: target.championName,
    slot: ability.slot,
    guesses: [],
    startedAt: null,
  };
  await saveGame(db, subject, fresh);
  return fresh;
}

async function getOrInitGame(db, subject) {
  const existing = await loadGame(db, subject);
  if (existing && existing.guesses.length < MAX_GUESSES) return existing;
  return startFreshGame(db, subject);
}

function caption(guesses) {
  return `🔮 Guess the champion from this ability. ${guesses.length}/${MAX_GUESSES} guesses so far.`;
}

export async function handleAbility(ctx, db) {
  const subject = getSubject(ctx);
  if (subject == null) return ctx.reply("Cannot identify chat.");
  const arg = argAfterCommand(ctx.message?.text ?? "");

  const game = await getOrInitGame(db, subject);
  const target = findByName(game.target);
  const ability = target && getAbilityBySlot(target, game.slot);
  if (!target || !ability) {
    await clearGame(db, subject);
    return ctx.reply(`Ability data was updated since this round started. ${NEW_ROUND_HINT}`, {
      parse_mode: "HTML",
    });
  }

  if (!arg) {
    return ctx.replyWithPhoto(ability.icon, { caption: caption(game.guesses) });
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
  const abilityLabel = `<i>${escapeHtml(ability.name)}</i> (${ability.slot})`;

  if (won) {
    const s = await recordResult(db, subject, true);
    await clearGame(db, subject);
    return ctx.reply(
      `🎉 Got it! That was <b>${answer}</b> — ${abilityLabel}. Solved in ${game.guesses.length}/${MAX_GUESSES}\n🔥 Streak: ${s.streak}\n${NEW_ROUND_HINT}`,
      { parse_mode: "HTML" },
    );
  }

  if (game.guesses.length >= MAX_GUESSES) {
    await recordResult(db, subject, false);
    await clearGame(db, subject);
    return ctx.reply(
      `❌ Out of guesses. Answer was <b>${answer}</b> — ${abilityLabel}.\n${NEW_ROUND_HINT}`,
      { parse_mode: "HTML" },
    );
  }

  await saveGame(db, subject, game);
  return ctx.reply(
    `❌ Not <b>${escapeHtml(guess.championName)}</b>. Guess ${game.guesses.length}/${MAX_GUESSES}.`,
    { parse_mode: "HTML" },
  );
}

export async function handleGiveup(ctx, db) {
  const subject = getSubject(ctx);
  if (subject == null) return ctx.reply("Cannot identify chat.");
  const existing = await loadGame(db, subject);
  if (!existing) {
    return ctx.reply(`No active round. ${NEW_ROUND_HINT}`, { parse_mode: "HTML" });
  }
  await recordResult(db, subject, false);
  const target = findByName(existing.target);
  const ability = target && getAbilityBySlot(target, existing.slot);
  await clearGame(db, subject);
  const abilityLabel = ability ? ` — <i>${escapeHtml(ability.name)}</i> (${ability.slot})` : "";
  return ctx.reply(
    `🏳️ Answer was <b>${escapeHtml(existing.target)}</b>${abilityLabel}.\n${NEW_ROUND_HINT}`,
    { parse_mode: "HTML" },
  );
}

export async function handleStats(ctx, db) {
  const subject = getSubject(ctx);
  if (subject == null) return ctx.reply("Cannot identify chat.");
  const s = await loadStats(db, subject);
  const winRate = s.played ? Math.round((s.wins / s.played) * 100) : 0;
  const scope = ctx.chat?.type === "private" ? "your" : "group";
  return ctx.reply(
    `📊 Loldle Ability ${scope} stats\n` +
      `Played: ${s.played}\n` +
      `Wins: ${s.wins} (${winRate}%)\n` +
      `Current streak: ${s.streak}\n` +
      `Best streak: ${s.bestStreak}`,
  );
}
