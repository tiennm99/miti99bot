---
phase: 2
title: "Phase 2: Store, set names, emoji parsing"
status: todo
priority: P1
effort: "4h"
dependencies: [1]
---

# Phase 2: Store, set names, emoji parsing

## Overview

Pure, Telegram-free foundation: the persisted pack record, the mapping between a
user-facing slug and a Telegram set name, sender validation, and emoji-argument parsing.
Every function here is unit-testable without a bot.

## Requirements

- Functional: persist one record per pack, keyed so a lookup by the caller's own user ID is
  itself the ownership check.
- Functional: build Telegram set names of the form `<slug>_by_<bot_username>`, and match a
  sticker's `set_name` against stored packs **without** re-deriving it from the live username.
- Functional: reject senders that are bots or anonymous chat surrogates.
- Functional: split an emoji argument run into individual emoji, accepting `😂 🔥` and `😂🔥`.
- Non-functional: no network calls and no Telegram API types in the store layer.

## Architecture

### Pack record

```go
// Pack is one bot-created sticker set owned by a Telegram user.
type Pack struct {
    Slug      string `bson:"slug"`      // user-facing id, e.g. "my_memes"
    Name      string `bson:"name"`      // Telegram set name, "my_memes_by_botname"
    Title     string `bson:"title"`     // display title
    OwnerID   int64  `bson:"ownerId"`   // Telegram user the set belongs to
    Count     int    `bson:"count"`     // stickers in the set; keeps /packlist API-free
    Pending   bool   `bson:"pending"`   // write-ahead intent; see Phase 3
    CreatedAt int64  `bson:"createdAt"` // unix millis
}
```

`Count` exists so `/packlist` makes zero API calls (plan C2 / R1). It is maintained by
`/addsticker` and `/delsticker` and is explicitly **advisory** — it can drift if a user edits
the pack through @Stickers. Phase 3 documents how it self-heals.

`Pending` implements write-ahead intent (plan C5). Phase 3 owns the state machine.

- Key: `packKey(ownerID int64, slug string)` → `strconv.FormatInt(ownerID,10) + ":" + slug`.
  `:` is legal — `internal/storage/keys.go:24-41` forbids only `/`, `.`, `..`, `__…__`, empty,
  and >1500 bytes.
- `listPacks(ctx, ownerID)` → `store.List(ctx, ownerID+":")` then `Get` each; sort by slug.
  The N+1 is structural: `mongo_doc_store.go:157-165` projects `_id` only.
- Ownership is structural: a command derives the key from the *caller's* ID, so a hit means
  the caller owns the pack.
- `maxPacksPerUser = 10` (plan O2), checked before create.
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
share one pack namespace and one quota, and `CreateNewStickerSet` with a bot `user_id` would
fail with an unmapped error. `rg "IsBot" internal/` returns zero hits today — `coin`, `gold`,
and `stock` all check only `From != nil && From.ID != 0`, which is safe for paper-trading
state but not for durable Telegram objects.

