# Blacklist

The `blacklist` module lets a chat keep a list of forbidden text and a list of
exceptions, then ask whether a given text is blocked.

| Command | Parameters | What it does |
|---|---|---|
| `/blacklist_add` | `[text...]` | Adds text to the blacklist, or the message you replied to |
| `/blacklist_del` | `<text...>` | Removes text from the blacklist |
| `/whitelist_add` | `[text...]` | Adds an exception, or the message you replied to |
| `/whitelist_del` | `<text...>` | Removes an exception |
| `/blacklist_rules` | — | Lists both lists in one message |
| `/blacklist_check` | `<text...>` | Judges a text against both lists |

All are public and single-shot.

## The bot does not police the chat

This is the first thing to know, because the module's name promises something it
deliberately does not do. Nothing happens automatically. The bot never reads
ordinary messages, never deletes anything, and never warns or restricts anyone.
The lists sit inert until `/blacklist_check` asks about a specific text.

Automatic moderation would need three things this bot does not have: a
message-level hook in the dispatcher, privacy mode disabled in BotFather so
Telegram delivers ordinary group messages at all, and admin rights with
permission to delete in every group. All three are out of scope by choice.

## Lists belong to a topic, not to the bot

Each thread keeps its own two lists. In a forum supergroup, entries added in one
topic are invisible in the next. A non-forum group has one set of lists; a
private chat with the bot has its own, shared with nobody.

This is the most common surprise: running `/blacklist_check` in the wrong topic
gives a different answer than the same command one topic over, and it is not a
bug. Every reply says "in this topic" for that reason.

Within a thread, anyone can add and anyone can remove — including entries
someone else added. The lists belong to the conversation, so the permission to
edit them does too. A per-owner rule would strand entries whose author has left
the group.

## How the whitelist works

The whitelist is not a second independent list. It is an exception layer over
the blacklist, and it rescues a match only when it **covers** that match in the
text being checked.

With `ass` blacklisted and `assassin` whitelisted:

| Checked text | Verdict | Why |
|---|---|---|
| `assassin` | Allowed | The whitelist entry spans the whole match |
| `dumbass` | Blacklisted | Nothing whitelisted covers this `ass` |
| `I met an assassin, dumbass` | **Blacklisted** | The first match is rescued; the second is not |

The third row is the one worth remembering. A whitelist entry does not make a
whole message safe — it only rescues the occurrences it actually contains.

## Matching

Matching is by substring, after the text is normalized:

- **Case does not matter.** `Cat`, `CAT` and `cat` are one rule.
- **Spacing does not matter.** `cat  dog` and `cat dog` are one rule, and a
  newline is just a space.
- **Diacritics do matter.** `ma`, `má` and `mà` are three separate entries. To
  catch all three, add all three.
- **The same word matches across devices.** Vietnamese typed on an iPhone and on
  an Android phone can differ in how the accents are encoded; normalization
  resolves that, so the two forms match each other.
- **Full-width and other presentation variants fold** onto their plain forms.

`/blacklist_check` accepts text of any length. Entries themselves are capped at
200 bytes — roughly 200 plain letters, or about 65 Vietnamese characters.

## Listing

`/blacklist_rules` prints both lists in one message, each with its entry count,
showing entries as they were typed rather than in the normalized form. Each
entry is tappable to copy, ready to paste into a `_del` command.

Telegram caps a message at 4096 characters. A list longer than that is trimmed
with a count of what was left out; both headings always appear, so a long
blacklist never hides the whitelist entirely.
