# Stock Dividend Command Plan Journal

## Context

Created the implementation plan at
`plans/260720-1616-stock-dividend-commands/` from the approved research and
brainstorm decisions.

## What Happened

- Added three pending phases: implement ratio dividend commands, migrate command
  statistics, and verify user-facing contracts.
- Planned `/stock_cash_dividend` with VND/share, `/stock_share_dividend` with an
  `owned:new` ratio, and combined `/stock_dividend` using both inputs.
- Preserved floor rounding, pre-event holdings, and one atomic portfolio save.
- Defined zero-share behavior: reject share-only events; combined events still
  credit cash.
- Tightened migration design after review: each stats row gets a `prepared`
  checkpoint containing its exact final target count. Crash retries set that
  count instead of incrementing again.
- Could not persist the active-plan selection because `CK_SESSION_ID` was unset.

## Status

- Plan: pending
- Progress: 0/3 phases complete
- Source implementation: not started

## Next Steps

- Execute the three planned phases, beginning with command contracts and tests.
- Keep the stats migration before module registration so the reused
  `/stock_dividend` name cannot misattribute historical usage.
