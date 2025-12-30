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

Services are wired up. Gateway routes to controlplane which selects tier based on budget and calls workers. Postgres stores audit logs, Prometheus collects metrics.

## Services

- Gateway (8080): Entry point, logs requests to DB
- Controlplane (8081): Tier selection logic
- Workers (8090-8092): Model inference
- Postgres (5432): Audit logging
- Prometheus (9090): Metrics

## TODO

- [ ] Add Grafana dashboards
- [ ] Improve decision algorithm
- [ ] Add retry logic
- [ ] Load testing
