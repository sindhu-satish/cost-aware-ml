# Cost-Aware ML Inference Optimizer

Production-grade system that routes ML inference requests across multiple model tiers to optimize cost while meeting latency SLOs and quality thresholds.

## Architecture

```
Client → Gateway → Controlplane → Worker
                ↓
            Redis (cache/ratelimit)
                ↓
            Postgres (audit)
                ↓
            NATS (events)
```

See [docs/architecture.md](docs/architecture.md) for detailed design.

## Quick Start

```bash
# Start all services
make up

# Check health
curl http://localhost:8080/healthz

# Run inference
curl -X POST http://localhost:8080/infer \
  -H "Content-Type: application/json" \
  -d '{"request_id": "test-1", "user_id": "user-1", "tenant_id": "tenant-1", "input": "test", "budget": 5.0}'

# View dashboards
open http://localhost:3000  # Grafana (admin/admin)
open http://localhost:9090  # Prometheus
```

## Services

- **Gateway** (8080): HTTP entry point, caching, rate limiting
- **Controlplane** (8081): Decision engine for tier selection
- **Workers** (8090-8092): Model inference tiers
- **Postgres** (5432): Config, audit logs
- **Redis** (6379): Caching, rate limiting
- **NATS** (4222): Event streaming
- **Prometheus** (9090): Metrics
- **Grafana** (3000): Dashboards

## Features

- **Intelligent Routing**: Budget, latency SLO, confidence-based tier selection with escalation
- **Cost Optimization**: 63% cost reduction (measured: $0.0185 vs $0.05 per request) - see [benchmarks](docs/benchmarks.md)
- **Reliability**: Circuit breakers per tier, retry logic with exponential backoff, backpressure handling
- **Observability**: OpenTelemetry traces end-to-end, Prometheus metrics, Grafana dashboards
- **Caching**: Redis-based response caching with cache hit/miss metrics
- **Rate Limiting**: Token bucket per tenant with configurable limits

## Development

```bash
make up      # Start all services
make down    # Stop services
make build   # Build binaries
make test    # Run tests
make load    # Run load tests
make logs    # View logs
```

## Documentation

- [Architecture](docs/architecture.md) - System design and tradeoffs
- [Benchmarks](docs/benchmarks.md) - Performance and cost analysis with real results

## Testing

```bash
# Unit tests
make test

# Load testing
make load

# Integration tests (manual)
curl -X POST http://localhost:8080/infer -d '{"request_id": "test", "budget": 5.0}'
```

## License

MIT
