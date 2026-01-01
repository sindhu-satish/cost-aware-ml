package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/cost-aware-ml/pkg/cache"
	"github.com/cost-aware-ml/pkg/observability"
	"github.com/cost-aware-ml/pkg/ratelimit"
	"github.com/cost-aware-ml/pkg/retry"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
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
	cacheHits = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_cache_hits_total",
			Help: "Total cache hits",
		},
		[]string{"tier"},
	)
	cacheMisses = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_cache_misses_total",
			Help: "Total cache misses",
		},
		[]string{"tier"},
	)
	rateLimitRejected = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "gateway_rate_limit_rejected_total",
			Help: "Total requests rejected by rate limiter",
		},
	)
	queueDepth = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "gateway_queue_depth",
			Help: "Current queue depth",
		},
	)
)

func init() {
	prometheus.MustRegister(requestsTotal)
	prometheus.MustRegister(requestDuration)
	prometheus.MustRegister(cacheHits)
	prometheus.MustRegister(cacheMisses)
	prometheus.MustRegister(rateLimitRejected)
	prometheus.MustRegister(queueDepth)
}

var controlplaneURL = os.Getenv("CONTROLPLANE_URL")
var tier0URL = os.Getenv("TIER0_URL")
var tier1URL = os.Getenv("TIER1_URL")
var tier2URL = os.Getenv("TIER2_URL")
var dbURL = os.Getenv("DATABASE_URL")
var redisURL = os.Getenv("REDIS_URL")

const (
	maxQueueSize = 1000
	queueTimeout = 5 * time.Second
)

type RequestQueue struct {
	queue chan *QueuedRequest
	mu    sync.Mutex
}

type QueuedRequest struct {
	req     *http.Request
	resp    http.ResponseWriter
	done    chan bool
	body    []byte
	request map[string]interface{}
}

func NewRequestQueue(size int) *RequestQueue {
	return &RequestQueue{
		queue: make(chan *QueuedRequest, size),
	}
}

func (q *RequestQueue) Enqueue(req *QueuedRequest) bool {
	select {
	case q.queue <- req:
		queueDepth.Inc()
		return true
	default:
		return false
	}
}

