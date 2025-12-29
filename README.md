# cost-aware-ml

Experimenting with tiered ML inference routing to balance cost vs quality.

## Idea

Route inference requests to different model tiers based on budget/latency constraints:
- **tier0**: fast/cheap (~15ms)
- **tier1**: balanced (~85ms)
- **tier2**: best quality (~250ms)

## Run

```bash
cp env.example .env
make up
```

## Status

WIP - basic scaffolding only. Gateway returns 501 for now.

## TODO

- [ ] Wire up gateway → controlplane → workers
- [ ] Add actual decision logic
- [ ] Postgres for audit logs
- [ ] Metrics
