# /gacha wish command

Status: done

## Outcome

`/gacha <option[*rarity],...>` picks one option and replies with a Genshin
Impact style wish animation (meteor coloured by rarity, white flash, reveal
with stars), rendered as a silent MP4 by the existing wheelofnames service.

## Decisions (user-approved 2026-10-01)

- Renderer lives in `tiennm99/wheelofnames` as `POST /api/gacha`; no new repo
  or deployment.
- Rarity is user-tagged: `Pizza*5, Pho*4, Rice` (untagged = 3★).
- Single pull only.
- Output MP4 (H.264, no audio) sent via `sendAnimation`.

## Design

- Every option is equally likely, as with /random; the rarity tag is
  cosmetic. (First shipped with Genshin tier rates; the user switched to
  uniform odds on 2026-10-01.)
- All visuals are procedural (CSS/gradients); no HoYoverse assets.
- Bot derives the gacha endpoint from `WHEELOFNAMES_API_URL` (sibling path
  `gacha` next to `gif`), so no new env var. Same bearer token.
- Bot is the source of truth for winner and rarity; renderer only draws.
- Fallback without renderer or on failure: text reply `★★★★★ Pizza`.

## Phases

1. Renderer: schema, `GachaComposition`, MP4 render, route, tests, docs.
2. Bot: parser, weighted pick, API client, command, tests, README/docs.
3. Verify: renderer stills via Docker, lint/typecheck/test, go test/vet/lint.

## Acceptance

- Each rarity renders with the correct meteor/glow colour and star count.
- `/gacha` with no options shows usage; untagged options behave like /random.
- Renderer failure falls back to text; thread IDs forwarded.
