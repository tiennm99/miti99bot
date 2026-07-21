---
type: research-report
conducted_at: 2026-07-21T14:18:00+07:00
scope: Telegram command discovery syntax
status: complete
---

# Research Report: Minimal Command Parameter Schema

## Summary

Keep the project's current lightweight schema. It already matches familiar CLI
synopsis conventions closely enough:

```text
<name>       required value
[name]       optional value
[name...]    optional remaining text
a | b        alternatives inside one optional group
```

Do not add types such as `<quantity:number>`. Common CLI usage lines describe
structure, not a type system. Use a descriptive name (`<quantity>`,
`<vnd_amount>`) and put validation details in the summary or usage error.

Only one current notation is needlessly heavy. After project-owner review, the
approved compact replacement is:

```text
<options(comma-separated)>  ->  <option,...>
```

This keeps the literal comma visible without putting prose inside the
placeholder. Keep `<ratio(owned:new)>` for the same reason: the separator is
essential input syntax and the concise shape prevents a likely user error.

## Contents

- [Scope and method](#scope-and-method)
- [Evidence](#evidence)
- [Recommended schema](#recommended-schema)
- [Current command audit](#current-command-audit)
- [Rejected alternatives](#rejected-alternatives)
- [Next steps](#next-steps)
- [References](#references)
- [Unresolved questions](#unresolved-questions)

## Scope and Method

Evaluated only the syntax displayed after `/command` in Telegram's native menu
and `/help`. Criteria, in order:

1. Understandable without a legend
2. Minimal visual noise on a phone
3. Accurate to the parser
4. Consistent across commands
5. Familiar to terminal users

Sources span POSIX.1-2024, current GNU guidance, Python 3.14 documentation,
Microsoft's command-line syntax key, docopt's usage grammar, and Telegram's
current Bot API. The configured Gemini path was disabled. Web search returned
an authorization error, so primary sources were retrieved directly over HTTPS.

## Evidence

### Strong consensus

- Square brackets mean optional material. POSIX, Microsoft, argparse, and
  docopt all use this convention.
- Angle brackets are a recognized way to mark a value the user must replace.
  POSIX explicitly permits `<parameter name>`; Microsoft defines angle-bracket
  text as a placeholder.
- A vertical bar separates mutually exclusive alternatives. Microsoft and
  docopt explicitly document this.
- An ellipsis indicates repetition. POSIX uses `[operand ...]`; docopt uses
  `FILE ...` for one or more and `[FILE ...]` for zero or more.
- Metavariable names should explain the role of the value. Python argparse uses
  `metavar` for this purpose and generates the structural punctuation itself.

### Important nuance

Traditional terminal help often uses uppercase metavariables such as `FILE` or
`FOO`. Lowercase angle-bracket names such as `<ticker>` remain unambiguous and
are calmer in Telegram's compact UI. Changing case would add churn without
improving comprehension.

POSIX's spaced ellipsis (`operand ...`) means repeated operands. This bot's
`[target...]` means one optional free-text remainder. That is a small local
extension, but it communicates the behavior better than `[target]`, which can
look limited to one word. Document it once and keep it consistent.

## Recommended Schema

Use only constructs the project currently needs:

| Meaning | Format | Example |
|---|---|---|
| Required value | `<name>` | `<ticker>` |
| Required comma-separated values | `<name,...>` | `<option,...>` |
| Optional value | `[name]` | `[date]` |
| Optional remaining text | `[name...]` | `[target...]` |
| Optional alternatives | `[literal | literal <name>]` | `[users | user <username>]` |

Naming rules:

- Lowercase `snake_case`.
- Prefer one plain noun: `<ticker>`, `<quantity>`, `<champion>`.
- Include a unit when it prevents ambiguity: `<vnd_amount>`, `<usd_to_spend>`.
- Do not encode primitive types: avoid `<quantity:number>` and `<date:string>`.
- Do not put format prose inside a placeholder.
- Include a compact value shape only when users must type its punctuation
  exactly: `<ratio(owned:new)>`, `<option,...>`.
- Keep literal subcommands bare: `users`, `user`, `cmd`.
- Put spaces around `|`; readability is worth the two characters.

This is presentation metadata, not a machine-validated schema language. Avoid
adding an AST, parser, enums, or type annotations until the bot needs generated
validation or completion.

## Current Command Audit

| Commands | Current | Recommendation | Reason |
|---|---|---|---|
| `/random`, `/wheelofnames` | `<options(comma-separated)>` | `<option,...>` | Shows the literal delimiter without prose. |
| `/stats` | `[users | user <username> | cmd <command_name>]` | Keep | Accurate compact grammar; `command_name` avoids confusing the value with a literal command. |
| `/trongtruonghop`, `/tth` | `[target...]` | Keep | Clearly signals optional multi-word remainder. |
| `/lol` | `[date]` | Keep | Minimal; accepted date shapes belong in the summary/error. |
| `/loldle`, `/wordle` | `[champion]`, `[word]` | Keep | Optional value accurately reflects start-without-argument behavior. |
| Coin commands | `<coin>`, `<usd_amount>`, `<coin> <usd_to_spend>`, `<coin> <usd_to_receive>` | Keep | Names expose currency and transaction intent without type noise. |
| Gold commands | `<vnd_amount>`, `<luong>` | Keep | Units are essential and concise. |
| Stock price/trade commands | `<ticker>`, `<vnd_amount>`, `<quantity> <ticker>` | Keep | Conventional positional metavariables. |
| Stock dividend commands | `<vnd_per_share>`, `<ratio(owned:new)>`, `<ticker>` combinations | Keep | The ratio shape is valuable syntax, not redundant prose. |

Result: change two registrations and their matching usage/tests; leave every
other public parameter string unchanged.

## Rejected Alternatives

### Uppercase metavariables

```text
<TICKER> <QUANTITY>
```

Familiar in man pages, but redundant when angle brackets already mark values.
It is visually louder and would touch every command for no behavior gain.

### Inline type annotations

```text
<quantity:number> <ticker:string>
```

Not a common shell synopsis convention. Longer, falsely formal, and still
cannot express real constraints such as positive integers or known tickers.

### Generic names everywhere

```text
<amount> <value> <input>
```

Short but ambiguous. Minimalism should remove redundancy, not meaning.

### Full docopt grammar

Docopt can represent required groups, nested alternatives, and repetition
precisely. The bot does not need that complexity. Adopt only its familiar
surface notation when a real command requires it.

## Next Steps

1. Change `/random` and `/wheelofnames` parameter metadata to `<option,...>`.
2. Change their handler usage strings and exact-string tests at the same time.
3. Update README/AGENTS guidance: value format belongs in the summary or usage
   error unless punctuation is essential, as with `<ratio(owned:new)>`.
4. Keep current registration validation: non-empty descriptions, single-line
   parameters, and Telegram's length limit.
5. Do not build a schema parser.

## References

- [POSIX.1-2024, Utility Conventions](https://pubs.opengroup.org/onlinepubs/9799919799/basedefs/V1_chap12.html)
- [GNU Standards, Command-Line Interfaces](https://www.gnu.org/prep/standards/html_node/Command_002dLine-Interfaces.html)
- [Microsoft command-line syntax key](https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/command-line-syntax-key)
- [Python 3 argparse documentation](https://docs.python.org/3/library/argparse.html)
- [docopt usage-pattern grammar](https://github.com/docopt/docopt/blob/master/README.rst)
- [Telegram Bot API](https://core.telegram.org/bots/api#botcommand)

## Unresolved Questions

None. The recommendation is intentionally small and implementable without
changing command parsing or persisted data.
