package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_requests_total",
			Help: "Total number of requests",
		},
		[]string{"status"},
	)
	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gateway_request_duration_seconds",
			Help:    "Request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"tier"},
	)
)

func init() {
	prometheus.MustRegister(requestsTotal)
	prometheus.MustRegister(requestDuration)
}

var controlplaneURL = os.Getenv("CONTROLPLANE_URL")
var tier0URL = os.Getenv("TIER0_URL")
var tier1URL = os.Getenv("TIER1_URL")
var tier2URL = os.Getenv("TIER2_URL")
var dbURL = os.Getenv("DATABASE_URL")

func main() {
	if controlplaneURL == "" {
		controlplaneURL = "http://controlplane:8081"
	}
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
		port = "8080"
	}

	var db *sql.DB
	if dbURL != "" {
		var err error
		db, err = sql.Open("postgres", dbURL)
		if err != nil {
			log.Printf("failed to connect to db: %v", err)
		} else {
			if err := db.Ping(); err != nil {
				log.Printf("db ping failed: %v", err)
			}
		}
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "gateway"})
	})

	http.Handle("/metrics", promhttp.Handler())

	http.HandleFunc("/infer", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, _ := io.ReadAll(r.Body)
		r.Body.Close()

		var req map[string]interface{}
		if err := json.Unmarshal(body, &req); err != nil {
			requestsTotal.WithLabelValues("bad_request").Inc()
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		requestID, _ := req["request_id"].(string)
		if requestID == "" {
			requestID = fmt.Sprintf("req-%d", time.Now().UnixNano())
		}

		decideReq, _ := json.Marshal(req)
		resp, err := client.Post(controlplaneURL+"/decide", "application/json", bytes.NewBuffer(decideReq))
		if err != nil {
			requestsTotal.WithLabelValues("controlplane_error").Inc()
			http.Error(w, "controlplane error", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			requestsTotal.WithLabelValues("controlplane_error").Inc()
			http.Error(w, "controlplane error", http.StatusInternalServerError)
			return
		}

		var decision map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&decision)

		tier, _ := decision["tier"].(string)
		var workerURL string
		switch tier {
		case "tier0":
			workerURL = tier0URL
		case "tier1":
			workerURL = tier1URL
		case "tier2":
			workerURL = tier2URL
		default:
			requestsTotal.WithLabelValues("unknown_tier").Inc()
			http.Error(w, "unknown tier", http.StatusInternalServerError)
			return
		}

		workerResp, err := client.Post(workerURL+"/infer", "application/json", bytes.NewBuffer(body))
		if err != nil {
			requestsTotal.WithLabelValues("worker_error").Inc()
			http.Error(w, "worker error", http.StatusInternalServerError)
			return
		}
		defer workerResp.Body.Close()

		if workerResp.StatusCode != http.StatusOK {
			requestsTotal.WithLabelValues("worker_error").Inc()
			http.Error(w, "worker error", http.StatusInternalServerError)
			return
		}

		var result map[string]interface{}
		json.NewDecoder(workerResp.Body).Decode(&result)

		duration := time.Since(start).Seconds()
		requestDuration.WithLabelValues(tier).Observe(duration)
		requestsTotal.WithLabelValues("success").Inc()

		if db != nil {
			confidence, _ := result["confidence"].(float64)
			latency, _ := result["model_latency_ms"].(float64)
			budget, _ := req["budget"].(float64)
			db.Exec("INSERT INTO inference_requests (request_id, tier, budget, confidence, latency_ms) VALUES ($1, $2, $3, $4, $5)",
				requestID, tier, budget, confidence, int(latency))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	log.Printf("gateway listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
