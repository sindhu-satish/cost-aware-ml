package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type Request struct {
	RequestID string  `json:"request_id"`
	UserID    string  `json:"user_id"`
	TenantID  string  `json:"tenant_id"`
	Input     string  `json:"input"`
	Budget    float64 `json:"budget"`
	Priority  string  `json:"priority,omitempty"`
}

func main() {
	gatewayURL := "http://localhost:8080"
	concurrency := 10
	duration := 60 * time.Second
	rps := 50

	client := &http.Client{Timeout: 30 * time.Second}
	var wg sync.WaitGroup
	stop := make(chan bool)
	start := time.Now()

	var successCount, errorCount int64
	var mu sync.Mutex

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ticker := time.NewTicker(time.Second / time.Duration(rps/concurrency))
			defer ticker.Stop()

			for {
				select {
				case <-stop:
					return
				case <-ticker.C:
					req := Request{
						RequestID: fmt.Sprintf("req-%d-%d", id, time.Now().UnixNano()),
						UserID:    fmt.Sprintf("user-%d", id%100),
						TenantID:  "tenant-1",
						Input:     "test input",
						Budget:    float64((id % 3) * 5),
						Priority:  "normal",
					}
					if id%10 == 0 {
						req.Priority = "premium"
						req.Budget = 15.0
					}

					body, _ := json.Marshal(req)
					resp, err := client.Post(gatewayURL+"/infer", "application/json", bytes.NewBuffer(body))
					if err != nil {
						mu.Lock()
						errorCount++
						mu.Unlock()
						continue
					}
					resp.Body.Close()

					mu.Lock()
					if resp.StatusCode == 200 {
						successCount++
					} else {
						errorCount++
					}
					mu.Unlock()
				}
			}
		}(i)
	}

	time.Sleep(duration)
	close(stop)
	wg.Wait()

	elapsed := time.Since(start)
	log.Printf("Load test complete:")
	log.Printf("  Duration: %v", elapsed)
	log.Printf("  Success: %d", successCount)
	log.Printf("  Errors: %d", errorCount)
	log.Printf("  RPS: %.2f", float64(successCount)/elapsed.Seconds())
}