func (q *RequestQueue) Dequeue() *QueuedRequest {
	req := <-q.queue
	queueDepth.Dec()
	return req
}

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

	var redisClient *redis.Client
	var rateLimiter *ratelimit.RateLimiter
	var responseCache *cache.Cache

	if redisURL != "" {
		opt, err := redis.ParseURL(redisURL)
		if err != nil {
			log.Printf("failed to parse redis URL: %v", err)
		} else {
			redisClient = redis.NewClient(opt)
			if err := redisClient.Ping(context.Background()).Err(); err != nil {
				log.Printf("redis ping failed: %v", err)
			} else {
				rateLimiter = ratelimit.New(redisClient)
				responseCache = cache.New(redisClient, 5*time.Minute)
			}
		}
	}

	shutdown := observability.Init("gateway")
	defer shutdown()

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	requestQueue := NewRequestQueue(maxQueueSize)

	go func() {
		for {
			queuedReq := requestQueue.Dequeue()
			handleInference(queuedReq.req.Context(), queuedReq.resp, queuedReq.body, queuedReq.request, client, db, rateLimiter, responseCache, controlplaneURL, tier0URL, tier1URL, tier2URL)
			close(queuedReq.done)
		}
	}()

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "gateway"})
	})

	http.Handle("/metrics", promhttp.Handler())

	http.HandleFunc("/infer", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(r.Header))
		ctx, span := observability.Tracer.Start(ctx, "gateway.infer")
		defer span.End()

		body, _ := io.ReadAll(r.Body)
		r.Body.Close()

		var req map[string]interface{}
		if err := json.Unmarshal(body, &req); err != nil {
			requestsTotal.WithLabelValues("bad_request").Inc()
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		tenantID, _ := req["tenant_id"].(string)
		if tenantID == "" {
			tenantID = "default"
		}

		if rateLimiter != nil {
			allowed, err := rateLimiter.TokenBucket(ctx, "ratelimit:"+tenantID, 100, 10.0)
			if err != nil {
				log.Printf("rate limit error: %v", err)
			} else if !allowed {
				rateLimitRejected.Inc()
				requestsTotal.WithLabelValues("rate_limited").Inc()
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
		}

		queuedReq := &QueuedRequest{
			req:  r.WithContext(ctx),
			resp: w,
			done: make(chan bool),
			body: body,
			request: req,
		}

		if !requestQueue.Enqueue(queuedReq) {
			requestsTotal.WithLabelValues("queue_full").Inc()
			http.Error(w, "service overloaded", http.StatusServiceUnavailable)
			return
		}

		select {
		case <-queuedReq.done:
		case <-time.After(queueTimeout):
			requestsTotal.WithLabelValues("queue_timeout").Inc()
			http.Error(w, "request timeout", http.StatusRequestTimeout)
		}
	})

	log.Printf("gateway listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleInference(ctx context.Context, w http.ResponseWriter, body []byte, req map[string]interface{}, client *http.Client, db *sql.DB, rateLimiter *ratelimit.RateLimiter, responseCache *cache.Cache, controlplaneURL, tier0URL, tier1URL, tier2URL string) {
	start := time.Now()

	requestID, _ := req["request_id"].(string)
	if requestID == "" {
		requestID = fmt.Sprintf("req-%d", time.Now().UnixNano())
	}

	tenantID, _ := req["tenant_id"].(string)
	if tenantID == "" {
		tenantID = "default"
	}

	var cachedResult []byte
	var cacheHit bool
	if responseCache != nil {
		cacheKey, err := responseCache.Key(tenantID, req["input"])
		if err == nil {
			cached, err := responseCache.Get(ctx, cacheKey)
			if err == nil {
				cachedResult = cached
				cacheHit = true
			}
		}
	}

	if cacheHit {
		var result map[string]interface{}
		if err := json.Unmarshal(cachedResult, &result); err == nil {
			tier, _ := result["tier"].(string)
			cacheHits.WithLabelValues(tier).Inc()
			requestsTotal.WithLabelValues("success").Inc()
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache", "HIT")
			json.NewEncoder(w).Encode(result)
			return
		}
	}

	decideReq, _ := json.Marshal(req)
	
	var decision map[string]interface{}
	var err error

	retryConfig := retry.DefaultConfig()
	err = retry.Retry(retryConfig, func() error {
		httpReq, callErr := http.NewRequestWithContext(ctx, "POST", controlplaneURL+"/decide", bytes.NewBuffer(decideReq))
		if callErr != nil {
			return callErr
		}
		otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(httpReq.Header))

		resp, callErr := client.Do(httpReq)
		if callErr != nil {
			return callErr
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("controlplane returned %d", resp.StatusCode)
		}

		callErr = json.NewDecoder(resp.Body).Decode(&decision)
		return callErr
	})

	if err != nil {
		requestsTotal.WithLabelValues("controlplane_error").Inc()
		http.Error(w, "controlplane error", http.StatusInternalServerError)
		return
	}

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

	var result map[string]interface{}
	
	err = retry.Retry(retryConfig, func() error {
		httpReq, callErr := http.NewRequestWithContext(ctx, "POST", workerURL+"/infer", bytes.NewBuffer(body))
		if callErr != nil {
			return callErr
		}
		otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(httpReq.Header))

		workerResp, callErr := client.Do(httpReq)
		if callErr != nil {
			return callErr
		}
		defer workerResp.Body.Close()

		if workerResp.StatusCode != http.StatusOK {
			return fmt.Errorf("worker returned %d", workerResp.StatusCode)
		}

		callErr = json.NewDecoder(workerResp.Body).Decode(&result)
		return callErr
	})

	if err != nil {
		requestsTotal.WithLabelValues("worker_error").Inc()
		http.Error(w, "worker error", http.StatusInternalServerError)
		return
	}

	if responseCache != nil && !cacheHit {
		cacheKey, err := responseCache.Key(tenantID, req["input"])
		if err == nil {
			resultJSON, _ := json.Marshal(result)
			responseCache.Set(ctx, cacheKey, resultJSON)
		}
		cacheMisses.WithLabelValues(tier).Inc()
	}

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

	traceID := trace.SpanFromContext(ctx).SpanContext().TraceID().String()
	result["trace_id"] = traceID

	w.Header().Set("Content-Type", "application/json")
	if cacheHit {
		w.Header().Set("X-Cache", "HIT")
	} else {
		w.Header().Set("X-Cache", "MISS")
	}
	json.NewEncoder(w).Encode(result)
}
