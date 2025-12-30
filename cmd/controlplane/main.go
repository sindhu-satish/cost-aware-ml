package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	decisionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "controlplane_decisions_total",
			Help: "Total number of tier decisions",
		},
		[]string{"tier"},
	)
	decisionDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "controlplane_decision_duration_seconds",
			Help:    "Decision duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)
)

func init() {
	prometheus.MustRegister(decisionsTotal)
	prometheus.MustRegister(decisionDuration)
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

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

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
		json.Unmarshal(body, &req)

		tier := "tier0"
		if budget, ok := req["budget"].(float64); ok && budget > 5 {
			tier = "tier1"
		}
		if budget, ok := req["budget"].(float64); ok && budget > 10 {
			tier = "tier2"
		}

		workerURL := tier0URL
		if tier == "tier1" {
			workerURL = tier1URL
		}
		if tier == "tier2" {
			workerURL = tier2URL
		}

		workerResp, err := client.Post(workerURL+"/infer", "application/json", bytes.NewBuffer(body))
		if err != nil {
			decisionsTotal.WithLabelValues("error").Inc()
			http.Error(w, "worker error", http.StatusInternalServerError)
			return
		}
		defer workerResp.Body.Close()

		if workerResp.StatusCode != http.StatusOK {
			decisionsTotal.WithLabelValues("error").Inc()
			http.Error(w, "worker error", http.StatusInternalServerError)
			return
		}

		var result map[string]interface{}
		json.NewDecoder(workerResp.Body).Decode(&result)
		result["tier"] = tier

		duration := time.Since(start).Seconds()
		decisionDuration.Observe(duration)
		decisionsTotal.WithLabelValues(tier).Inc()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	log.Printf("controlplane listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
