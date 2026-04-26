# MongoDB Atlas Cost Tracking

Operational runbook for monitoring and managing MongoDB Atlas billing during and post-migration.

## Free Tier Limits (M0)

| Resource | Limit | Warning |
|----------|-------|---------|
| Storage | 512 MB | Reached 400 MB: prepare upgrade plan |
| Connections | 500 | Reached 400: review cron/handler concurrency |
| Throughput | ~100 ops/sec sustained | Degradation: check dashboard Charts |
| Backups | None | No PITR; only daily snapshots (read-only) |

## Cost Ladder

| Tier | Monthly | When | Ops/sec | Storage |
|------|---------|------|---------|---------|
| **M0** | Free | Initial, dev | ~100 | 512 MB |
| **Flex** | $8–$30 | Sustained >400 MB or >400 conn | 400–1000 | 10–256 GB |
| **M10** | $57 | Production high-throughput | 1000+ | Unlimited |

Flex tier auto-scales cost based on data volume and throughput. Start at M2 ($9) and scale to M5 ($70+) as needed.

## Monitoring & Upgrade Triggers

### Monthly Review Checklist

1. **Storage:** Open Atlas Dashboard → Metrics → check `Database Storage` chart
   - If > 400 MB: escalate to Flex within 2 weeks
   - Projection: (current MB / days since epoch) * 30 → projected month-end

2. **Connections:** Metrics → check `Current Connections` peak
   - If > 400: cron jobs or handlers running concurrently too often
   - Review `src/modules/*/index.js` cron frequency; stagger if possible

3. **Throughput:** Metrics → `Network Egress` + `Database Operations`
   - Spike during trading/wordle command storms: expected
   - Sustained >100 ops/sec: assess Flex upgrade

4. **Cluster Status:** Alerts → check if "Cluster unavailable" triggered
   - M0 never pauses if any cron writes data (bot has 6+ crons, any write prevents pause)

### Upgrade Decision

Plan upgrade **before** hitting limits:

| Condition | Action | Timeline |
|-----------|--------|----------|
| Storage trend → 512 MB in 3 months | Upgrade to Flex M2 | 2 weeks notice |
| Peak connections > 400 regularly | Upgrade to Flex M2 | Immediate if sustained |
| Ops/sec spikes > 2000 | Upgrade to Flex M5 or M10 | 1 week notice |

## Rotation & Maintenance

- **Password rotation:** Every 90 days, owner = repo maintainer. See `docs/using-mongodb.md` "Rotation" section.
- **Alert config review:** Monthly, ensure Atlas + CF Observability alerts are wired.
- **Backups:** M0 has none; data is live-only. Backups start at Flex tier.

## Post-Cutover Simplification

Once Phase 07 cutover completes and Cloudflare KV/D1 are deleted, MongoDB becomes the sole data store. Cost is dominated by storage growth (KV reads/writes are gone). Rebaseline this doc:

- Remove dual-write cost considerations
- Focus on pure Mongo spend
- Extend storage projections based on new single-source data
