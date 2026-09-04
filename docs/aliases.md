# Aliases

The `alias` module lets anyone give a short name to a message and send it back
later by that name.

| Command | Parameters | Reply to | What it does |
|---|---|---|---|
| `/alias` | `<name>` | any supported message | Saves it under that name |
| `/insert` | `<name>` | — | Sends back whatever is saved under it |
| `/aliases` | — | — | Lists every saved name |
| `/unalias` | `<name>` | — | Deletes a saved name |
| `/<name>` | — | — | Same as `/insert <name>` |
| `@botname <prefix>` | — | — | Inline picker, in any chat |

All are public and single-shot.

## The namespace is global

A name assigned in any chat works in **every** chat, for **everyone** — the same
way the sticker pack `/addsticker` writes to is shared. The store is a plain map
from name to content, with no chat or user in the key.

The consequence is worth stating plainly: anyone can reassign anyone's name.
`/alias` overwrites rather than refusing, and its reply says what it replaced —

> Replaced /insert cheer — it was a sticker, now it is a GIF.

Overwriting is deliberate: refusing would make a mistyped alias awkward to
correct. `/unalias <name>` deletes one, and is open to anyone for the same
reason overwriting is — the namespace is shared, so the permission model is too.
A per-owner restriction would leave an alias whose assigner has left the chat
permanently unremovable.

## Invoking an alias

Three ways, same binding:

1. **`/<name>`** — a saved name works as its own command. `/cheer` is `/insert cheer`.
2. **`/insert <name>`** — always works, including when inline mode is off.
3. **`@botname <prefix>`** — an inline picker with previews, usable in any chat,
   even ones the bot is not a member of.

**Code always beats an alias.** The dispatcher registers every real command
before the alias fallback, and the bot library returns the *first* handler whose
matcher accepts an update — so a command defined in code can never be shadowed
by a name resolved at runtime. `/alias` refuses a name that is already a
command for the same reason, since such an alias would only ever be reachable
through `/insert`.

If a future build adds a command whose name an alias already uses, the command
silently wins and the alias stays reachable via `/insert`. That is the intended
precedence, not a bug to fix.

**An unknown `/command` is answered with silence.** The fallback sees every
unrecognised command in every chat the bot is in, so replying would turn a typo
like `/pign` into noise, and would confirm to anyone probing which names exist.

## Inline mode

`@botname` with no query lists everything; with a query it filters by name
prefix, case-insensitively, sorted, capped at Telegram's 50 results per answer.

Each result is a **cached** inline type — it carries the `file_id` Telegram
already holds, so nothing is uploaded and the picker shows real previews. This
is the payoff for storing a `file_id` rather than bytes.

**Video-note aliases do not appear inline.** Telegram defines no
`InlineQueryResultCachedVideoNote`, and substituting a plain video would change
what was saved. They stay reachable through `/insert` and `/<name>`.

**Two things gate inline mode, and both fail silently.**

1. `inline_query` must be in `pollingAllowedUpdates`
   (`internal/telegram/client.go`). Telegram filters getUpdates server-side, so
   a missing kind means the handler is never called — no log line, no error.
2. Inline mode must be enabled for the bot in BotFather (`/setinline`). Until
   it is, Telegram does not offer the bot for inline use at all, so typing
   `@botname` shows nothing. This is a one-time operational step that cannot be
   done from code.

## Names

One word, username-shaped: starts with a letter, then letters, digits and
underscores, up to 32 characters. A leading `@` is stripped rather than
rejected, since these names imitate usernames and typing the sigil is a natural
slip.

Lookups fold case — `/insert LOUD` and `/insert loud` find the same entry — and
the spelling the assigner used is what gets echoed back.

Telegram's own username minimum is 5 characters; this allows 1 on purpose. The
point of an alias is to be shorter than what it replaces, and `gg` is a good
name for a sticker.

## What can be saved

| Replied message | Sent back with |
|---|---|
| Sticker | `sendSticker` |
| Photo | `sendPhoto` |
| GIF / animation | `sendAnimation` |
| Video | `sendVideo` |
| Video note | `sendVideoNote` |
| Audio | `sendAudio` |
| Voice message | `sendVoice` |
| File / document | `sendDocument` |
| Plain text | `sendMessage` |

Anything else — a location, a poll, a contact — is refused with the list above.

**Nothing is downloaded.** Every media kind is kept as the `file_id` Telegram
already issued, and `/insert` hands that same id straight back to a send call.
The module stores bytes for nothing but the name and a caption. A `file_id`
refers to a file on Telegram's servers, so an alias survives restarts and
redeploys.

The kind is stored alongside the id because a bare `file_id` does not say which
send method will accept it.

**Order matters when Telegram fills more than one field.** A GIF arrives as an
`Animation` *and* a `Document`, and the more specific kind is claimed first —
otherwise `/insert` would hand back a plain file instead of a looping GIF.

**Captions come back too**, for the kinds that can carry one. Stickers and video
notes cannot, and Telegram's send methods for them have no caption field at all.

## Listing

`/aliases` prints the count and every name, sorted, in one message.

**Names only, not what each holds.** The store answers "which keys exist" in a
single call, while naming each kind would cost one read per alias — a round trip
each against MongoDB, on a dispatcher that serves one update at a time. To find
out what a name holds, `/insert` it.

One line per alias, showing what the name holds, with the invocation in a
`<code>` span so tapping it copies a command ready to send:

```
3 aliases:
/cheer — sticker
/clip — video
/greeting — text
```

Names list in their folded (lowercase) form, which is exactly what `/insert`
takes.

This costs one store read per *listed* alias — `DocStore` has no bulk get and
the kind lives in the document. The reads stop as soon as the message is full,
so the cost is bounded by what fits in one reply rather than by how many
aliases exist. The list is trimmed to Telegram's 4096-character limit and ends
with `…and N more.`; the count at the top is always the true total.

## Behaviour worth knowing

**Formatting survives.** Bold, italic, code, links and mentions are stored as
entities alongside the text, and captions keep theirs too. This works because
the text is re-sent byte-identical: entity offsets are relative to that text,
so they stay valid. They are sent back as entities rather than re-rendered as
markup, which avoids escaping and re-parsing content the user never wrote as
markup.

**Another bot's message cannot be saved.** Telegram's own rule: *"Bots will not
be able to see messages from other bots regardless of mode."* The reply arrives
with its content stripped, so there is nothing to store and no setting that
would change it. `/alias` says so specifically rather than implying the format
was unsupported. Forwarding the message to yourself first and aliasing your own
copy works.

**A `file_id` can stop working** — the original file was deleted, or Telegram
rejects it. `/insert` answers with something actionable rather than a generic
failure:

> "gone" can no longer be sent. Save it again with /alias gone.

**Replies keep their forum topic.** Every send forwards `MessageThreadID`, for
the reason `chathelper.Reply` documents: without it Telegram routes the message
to a supergroup's General topic instead of the topic the command was typed in.

Both handlers run under a 10-second deadline. The bot processes updates one at a
time, so that bound is what keeps a slow store from stalling other users.
