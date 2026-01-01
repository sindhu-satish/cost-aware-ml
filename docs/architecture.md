# Architecture

## Overview

The Cost-Aware ML Inference Optimizer routes inference requests across multiple model tiers to optimize cost while meeting latency SLOs and quality thresholds.

## Components

### Gateway (`/cmd/gateway`)
- HTTP entry point
- Rate limiting (Redis token bucket)
- Response caching (Redis)
- Request queuing with backpressure
- OpenTelemetry trace propagation
- Metrics: request_count, latency, errors

### Controlplane (`/cmd/controlplane`)
- Decision engine for tier selection
- Policy evaluation (budget, latency SLO, confidence)
- Circuit breaker state management
- Escalation logic (tier0 → tier1 → tier2)
- Telemetry collection from Prometheus (P99 latency, error rates, queue depth)
- Event publishing to NATS (`inference.decisions.<tenant>`)

### Workers (`/services/workers`)
- **tier0_fast**: Fast, cheap model (~15ms, 72% confidence)
- **tier1_mid**: Balanced model (~85ms, 88% confidence)
- **tier2_best**: Best quality model (~250ms, 96% confidence)

### Infrastructure
- **Postgres**: Config, audit logs, tenant policies
- **Redis**: Caching, rate limiting tokens
- **NATS**: Event streaming for decisions
- **Prometheus**: Metrics collection
- **Grafana**: Dashboards
- **OTel Collector**: Trace aggregation

## Data Flow

```
Client → Gateway → Controlplane → Worker
                ↓
            Redis (cache/ratelimit)
                ↓
            Postgres (audit)
                ↓
            NATS (events)
```

## Decision Logic

1. Check cache (Redis) - return if hit
2. Apply rate limiting (Redis token bucket)
3. Call controlplane with request context
4. Controlplane collects telemetry from Prometheus (P99 latency, error rates, queue depth)
5. Controlplane evaluates:
   - Budget constraints
   - Latency SLO
   - Confidence threshold
   - Circuit breaker state
   - Telemetry data (P99 latency, error rates)
6. Start at tier0, escalate if confidence < threshold and budget allows
7. Publish decision event to NATS
8. Return result with tier, confidence, cost, latency

## Cost Model

- Base cost per tier (fixed)
- Compute cost: cost_per_ms * latency_ms
- Calibrated from observed worker latencies

## Reliability

- Circuit breakers per tier (failure threshold, cooldown)
- Retries with jitter (max 2 attempts)
- Timeouts (10s gateway, 30s workers)
- Backpressure: bounded queue, 429 when full
- Fallback: degraded response if all tiers down

## Tradeoffs

- **Latency vs Cost**: Cheaper tiers are faster but less accurate
- **Confidence vs Budget**: Escalation requires budget headroom
- **Cache vs Freshness**: Cached responses save cost but may be stale
- **Retries vs Load**: Retries increase load but improve reliability

