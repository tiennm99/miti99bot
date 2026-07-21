# Portfolio Cleanup Completion Report

## Summary

| Item | Result |
|---|---|
| Coin dividend cursor | Removed from schema, API, validation, callers, tests |
| Stock dividend cursor | Retained unchanged |
| Portfolio migrations | Completed runtime paths and migration-only tests removed |
| Stats rename migration | Removed; recurring indexes retained |
| LoL startup maintenance | TTL index retained |
| Historical system data | Untouched; reusable helper retained |
| Dividend API research | SSI iBoard recommended behind provider interface |

## Verification

- Focused and full Go tests passed.
- MongoDB 8 stats-index and LoL TTL-index tests executed and passed.
- `go vet ./...`, `go build ./...`, and `golangci-lint run` passed.
- `git diff --check` passed.
- Independent tester, debugger, and reviewer reported no defects.
- Coin BSON regression coverage proves a stale cursor loads safely and is
  omitted by the next whole-document encoding/write.

## Documentation

- README and deployment guide now distinguish stock and coin asset schemas.
- Standalone dividend API research report added under `plans/reports/`.
- Existing completed implementation plans remain historical records; no phase
  statuses changed.

## Known Limitations

- Untouched MongoDB coin documents retain the ignored cursor until their next
  portfolio write.
- SSI iBoard corporate actions are undocumented and have no published SLA.

## Next Steps

1. Commit the approved cleanup when requested.
2. Design the Telegram dividend-event selection/confirmation interaction.
3. Implement SSI behind a replaceable provider interface after command design
   approval.

## Unresolved Questions

- Should dividend lookup inspect one ticker per command or all stock assets?
- Should an event only prefill guidance or execute after explicit confirmation?
