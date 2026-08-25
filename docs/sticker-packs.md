# Sticker packs

The `sticker` module lets any user create and manage **one** personal Telegram
sticker pack through the bot. The pack is created on behalf of the calling user,
so it appears under their account, and it stays bot-manageable because the bot
created it.

Every command is single-shot: one message carrying its arguments, optionally
replying to a sticker or photo. There is no conversation state and no `/cancel`.

## Commands

| Command | Parameters | Reply to | What it does |
|---|---|---|---|
| `/newpack` | `<pack> <title...>` | sticker or photo | Creates your pack and returns its share link |
| `/mypack` | — | — | Shows your pack: name, title, sticker count, link |
| `/addsticker` | `[emoji...]` | sticker or photo | Adds it to your pack |
| `/delsticker` | — | a sticker in your pack | Removes it |
| `/editsticker` | `<emoji...>` | a sticker in your pack | Replaces that sticker's emoji |
| `/ordersticker` | `<position>` | a sticker in your pack | Moves it; positions start at 0 |
| `/setpackicon` | — | a sticker in your pack | Uses it as the pack icon |
| `/renamepack` | `<title...>` | — | Changes the displayed title |
| `/delpack` | — | — | Deletes the pack, after an inline confirmation |

Only `/newpack` names a pack. Every other command resolves your single pack from
storage, or from the replied sticker's set.

## The pack name is permanent

`/newpack mypack My Pack` creates `t.me/addstickers/mypack_by_<botusername>`.

**Telegram has no method to rename a sticker set's short name.** That link is
fixed for the life of the pack. `/renamepack` changes only the displayed title.

The only way to a different link is `/delpack` followed by `/newpack` under a new
name — and the stickers do not come along. `/delpack`'s confirmation states the
title, the number of stickers it destroys, the exact link being surrendered, and
that both are permanent, because that prompt is the last point at which someone
wanting "a rename" learns what it actually costs.

Pack-name rules: 3–40 characters, lowercase letters, digits and underscores,
starting with a letter, no two underscores in a row, no trailing underscore.

## Images

Static stickers only. Animated, video, mask, and custom-emoji stickers are
rejected.

- A replied **sticker** is added directly.
- A replied **photo** or image **document** (`image/png`, `image/jpeg`,
  `image/webp`) is downloaded, resized so its long edge is exactly 512px with
  the aspect ratio preserved, and uploaded as a PNG.
- Pack icons are resized to exactly 100×100, padded transparently.
- Sources above 2 MB, or with either side above 4096px, are rejected.

Telegram allows 120 stickers per pack and 1–20 emoji per sticker. There is no
documented file-size limit for static stickers; the module applies its own
client-side ceiling and never presents it as a Telegram rule.

## Who can use it

Public — every user manages their own pack.

**Anonymous group admins are refused.** Telegram substitutes a single global
`GroupAnonymousBot` user for every anonymous admin message, so without this
refusal all anonymous admins across all groups would share one pack. Turn off
anonymous posting for the message and try again.

## Deliberate omissions

- **Usage statistics** (`/stats`, `/top`, `/packstats`, …) — the Bot API does
  not expose sticker usage counts, and `/stats` belongs to the `stats` module.
- **Animated, video, emoji, and mask packs** — out of scope; this module is
  static-only.
- **More than one pack per user** — a deliberate simplification. It is what lets
  every command but `/newpack` drop its pack argument.
- **`/cancel`** — meaningless without conversation state.
- **A `/repack` migration command** — copying a full pack is up to ~121
  sequential API calls, which would stall the bot for every user.

## Behaviour worth knowing

**Sticker counts are advisory.** `/mypack` reads the count from storage and
makes no API calls at all. Editing your pack through @Stickers changes the real
count without the bot seeing it; the number re-syncs whenever a command already
has a fresh view of the set.

**Pack names are claimed first-come and held permanently.** The bot records who
claimed each name before it creates anything on Telegram, and only that user can
ever manage a pack under it. This is what stops someone from reading a pack's
name off its public link and taking it over.

`/newpack` therefore reports when a name is taken, which reveals that some user
of this bot holds it. That is accepted: `t.me/addstickers/<name>_by_<bot>` is
publicly probeable without the bot, so the command discloses nothing new. It
never says *who* holds a name. Refusals about *managing* a pack are deliberately
uniform for the opposite reason — see below.

A name is claimed *before* the bot calls Telegram, not after the pack exists.
That ordering is the point: the claim is what proves, on a later re-run, that an
existing set under that name is yours to finish rather than someone else's to
take.

The claim is given up again whenever the bot has positive evidence that no pack
stands behind it — Telegram refusing the creation outright, `/delpack`, or a
later command finding the set already gone. A `/newpack` that never got as far
as claiming, or that is refused before Telegram is contacted, leaves nothing
behind.

Telegram may keep a deleted short name reserved on its own side, so a freed name
is not guaranteed to be usable again by anyone, including its previous owner.

**Ownership refusals are identical by design.** "You don't have a pack" and
"that sticker isn't from your pack" produce the exact same reply. Distinct
wording would let anyone probe which sets exist under this bot.

**An interrupted `/newpack` can be finished.** The bot records your intent
before calling Telegram, so if a deploy or crash lands mid-creation, re-running
the same `/newpack` command completes it instead of reporting the name taken.
`/mypack` marks an unfinished attempt so it is visible rather than mysterious.

This depends on the bot still holding your claim to the name. If its storage has
been wiped since — which is what a restart does when no database is configured —
the claim is gone while the pack at Telegram is not, and `/newpack` reports the
name as taken rather than adopting a set it can no longer prove is yours.
Recovering a pack in that state needs operator help. Run this module against a
real database, not the in-memory backend.

**Deleting the last sticker may delete the pack.** Telegram's behaviour here is
undocumented, so the bot does not guess: it will not remove your pack record on
anything less than a positive "this set no longer exists" from Telegram. If a
command reports the pack is gone, `/delpack` clears the stale record and
`/newpack` works again.

## Operations

The module is enabled by listing `sticker` in `MODULES` (an empty `MODULES`
loads every module). It stores one record per user, keyed by Telegram user ID,
plus at most one pending `/delpack` confirmation per user — running `/delpack`
again supersedes the previous prompt, and a confirmation stops working after 10
minutes.

Every handler runs under a 10-second deadline. The bot processes updates one at
a time, so this bound is what keeps an image conversion from stalling other
users.
