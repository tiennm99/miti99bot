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
That ordering is what keeps two users from racing for the same name: the first
claimant wins it and everyone else is refused before any set is created.

The claim is given up again whenever the bot has positive evidence that no pack
stands behind it — Telegram refusing the creation outright, `/delpack`, or a
later command finding the set already gone. A `/newpack` that never got as far
as claiming, or that is refused before Telegram is contacted, leaves nothing
behind.

The claim is deliberately **not** treated as proof of ownership over a set that
already exists. See "The bot never takes over an existing pack" below.

Telegram may keep a deleted short name reserved on its own side, so a freed name
is not guaranteed to be usable again by anyone, including its previous owner.

**Ownership refusals are identical by design.** "You don't have a pack" and
"that sticker isn't from your pack" produce the exact same reply. Distinct
wording would let anyone probe which sets exist under this bot.

**The bot never takes over an existing pack.** If a set already exists under the
name you ask for, `/newpack` refuses — always, for everyone, whatever the bot's
records say about it.

Earlier versions adopted such a set when local records suggested it came from
your own interrupted attempt. That was wrong in a way no amount of checking
fixes: every fact the bot could use to prove "this set is yours" lives in the
same storage a restart erases, while the packs at Telegram survive. Once the
proof is gone, a genuine interrupted attempt and a stranger naming your pack's
public link present the bot with identical evidence. The feature was the hole,
so the feature is gone.

The cost is real and worth stating plainly: if the bot crashes between creating
your set at Telegram and recording it, the set is stranded. It exists, it is
linkable, and no command in this bot can manage or delete it. `/mypack` marks
the unfinished attempt, and `/delpack` clears the leftover record so you can
create a pack under a different name — but it will not delete anything at
Telegram, because an unfinished record is not evidence that the bot made that
set for you. Anyone can produce such a record for any name.

Run this module against a real database. On the in-memory backend every restart
strands every pack.

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
