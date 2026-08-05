---
phase: 1
title: Feasibility and mapping discovery
status: completed
effort: 'S — curl session only, no production code'
---

# Phase 1: Feasibility and mapping discovery

## Overview

Answer every open question with the live token before writing code. **Done
2026-08-05** — all questions answered, no assumption broke badly enough to
reshape Phase 2. ~7 quota requests used. Findings below supersede the
original question list.

## Findings (verified live 2026-08-05)

1. **Auth + quota.** Bearer header works; `HTTP 200`;
   `x-rate-limit-remaining: 999` after first call → 1000/h free tier
   confirmed. Token in repo `.env` as `LOL_PANDASCORE_TOKEN` (51 chars);
   `.env` is NOT shell-sourceable (unquoted `&` on line 11) — Go reads env
   directly, irrelevant for production.
2. **Window recipe.**
   `GET /lol/matches?range[begin_at]=<fromISO>,<toISO>&sort=begin_at&per_page=100`
   → 77 matches for a 2-day window, ascending. **Upper range bound is
   INCLUSIVE** (event at exactly `to` returned) → client-side `[from,to)`
   filter is mandatory, keep it. Pagination: `Link` headers rel=next/last
   AND `page=N` both available → walk `page` while `len==per_page`.
   Response is a **top-level JSON array** (no envelope).
3. **Status vocabulary** (observed): `not_started`, `running`, `finished`.
   Map to `unstarted`/`inProgress`/`completed`; **drop any other status**
   (canceled, postponed) defensively.
4. **League slug mapping** (from `/lol/leagues`, 133 leagues, 3 pages):

   | canonical (format.go) | PandaScore slug | id |
   |---|---|---|
   | lck | league-of-legends-lck-champions-korea | 293 |
   | lpl | league-of-legends-lpl-china | 294 |
   | lec | league-of-legends-lec | 4197 |
   | lcs | league-of-legends-lcs | 4198 |
   | lcs | league-of-legends-lta-north | 5345 | (LTA-era NA top flight; slug canonicalized, display name passes through) |
   | worlds | league-of-legends-world-championship | 297 |
   | msi | league-of-legends-mid-invitational | 300 |
   | first_stand | league-of-legends-first-stand | 5369 |
   | ewc_lol | league-of-legends-esports-world-cup | 5262 |
   | lcp | league-of-legends-lcp | 5351 |
   | cblol-brazil | league-of-legends-cblol-brazil | 302 |
   | emea_masters | league-of-legends-emea-masters | 4996 |

   Active majors verified in live windows: lck, lpl, lec, cblol-brazil, lcp.
5. **Field shapes.**
   - `begin_at`: RFC3339 `Z` — parses with existing `time.RFC3339`.
   - `results[]`: `{team_id, score}` present in ALL states (0-0 upcoming,
     1-1 running, final on finished) → live 🔴 scores work. Join to
     opponents strictly by `team_id`.
   - `winner_id`: non-null only when finished → Outcome win/loss derived
     from `status==finished && winner_id != nil`; finished without winner_id
     → no outcome → existing `☑️ score pending` path (semantics preserved).
   - `number_of_games` → Strategy.Count; `opponents[].opponent.{acronym,name,image_url}`
     → Team.Code/Name/Image.
   - **BlockName ← `tournament.name`** ("Regular Season", "Group Ascend",
     "Playoffs") — closest analogue to old block labels.
   - `name` is "MCN vs BAN" — redundant, unused.
6. **TBD.** 0 matches with <2 opponents in 100-match upcoming sample;
   mapping must still tolerate `len(opponents)<2` (distant playoffs) —
   `formatEventLine`/`teamLabel` already render "TBD" for missing teams.
7. **ToS.** Docs state Fixtures-Only plan free for all users; no explicit
   attribution requirement found in developer docs. Low risk; see open
   question in plan.md.

## Success Criteria

- [x] 200 with token; quota headers recorded; free-tier headroom confirmed
- [x] Date-window recipe verified incl. pagination behavior
- [x] Status vocabulary + mapping decided
- [x] League mapping table fully filled
- [x] BlockName source + TBD shape decided
- [x] ToS/attribution answer recorded (docs level; full ToS text unread)
- [x] Phase 2 adjustments: none structural — confirmed top-level array,
      inclusive upper bound, results-in-all-states, lta-north dual mapping

## Risk Assessment (resolved)

- Worlds/MSI are ordinary leagues (297/300) — no serie-based lookup needed.
- EWC/First Stand exist as leagues (5262/5369) — no gaps.
