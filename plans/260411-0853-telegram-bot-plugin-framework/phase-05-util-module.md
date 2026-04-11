# Phase 05 — util module (`/info`, `/help`)

## Context Links
- Plan: [plan.md](plan.md)
- Phase 04: [module framework](phase-04-module-framework.md)

## Overview
- **Priority:** P1
- **Status:** pending
- **Description:** fully-implemented `util` module with two public commands. `/info` reports chat/thread/sender IDs. `/help` iterates the registry and prints public+protected commands grouped by module.

## Key Insights
- `/help` is a **renderer** over the registry — it does NOT hold its own command metadata. Single source of truth = registry.
- Forum topics: `message_thread_id` may be absent for normal chats. Output "n/a" rather than omitting, so debug users know the field was checked.
- Parse mode: **HTML** (decision locked). Easier escaping than MarkdownV2. Only 4 chars to escape: `&`, `<`, `>`, `"`. Write a small `escapeHtml()` util.
- `/help` must access the registry. Use an exported getter from `src/modules/dispatcher.js` or `src/modules/registry.js` that returns the currently-built registry. The util module reads it inside its handler — not at module load time — so the registry exists by then.

## Requirements
### Functional
- `/info` replies with:
  ```
  chat id: 123
  thread id: 456  (or "n/a" if undefined)
  sender id: 789
  ```
  Plain text, no parse mode needed.
- `/help` output grouped by module:
  ```html
  <b>util</b>
  /info — Show chat/thread/sender IDs
  /help — Show this help

  <b>wordle</b>
  /wordle — Play wordle
  /wstats — Stats (protected)
  ...
  ```
  - Modules with zero visible commands omitted entirely.
  - Private commands skipped.
  - Protected commands appended with `" (protected)"` suffix so users understand the distinction.
  - Module order: insertion order of `env.MODULES`.
  - Sent with `parse_mode: "HTML"`.
- Both commands are **public** visibility.

### Non-functional
- `src/modules/util/index.js` < 150 LOC.
- No new deps.

## Architecture
```
src/modules/util/
├── index.js          # module default export
├── info-command.js   # /info handler
├── help-command.js   # /help handler + HTML renderer
```

Split by command file for clarity. Each command file < 80 LOC.

Registry access: `src/modules/registry.js` exports `getCurrentRegistry()` returning the memoized instance (set by `buildRegistry`). `/help` calls this at handler time.

## Related Code Files
### Create
- `src/modules/util/index.js`
- `src/modules/util/info-command.js`
- `src/modules/util/help-command.js`
- `src/util/escape-html.js` (shared escaper)

### Modify
- `src/modules/registry.js` — add `getCurrentRegistry()` exported getter

### Delete
- none

## Implementation Steps
1. `src/util/escape-html.js`:
   ```js
   export function escapeHtml(s) {
     return String(s)
       .replaceAll("&", "&amp;")
       .replaceAll("<", "&lt;")
       .replaceAll(">", "&gt;")
       .replaceAll('"', "&quot;");
   }
   ```
2. `src/modules/registry.js`:
   - Add module-scope `let currentRegistry = null;`
   - `buildRegistry` assigns to it before returning.
   - `export function getCurrentRegistry() { if (!currentRegistry) throw new Error("registry not built yet"); return currentRegistry; }`
3. `src/modules/util/info-command.js`:
   - Exports `{ name: "info", visibility: "public", description: "Show chat/thread/sender IDs", handler }`.
   - Handler reads `ctx.chat?.id`, `ctx.message?.message_thread_id`, `ctx.from?.id`.
   - Reply: `\`chat id: ${chatId}\nthread id: ${threadId ?? "n/a"}\nsender id: ${senderId}\``.
4. `src/modules/util/help-command.js`:
   - Exports `{ name: "help", visibility: "public", description: "Show this help", handler }`.
   - Handler:
     - `const reg = getCurrentRegistry();`
     - Build `Map<moduleName, string[]>` of lines.
     - Iterate `reg.publicCommands` + `reg.protectedCommands` (in insertion order; `Map` preserves it).
     - For each entry, push `"/" + cmd.name + " — " + escapeHtml(cmd.description) + (visibility === "protected" ? " (protected)" : "")` under its module name.
     - Iterate `reg.modules` in order; for each with non-empty lines, emit `<b>${escapeHtml(moduleName)}</b>\n` + lines joined by `\n` + blank line.
     - `await ctx.reply(text, { parse_mode: "HTML" });`
5. `src/modules/util/index.js`:
   - `import info from "./info-command.js"; import help from "./help-command.js";`
   - `export default { name: "util", commands: [info, help] };`
6. Add `util` to `wrangler.toml` `MODULES` default if not already: `MODULES = "util,wordle,loldle,misc"`.
7. Lint.

## Todo List
- [ ] `escape-html.js`
- [ ] `getCurrentRegistry()` in registry.js
- [ ] `info-command.js`
- [ ] `help-command.js` renderer
- [ ] `util/index.js`
- [ ] Manual smoke test via `wrangler dev`
- [ ] Lint clean

## Success Criteria
- `/info` in a 1:1 chat shows chat id + "thread id: n/a" + sender id.
- `/info` in a forum topic shows a real thread id.
- `/help` shows `util` section with both commands, and stub module sections (after phase-06).
- `/help` does NOT show private commands.
- Protected commands show `(protected)` suffix.
- HTML injection attempt in module description (e.g. `<script>`) renders literally.

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Registry not built before handler fires | Low | Med | `getCurrentRegistry()` throws with clear error; dispatcher ensures build before bot handles updates |
| `/help` output exceeds Telegram 4096-char limit | Low (at this scale) | Low | Phase-09 mentions future pagination; current scale is fine |
| Module descriptions contain raw HTML | Med | Med | `escapeHtml` all descriptions + module names |
| Missing `message_thread_id` crashes | Low | Low | `?? "n/a"` fallback |

## Security Considerations
- Escape ALL user-influenced strings (module names, descriptions) — even though modules are trusted code, future-proofing against dynamic registration.
- `/info` reveals sender id — that's the point. Document in help text that it's a debugging tool.

## Next Steps
- Phase 06 adds the stub modules that populate the `/help` output.
- Phase 08 tests `/help` rendering against a synthetic registry.
