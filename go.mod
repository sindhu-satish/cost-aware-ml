module github.com/cost-aware-ml

go 1.22

require (
	github.com/lib/pq v1.10.9
	github.com/prometheus/client_golang v1.20.5
	github.com/redis/go-redis/v9 v9.5.1
	github.com/nats-io/nats.go v1.34.0
	go.opentelemetry.io/otel v1.27.0
	go.opentelemetry.io/otel/attribute v1.27.0
	go.opentelemetry.io/otel/trace v1.27.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.27.0
	go.opentelemetry.io/otel/sdk v1.27.0
	go.opentelemetry.io/otel/propagation v1.27.0
)