The refusal text should explain the fix ("sticker packs need a personal account — turn off
anonymous posting for this message"), not just deny.

### Set names

- `slugRe = ^[a-z][a-z0-9_]{2,39}$` — 3 to 40 chars. Additionally reject `__` (Telegram
  forbids consecutive underscores) and a trailing `_`. The 40-char cap is for link
  readability and title budget; it is **no longer** coupled to callback-payload size, since
  Phase 3 puts an opaque id in the callback rather than a slug.
- `buildSetName(slug, botUsername) (string, error)` → `slug + "_by_" + botUsername`, erroring
  above 64 chars and reporting the remaining slug budget so the reply can say "max N
  characters".
- **`matchPack(packs []Pack, setName string) (Pack, bool)`** — case-insensitive comparison of
  `setName` against each `pack.Name`. This is the ownership resolver used by Phase 4.

  It deliberately replaces the earlier `parseSlug(setName, botUsername)` design. That version
  re-derived the slug from the *live* username and discarded the persisted `Pack.Name`
  entirely — so renaming the bot in BotFather (a supported operation that leaves existing set
  names untouched) would make every user's own packs refuse as "not yours", while `/packlist`
  still listed them. It also let a case variant in `SetName` miss the key. Matching the stored
  name fixes both (plan R8).

- `usernameResolver` caches `GetMe` and **must not cache failures**. The bot starts with
  `bot.WithSkipGetMe()` (`internal/telegram/client.go:26`), so nothing populates a username
  until the module asks. It is used **only** to build names for new packs — never for
  ownership. It takes the handler's `b *bot.Bot`, **not** `deps.Bot`, which is documented
  nil-safe (`internal/modules/module.go:88`) and is nil under
  `modules.Build(nil, factories(), …, BuildOptions{})` (`cmd/server/command_menu_test.go:55`).

### Emoji parsing

`parseEmoji(args []string) ([]string, error)` — join arguments, then split into clusters:

- keep ZWJ (`U+200D`) sequences together;
- absorb variation selectors (`U+FE0F`/`U+FE0E`), skin-tone modifiers (`U+1F3FB`–`U+1F3FF`),
  and combining marks into the preceding cluster;
- pair regional indicators (`U+1F1E6`–`U+1F1FF`);
- keep keycap sequences (`<base> U+FE0F U+20E3`) together.

Reject non-emoji text with a usage error. Cap at 20 (`emoji_list` is documented 1–20; the
server's own message is the literal `too many emoji specified`). `defaultEmoji = "⭐"` for
sources carrying none.

Note `models.Sticker.Emoji` is a **single string** (`models/sticker.go:23`), so emoji
inherited from a replied sticker yields at most one element.

## Related Code Files

- Create: `internal/modules/sticker/pack.go`, `setname.go`, `sender.go`, `emoji.go`
- Create: `internal/modules/sticker/pack_test.go`, `setname_test.go`, `sender_test.go`,
  `emoji_test.go`
- Reference: `internal/storage/doc_store.go:38` (DocStore contract), `keys.go:24-41`
- Reference: `internal/modules/coin/handlers_test.go:36` (memory-store test pattern)
- Reference: `go-telegram/bot@v1.20.0` `models/message.go:86-87` (`SenderChat`),
  `models/user.go:12` (`IsBot`), `models/sticker.go:23` (`Emoji` is one string)

## Implementation Steps

1. `pack.go` — record, key helpers, `listPacks`, `maxPacksPerUser`.
2. `sender.go` — `senderID` with the bot/anonymous refusals.
3. `setname.go` — slug validation, `buildSetName`, `matchPack`, cached resolver interface.
4. `emoji.go` — cluster scanner and `defaultEmoji`.
5. Tests per the Todo list.

## Todo

- [ ] Define `Pack` incl. `Count` and `Pending`; assert no reserved-bson collision
- [ ] `packKey` and `listPacks` with owner-prefix scan
- [ ] `maxPacksPerUser` quota helper
- [ ] `senderID` rejecting nil/zero/`IsBot`/`SenderChat` with an explanatory message
- [ ] `slugRe` validation incl. `__`, trailing `_`, 40-char cap
- [ ] `buildSetName` with 64-char guard and budget-reporting error
- [ ] `matchPack` case-insensitive match against stored `Pack.Name`
- [ ] `usernameResolver` caching success but never failure, taking the handler's `b`
- [ ] `parseEmoji` cluster scanner with the 20-entry cap
- [ ] Four test files per the success criteria

## Success Criteria

- [ ] `listPacks` for owner A never returns owner B's packs
- [ ] Quota boundary tested at 9, 10, 11 packs
- [ ] Slug table rejects leading digit, `__`, trailing `_`, 2 chars, 41 chars
- [ ] `buildSetName` errors when `len(slug)+len("_by_"+username) > 64`
- [ ] `matchPack` matches `MyPack_by_Bot` against a stored `mypack_by_bot`
- [ ] `matchPack` returns false for a set name belonging to another bot
- [ ] A simulated bot username change does **not** break `matchPack` for existing packs
- [ ] `senderID` rejects `IsBot: true` and a non-nil `SenderChat`, each with zero store access
- [ ] `parseEmoji` handles joined input, ZWJ family, flag, keycap, skin tone; rejects plain text; errors above 20
- [ ] `gofmt -l internal/modules/sticker` empty; `go test`/`go vet` clean

## Risk Assessment

**Emoji cluster scanning is hand-rolled.** Go has no stdlib grapheme segmentation and a
dependency for this is disproportionate. Signal: a user reports an emoji split or rejected.
Response: extend the table-driven test with the failing sequence. If failures accumulate
across many scripts, take a segmentation dependency.

**`Count` can drift.** A user editing their pack through @Stickers changes the real count
without the bot seeing it. Accepted: the field is advisory and only feeds a display column.
Phase 3 refreshes it opportunistically whenever a command already holds a `GetStickerSet`
response, so it self-heals without any command paying for a lookup it did not need.

**The bot username is still a single point of failure for `/newpack`.** `matchPack` protects
existing packs from a rename (R8), but creation still builds `<slug>_by_<current username>`.
After a rename, old packs keep working and new ones use the new suffix — correct behaviour,
but worth stating so nobody "fixes" it later by rewriting stored names.
