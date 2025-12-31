package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/cost-aware-ml/pkg/circuitbreaker"
	"github.com/cost-aware-ml/pkg/client"
	"github.com/cost-aware-ml/pkg/decision"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	decisionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "controlplane_decisions_total",
			Help: "Total number of tier decisions",
		},
		[]string{"tier", "reason"},
	)
	decisionDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "controlplane_decision_duration_seconds",
			Help:    "Decision duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)
	escalationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "controlplane_escalations_total",
			Help: "Total number of tier escalations",
		},
		[]string{"from", "to"},
	)
	circuitBreakerState = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "controlplane_circuit_breaker_state",
			Help: "Circuit breaker state (0=closed, 1=open, 2=half-open)",
		},
		[]string{"tier"},
	)
)

func init() {
	prometheus.MustRegister(decisionsTotal)
	prometheus.MustRegister(decisionDuration)
	prometheus.MustRegister(escalationsTotal)
	prometheus.MustRegister(circuitBreakerState)
}

var tier0URL = os.Getenv("TIER0_URL")
var tier1URL = os.Getenv("TIER1_URL")
var tier2URL = os.Getenv("TIER2_URL")

func main() {
	if tier0URL == "" {
		tier0URL = "http://tier0-fast:8090"
	}
	if tier1URL == "" {
		tier1URL = "http://tier1-mid:8091"
	}
	if tier2URL == "" {
		tier2URL = "http://tier2-best:8092"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	engine := decision.NewEngine()
	clients := map[decision.Tier]*client.WorkerClient{
		decision.Tier0: client.New(tier0URL),
		decision.Tier1: client.New(tier1URL),
		decision.Tier2: client.New(tier2URL),
	}

	circuitBreakers := map[decision.Tier]*circuitbreaker.CircuitBreaker{
		decision.Tier0: circuitbreaker.New(5, 3, 30*time.Second),
		decision.Tier1: circuitbreaker.New(5, 3, 30*time.Second),
		decision.Tier2: circuitbreaker.New(5, 3, 30*time.Second),
	}

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		for range ticker.C {
			for tier, cb := range circuitBreakers {
				state := cb.State()
				circuitBreakerState.WithLabelValues(string(tier)).Set(float64(state))
			}
		}
	}()

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "controlplane"})
	})

	http.Handle("/metrics", promhttp.Handler())

	http.HandleFunc("/decide", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, _ := io.ReadAll(r.Body)
		r.Body.Close()

		var req map[string]interface{}
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		requestID, _ := req["request_id"].(string)
		userID, _ := req["user_id"].(string)
		tenantID, _ := req["tenant_id"].(string)
		priority, _ := req["priority"].(string)
		budget, _ := req["budget"].(float64)
		maxLatencyMS := 0
		if ml, ok := req["max_latency_ms"].(float64); ok {
			maxLatencyMS = int(ml)
		}
		maxCostCents := 0.0
		if mc, ok := req["max_cost_cents"].(float64); ok {
			maxCostCents = mc
		}

		decisionReq := decision.Request{
			RequestID:    requestID,
			UserID:       userID,
			TenantID:     tenantID,
			Input:        req["input"],
			Priority:     priority,
			MaxLatencyMS: maxLatencyMS,
			MaxCostCents: maxCostCents,
			Budget:       budget,
		}

		telemetry := decision.Telemetry{
			P99LatencyMS: make(map[decision.Tier]int),
			ErrorRate:    make(map[decision.Tier]float64),
			QueueDepth:   make(map[decision.Tier]int),
		}

		var finalResult map[string]interface{}
		var finalTier decision.Tier
		var finalReason string

		currentTier := decision.Tier0
		var tier0Confidence float64

		for i := 0; i < 3; i++ {
			cb := circuitBreakers[currentTier]
			workerClient := clients[currentTier]

			var result *client.InferResponse
			var err error

			err = cb.Call(func() error {
				var callErr error
				result, callErr = workerClient.Infer(client.InferRequest{
					RequestID: requestID,
					Payload:   req["input"],
				})
				return callErr
			})

			if err != nil {
				if err == circuitbreaker.ErrCircuitOpen {
					if currentTier == decision.Tier0 {
						currentTier = decision.Tier1
						continue
					}
					if currentTier == decision.Tier1 {
						currentTier = decision.Tier2
						continue
					}
				}
				http.Error(w, fmt.Sprintf("tier %s error: %v", currentTier, err), http.StatusInternalServerError)
				return
			}

			if currentTier == decision.Tier0 {
				tier0Confidence = result.Confidence
				dec := engine.Decide(decisionReq, telemetry, tier0Confidence)

				if dec.Tier == decision.Tier0 {
					resultJSON, _ := json.Marshal(result)
					json.Unmarshal(resultJSON, &finalResult)
					finalResult["tier"] = string(decision.Tier0)
					finalResult["reason"] = dec.Reason
					finalResult["estimated_cost_cents"] = dec.EstimatedCost
					finalTier = decision.Tier0
					finalReason = dec.Reason
					break
				}

				nextTier, _ := engine.Escalate(decision.Tier0, tier0Confidence, budget)
				if nextTier != decision.Tier0 {
					escalationsTotal.WithLabelValues(string(decision.Tier0), string(nextTier)).Inc()
					currentTier = nextTier
					continue
				}

				resultJSON, _ := json.Marshal(result)
				json.Unmarshal(resultJSON, &finalResult)
				finalResult["tier"] = string(decision.Tier0)
				finalResult["reason"] = dec.Reason
				finalResult["estimated_cost_cents"] = dec.EstimatedCost
				finalTier = decision.Tier0
				finalReason = dec.Reason
				break
			} else if currentTier == decision.Tier1 {
				if tier0Confidence < 0.75 && budget >= 2.0 {
					resultJSON, _ := json.Marshal(result)
					json.Unmarshal(resultJSON, &finalResult)
					finalResult["tier"] = string(decision.Tier1)
					finalResult["reason"] = "escalated_from_tier0"
					finalResult["estimated_cost_cents"] = 2.0
					finalTier = decision.Tier1
					finalReason = "escalated_from_tier0"
					break
				}

				nextTier, _ := engine.Escalate(decision.Tier1, result.Confidence, budget)
				if nextTier == decision.Tier2 {
					escalationsTotal.WithLabelValues(string(decision.Tier1), string(decision.Tier2)).Inc()
					currentTier = decision.Tier2
					continue
				}

				resultJSON, _ := json.Marshal(result)
				json.Unmarshal(resultJSON, &finalResult)
				finalResult["tier"] = string(decision.Tier1)
				finalResult["reason"] = "escalated_from_tier0"
				finalResult["estimated_cost_cents"] = 2.0
				finalTier = decision.Tier1
				finalReason = "escalated_from_tier0"
				break
			} else {
				resultJSON, _ := json.Marshal(result)
				json.Unmarshal(resultJSON, &finalResult)
				finalResult["tier"] = string(decision.Tier2)
				finalResult["reason"] = "escalated_to_tier2"
				finalResult["estimated_cost_cents"] = 5.0
				finalTier = decision.Tier2
				finalReason = "escalated_to_tier2"
				break
			}
		}

		duration := time.Since(start).Seconds()
		decisionDuration.Observe(duration)
		decisionsTotal.WithLabelValues(string(finalTier), finalReason).Inc()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(finalResult)
	})

	log.Printf("controlplane listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
