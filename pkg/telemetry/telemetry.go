package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cost-aware-ml/pkg/decision"
)

type Collector struct {
	prometheusURL string
	client        *http.Client
}

type PrometheusResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

func NewCollector(prometheusURL string) *Collector {
	if prometheusURL == "" {
		prometheusURL = "http://prometheus:9090"
	}
	return &Collector{
		prometheusURL: prometheusURL,
		client: &http.Client{
			Timeout: 2 * time.Second,
		},
	}
}

func (c *Collector) CollectTelemetry(ctx context.Context) (decision.Telemetry, error) {
	telemetry := decision.Telemetry{
		P99LatencyMS: make(map[decision.Tier]int),
		ErrorRate:    make(map[decision.Tier]float64),
		QueueDepth:   make(map[decision.Tier]int),
	}

	for _, tier := range []decision.Tier{decision.Tier0, decision.Tier1, decision.Tier2} {
		tierStr := string(tier)

		p99Latency, err := c.queryP99Latency(ctx, tierStr)
		if err == nil {
			telemetry.P99LatencyMS[tier] = p99Latency
		}

		errorRate, err := c.queryErrorRate(ctx, tierStr)
		if err == nil {
			telemetry.ErrorRate[tier] = errorRate
		}

		queueDepth, err := c.queryQueueDepth(ctx, tierStr)
		if err == nil {
			telemetry.QueueDepth[tier] = queueDepth
		}
	}

	return telemetry, nil
}

func (c *Collector) queryP99Latency(ctx context.Context, tier string) (int, error) {
	query := fmt.Sprintf(`histogram_quantile(0.99, rate(gateway_request_duration_seconds_bucket{tier="%s"}[5m])) * 1000`, tier)
	value, err := c.queryPrometheus(ctx, query)
	if err != nil {
		return 0, err
	}
	return int(value), nil
}

func (c *Collector) queryErrorRate(ctx context.Context, tier string) (float64, error) {
	query := fmt.Sprintf(`sum(rate(gateway_requests_total{status=~"error|worker_error|controlplane_error",tier="%s"}[5m])) / sum(rate(gateway_requests_total{tier="%s"}[5m]))`, tier, tier)
	value, err := c.queryPrometheus(ctx, query)
	if err != nil {
		return 0, err
	}
	if value > 1.0 {
		value = 1.0
	}
	return value, nil
}

func (c *Collector) queryQueueDepth(ctx context.Context, tier string) (int, error) {
	query := `gateway_queue_depth`
	value, err := c.queryPrometheus(ctx, query)
	if err != nil {
		return 0, err
	}
	return int(value), nil
}

func (c *Collector) queryPrometheus(ctx context.Context, query string) (float64, error) {
	url := fmt.Sprintf("%s/api/v1/query?query=%s", c.prometheusURL, query)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("prometheus query failed: %s", string(body))
	}

	var result PrometheusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	if result.Status != "success" || len(result.Data.Result) == 0 {
		return 0, fmt.Errorf("no data from prometheus")
	}

	valueStr, ok := result.Data.Result[0].Value[1].(string)
	if !ok {
		return 0, fmt.Errorf("invalid value type")
	}

	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return 0, err
	}

	if strings.Contains(query, "NaN") || value != value {
		return 0, fmt.Errorf("NaN value")
	}

	return value, nil
}

