---
phase: 2
title: "Phase 2: Store, set names, emoji parsing"
status: done
priority: P1
effort: "3h"
dependencies: [1]
---

# Phase 2: Store, set names, emoji parsing

## Overview

Pure, Telegram-free foundation: the persisted pack record, the mapping between a
user-chosen slug and a Telegram set name, sender validation, and emoji-argument parsing.
Every function here is unit-testable without a bot.

One pack per user means the store layer is a single keyed record, not a collection scan.

## Requirements

- Functional: persist at most one pack record per user, keyed so the caller's own user ID
  is the whole key — making the lookup itself the ownership check.
- Functional: construct a Telegram set name `<slug>_by_<bot_username>` at creation, and match
  a sticker's `set_name` against the stored pack **without** re-deriving it from the live
  username.
- Functional: reject senders that are bots or anonymous chat surrogates.
- Functional: split an emoji argument run into individual emoji, accepting `😂 🔥` and `😂🔥`.
- Non-functional: no network calls and no Telegram API types in the store layer.

## Architecture

### Pack record

```go
// Pack is the single bot-created sticker set owned by a Telegram user.
type Pack struct {
    Slug      string `bson:"slug"`      // chosen at creation, fixes the permanent URL
    Name      string `bson:"name"`      // Telegram set name, "<slug>_by_<botname>"
    Title     string `bson:"title"`     // display title, mutable
    OwnerID   int64  `bson:"ownerId"`   // Telegram user the set belongs to
    Count     int    `bson:"count"`     // stickers in the set; keeps /mypack API-free
    Pending   bool   `bson:"pending"`   // write-ahead intent; see Phase 3
    CreatedAt int64  `bson:"createdAt"` // unix millis
}
```

- **Key: `strconv.FormatInt(ownerID, 10)`** — the user ID alone. One pack per user makes the
  slug unnecessary as a key component, which removes the prefix scan entirely.
- `getPack(ctx, ownerID) (Pack, bool, error)` — one `Get`; `storage.ErrNotFound` maps to
  `false, nil`. There is no `listPacks` and no `List` call anywhere in the module.

  This retires the worst red-team finding. The previous `listPacks` was a structural N+1
  (`mongo_doc_store.go:157-165` projects `_id` only, forcing a `Get` per key), and
  `/packlist` layered ten `GetStickerSet` calls on top of it under a 60s-per-call ceiling.

- `Count` keeps `/mypack` free of API calls. It is **advisory** — a user editing the pack
  through @Stickers desyncs it. Phase 3 refreshes it opportunistically.
- `Pending` implements write-ahead intent (plan C5). Phase 3 owns the state machine.
- `Pack`'s bson tags must not collide with `_id` / `version` / `updatedAt`; `storage.Typed`
  panics on collision (`internal/storage/doc_store.go:72`).

### Sender validation

```go
// senderID returns the personal Telegram user behind msg, or an error when the
// message has no usable personal identity.
func senderID(msg *models.Message) (int64, error)
```

Rejects, in order: nil `msg`/`From`, zero ID, `From.IsBot`, and non-nil `msg.SenderChat`.

The last two are not theoretical. Telegram substitutes a single global `GroupAnonymousBot`
user for **every** anonymous group-admin message and puts the real origin in `SenderChat`
(`models/message.go:86-87`). Without this check, all anonymous admins across all groups would
share one pack — and under one-pack-per-user that is worse than it was before, because the
first anonymous admin to run `/newpack` would block every other one and own the result.
`rg "IsBot" internal/` returns zero hits today; `coin`, `gold`, and `stock` check only
`From != nil && From.ID != 0`, which is safe for paper-trading state but not for durable
Telegram objects.

