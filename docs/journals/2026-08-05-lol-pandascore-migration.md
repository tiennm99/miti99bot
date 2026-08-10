# Decade-Old Riot API Dies; Twice-Rewritten Transport in One Day

**Date**: 2026-08-05 morning/afternoon
**Severity**: Critical
**Component**: lol module (Go Telegram bot, LoL esports schedules)
**Status**: Resolved

## What Happened

The bot's lol schedule fetch started failing HTTP 403. Investigation revealed Riot revoked the decade-old x-api-key for esports-api.lolesports.com. The new lolesports.com is a Next.js/Netlify site keeping its API key server-side behind /api/gql persisted-query proxies. Official replacement (GRID/formerly Bayes LDP) is paid-only. Two transports were implemented and shipped the same day.

## The Brutal Truth

This is incredibly frustrating because a clever same-day reverse-engineering win was immediately exposed as unmaintainable and abandoned. Commit b76d0ca reverse-engineered lolesports.com /api/gql by extracting the persisted-query ID manifest from webpack chunk 29—extracting sha256 hashes failed; the Apollo client uses `generatePersistedQueryIdsFromManifest`, and the manifest URL came from the webpack runtime's chunk-hash map. It worked. We verified it live on a headless ARM64 server without a browser. And then we threw it away. The second rewrite took a fraction of the effort because the ScheduleEvent boundary held—architecture won out over cleverness.

## Technical Details

**b76d0ca (reverse-engineering, abandoned)**: Persisted query approach worked on first live probe, pulling real homeEvents data from lolesports.com /api/gql proxy.

**15f7e50 (PandaScore, shipped)**: REST transport to /lol/matches with Bearer token auth. Persisted ScheduleEvent contract and bson cache untouched. League slugs canonicalized via 12-entry mapping (e.g., `league-of-legends-lck-champions-korea` → `lck`). Results joined by team_id; outcome only set when winner_id present (preserves "score pending" semantics). Code review caught silent page-budget truncation; fixed with warn log and budget increase (3→5). Live probe rendered a real week with LIVE JDG 1–0 LGD—bracket-encoding verified correct.

## Root Cause Analysis

Riot's infrastructure changed, not our code. But the reverse-engineered approach depended on webpack chunk IDs and manifest URLs rotating with each frontend deploy—a hidden maintenance tax. The persisted-query ID schema is Riot's internal concern; every redeployment could break the mapping without warning. We should have recognized this immediately instead of celebrating the win.

## Lessons Learned

**Data-source sovereignty beats cleverness.** A PandaScore REST endpoint (versioned, documented, intentionally exposed) beats reverse-engineered gql IDs. **Stable boundaries matter.** The ScheduleEvent contract was unchanged between transport rewrites; that boundary made the second pass clean and fast. **Reverse-engineering is a temporary fix.** It solved a crisis, but architecture decides sustainability.

## Next Steps

None—migration complete and live-verified. Monitor PandaScore rate limits (1000 req/h free tier). Superseded pending Leaguepedia enrichment plan (260726-0952) since PandaScore results already carry scores.
