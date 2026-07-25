# Animation and Confetti Refinement

**Status:** complete. Phases 1-4 implemented and verified on 2026-07-25.
**Branch:** main
**Depends on:** the celebration rework already on `main` (two-cannon confetti,
winner pill, SVG pointer, slope-matched spin, grayscale label AA)

Second pass over the rendered animation. The first pass fixed defects; this pass
is about motion truth and craft, plus one documentation correction.

## Accepted trade already decided

Grayscale label antialiasing stays, at +22.4% GIF size (+463KB on the 512
default). Clean text and an unstarved palette are worth the bytes. The offset
programme that would claw them back (opaque bezel replacing the soft
`wheelShadow`, halo cut from 4 offsets to 2) is explicitly **out of scope** —
see Not doing.

## Phases

| # | Phase | Depends on | Risk | Outcome |
|---|---|---|---|---|
| 1 | [Motion truth](phase-01-motion-truth.md) — adaptive turn count, settle recoil | — | medium | done |
| 2 | [Confetti craft](phase-02-confetti-craft.md) — motion streaks, second wave | — | low | done |
| 3 | [Label orientation](phase-03-label-orientation.md) — the 90/270 vertical case | — | medium | done, option A only |
| 4 | [Verification and docs](phase-04-verification-and-docs.md) | 1, 2, 3 | low | done |

Phases 1-3 touch disjoint files and can run in any order or in parallel. Phase 4
re-measures everything once they land.

## Outcome

Turn count now scales with the frame budget: 5 turns at 8 options as before, 2 at
16, 1 at 24 and above. Aliasing goes from 65 of 98 frames at 32 options to zero
at every count the frame budget can serve.

The open risk — that one turn would read as a broken wheel rather than a slow
one — did not materialise. At 32 options the rim still moves 30px per frame and
77 of 98 frames are perceptibly in motion, so the fallback floor of 2 turns was
not needed.

Confetti now holds a populated frame throughout: a 2500ms hold ended with 0
particles before and 68 after. Streaks are strongest at launch and relax as chips
slow.

Reducing turns on dense wheels also cut the schema-maximum render from 14.4MB and
8999ms to 12.0MB and 7800ms.

`88 tests passing`, lint and typecheck clean.

## Acceptance criteria

- No spin frame advances more than one slice, at every option count, except
  where the frame budget makes it impossible (3000ms at 12fps leaves 36 spin
  frames; 24+ slices cannot avoid it). That exception is asserted, not ignored.
- The winner still lands at exactly screen angle 0 for every option count and
  winner index, including through the recoil.
- Confetti holds a populated frame for the whole celebration at `holdMs` 500,
  1200, and 2500.
- No label rests within 15 degrees of vertical, or the vertical case is handled
  deliberately and consistently.
- `pnpm lint`, `pnpm typecheck`, `pnpm test` clean. Every new behavior has a test
  asserting the invariant rather than a magic number.
- `docs/render-benchmarks.md` states the correct byte attribution.

## Not doing

Carried from the two review reports, deliberately excluded:

- Bezel ring and reduced `wheelShadow`. Would free ~90 palette entries, but the
  size trade was decided against pursuing.
- Text halo reduced from 4 offsets to 2.
- Raising the label font-size cap (`radius * 0.15`), which would grow text ~31%
  on 2-6 option wheels.
- A 4th `night` theme. Additive schema change, still needs a call.
- Narrowing the request schema to cap total frames. The 32-option maximum is a
  14.4MB GIF, but render time is back at baseline (8999ms) so nothing is
  breaching a timeout today. Revisit only if a container actually times out.
- `numberOfGifLoops`. The GIF keeps looping forever.
- Everything in the API/DX review — separate track.