The refusal must explain the fix ("sticker packs need a personal account — turn off anonymous
posting for this message"), not just deny.

### Set names

- `slugRe = ^[a-z][a-z0-9_]{2,39}$` — 3 to 40 chars. Additionally reject `__` (Telegram
  forbids consecutive underscores) and a trailing `_`. The cap is for link readability and to
  stay inside the 64-char set-name budget.
- `makeSetName(slug, botUsername) (string, error)` → `slug + "_by_" + botUsername`, erroring
  above 64 chars and reporting the remaining slug budget so the reply can say "max N
  characters".
- **`ownsSet(pack Pack, setName string) bool`** — case-insensitive comparison of `setName`
  against the stored `pack.Name`. This is the ownership resolver used by Phase 4.

  It deliberately replaces a `parseSlug(setName, botUsername)` design that re-derived the slug
  from the *live* username and discarded the persisted `Pack.Name`. Renaming the bot in
  BotFather — supported, and it leaves existing set names untouched — would have made every
  user's own pack refuse as "not yours" while `/mypack` still displayed it. Comparing the
  stored name also removes a case-sensitivity trap, since Telegram returns `SetName` with
  whatever casing the set was created with (plan R8).

- `usernameResolver` caches `GetMe` and **must not cache failures**. The bot starts with
  `bot.WithSkipGetMe()` (`internal/telegram/client.go:26`), so nothing populates a username
  until the module asks. It is used **only** by `/newpack` to name a new set — never for
  ownership. It takes the handler's `b *bot.Bot`, **not** `deps.Bot`, which is documented
  nil-safe (`internal/modules/module.go:88`) and is nil under `BuildOptions{}`
  (`cmd/server/command_menu_test.go:55`).

### Emoji parsing

`parseEmoji(args []string) ([]string, error)` — join arguments, then split into clusters:

- keep ZWJ (`U+200D`) sequences together;
- absorb variation selectors (`U+FE0F`/`U+FE0E`), skin-tone modifiers (`U+1F3FB`–`U+1F3FF`),
  and combining marks into the preceding cluster;
- pair regional indicators (`U+1F1E6`–`U+1F1FF`);
- keep keycap sequences (`<base> U+FE0F U+20E3`) together.

Reject non-emoji text with a usage error. Cap at 20 (`emoji_list` is documented 1–20; the
server's own message is the literal `too many emoji specified`). `defaultEmoji = "⭐"`.

Because `/addsticker` now takes only `[emoji...]`, every one of its arguments is an emoji —
there is no first-token disambiguation to perform, and a stray word fails loudly here rather
than being mistaken for a pack name.

`models.Sticker.Emoji` is a **single string** (`models/sticker.go:23`), so emoji inherited
from a replied sticker yields at most one element.

## Related Code Files

- Create: `internal/modules/sticker/pack.go`, `setname.go`, `sender.go`, `emoji.go`
- Create: matching `_test.go` files for each
- Reference: `internal/storage/doc_store.go:38` (DocStore contract), `keys.go:24-41`
- Reference: `internal/modules/coin/handlers_test.go:36` (memory-store test pattern)
- Reference: `models/message.go:86-87` (`SenderChat`), `models/user.go:12` (`IsBot`),
  `models/sticker.go:23` (`Emoji` is one string)

## Implementation Steps

1. `pack.go` — record, `packKey`, `getPack`.
2. `sender.go` — `senderID` with the bot/anonymous refusals.
3. `setname.go` — slug validation, `makeSetName`, `ownsSet`, cached resolver interface.
4. `emoji.go` — cluster scanner and `defaultEmoji`.
5. Tests per the Todo list.

## Todo

- [x] Define `Pack` incl. `Count` and `Pending`; assert no reserved-bson collision
- [x] `packKey(ownerID)` and `getPack` returning a found flag
- [x] `senderID` rejecting nil/zero/`IsBot`/`SenderChat` with an explanatory message
- [x] `slugRe` validation incl. `__`, trailing `_`, 40-char cap
- [x] `makeSetName` with 64-char guard and budget-reporting error
- [x] `ownsSet` case-insensitive match against stored `Pack.Name`
- [x] `usernameResolver` caching success but never failure, taking the handler's `b`
- [x] `parseEmoji` cluster scanner with the 20-entry cap
- [x] Four test files per the success criteria

## Success Criteria

- [x] `getPack` for owner A never returns owner B's pack
- [x] `getPack` on an unknown owner returns `found == false` and a nil error
- [x] No `List` call exists anywhere in the module
- [x] Slug table rejects leading digit, `__`, trailing `_`, 2 chars, 41 chars
- [x] `makeSetName` errors when `len(slug)+len("_by_"+username) > 64`
- [x] `ownsSet` matches `MyPack_by_Bot` against a stored `mypack_by_bot`
- [x] `ownsSet` returns false for a set name belonging to another bot
- [x] A simulated bot username change does **not** break `ownsSet` for an existing pack
- [x] `senderID` rejects `IsBot: true` and a non-nil `SenderChat`, each with zero store access
- [x] `parseEmoji` handles joined input, ZWJ family, flag, keycap, skin tone; rejects plain text; errors above 20
- [x] `gofmt -l internal/modules/sticker` empty; `go test`/`go vet` clean

## Risk Assessment

**Emoji cluster scanning is hand-rolled.** Go has no stdlib grapheme segmentation and a
dependency for this is disproportionate. Signal: a user reports an emoji split or rejected.
Response: extend the table-driven test with the failing sequence. If failures accumulate
across many scripts, take a segmentation dependency.

**`Count` can drift.** Editing the pack through @Stickers changes the real count without the
bot seeing it. Accepted: the field is advisory and feeds one display column. Phase 3 refreshes
it whenever a command already holds a `GetStickerSet` response, so it self-heals without any
command paying for a lookup it did not otherwise need.

**Keying on the user ID alone bakes in the one-pack limit.** Reversing to multiple packs later
means a key migration, not just new commands — every existing record would need rewriting
under a compound key. That is the real cost of plan R10, and it is why the limit belongs in
the plan rather than living as a constant someone can bump.
