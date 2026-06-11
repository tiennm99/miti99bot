---
title: "Gold price command"
description: "Add /gold_price command showing spot XAU in USD/oz and VND/luong"
status: completed
priority: P2
branch: "main"
tags: [gold]
blockedBy: []
blocks: []
created: "2026-06-11T10:44:35.588Z"
createdBy: "ck:plan"
source: skill
---

# Gold price command

## Overview

Add `/gold_price` to the gold module. Displays current spot gold price in both USD per troy ounce and VND per luong. No arguments, no portfolio mutation — read-only price lookup.

## Context

`GoldPriceClient` already fetches XAU USD/oz and USD/VND internally via unexported `fetchXAUUSD` and `fetchUSDVND`. Currently only `FetchLuongPrice` (combined VND/luong) is exposed. Need a new method returning all components.

## Phases

| Phase | Name | Status |
|-------|------|--------|
| 1 | [Implement gold_price command](./phase-01-implement-gold-price-command.md) | Pending |

## Dependencies

None. Builds on committed gold module (`2304657`).
