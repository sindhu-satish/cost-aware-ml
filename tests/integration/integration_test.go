package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

const (
	gatewayURL = "http://localhost:8080"
)

func TestHealthChecks(t *testing.T) {
	services := []struct {
		name string
		url  string
	}{
		{"gateway", gatewayURL + "/healthz"},
		{"controlplane", "http://localhost:8081/healthz"},
		{"tier0", "http://localhost:8090/healthz"},
		{"tier1", "http://localhost:8091/healthz"},
		{"tier2", "http://localhost:8092/healthz"},
	}

	client := &http.Client{Timeout: 5 * time.Second}

	for _, svc := range services {
		t.Run(svc.name, func(t *testing.T) {
			resp, err := client.Get(svc.url)
			if err != nil {
				t.Fatalf("failed to connect to %s: %v", svc.name, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("expected 200, got %d", resp.StatusCode)
			}
		})
	}
}

func TestInferenceFlow(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	tests := []struct {
		name   string
		budget float64
		expect string
	}{
		{"low_budget_tier0", 0.5, "tier0"},
		{"medium_budget_tier1", 3.0, "tier1"},
		{"high_budget_tier2", 15.0, "tier2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := map[string]interface{}{
				"request_id": fmt.Sprintf("test-%d", time.Now().UnixNano()),
				"user_id":    "test-user",
				"tenant_id":  "tenant-1",
				"input":      "test input",
				"budget":     tt.budget,
			}

			body, _ := json.Marshal(req)
			resp, err := client.Post(gatewayURL+"/infer", "application/json", bytes.NewBuffer(body))
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected 200, got %d", resp.StatusCode)
			}

			var result map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			tier, ok := result["tier"].(string)
			if !ok {
				t.Fatal("tier not found in response")
			}

			if tier != tt.expect {
				t.Errorf("expected tier %s, got %s", tt.expect, tier)
			}

			confidence, ok := result["confidence"].(float64)
			if !ok {
				t.Fatal("confidence not found in response")
			}

			if confidence < 0 || confidence > 1 {
				t.Errorf("confidence should be between 0 and 1, got %f", confidence)
			}
		})
	}
}

func TestEscalationFlow(t *testing.T) {
	client := &http.Client{Timeout: 30 * time.Second}

	req := map[string]interface{}{
		"request_id": fmt.Sprintf("test-escalation-%d", time.Now().UnixNano()),
		"user_id":    "test-user",
		"tenant_id":  "tenant-1",
		"input":      "test input",
		"budget":     10.0,
		"priority":   "normal",
	}

	body, _ := json.Marshal(req)
	resp, err := client.Post(gatewayURL+"/infer", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	tier, ok := result["tier"].(string)
	if !ok {
		t.Fatal("tier not found in response")
	}

	if tier == "" {
		t.Error("tier should not be empty")
	}

	reason, ok := result["reason"].(string)
	if !ok {
		t.Fatal("reason not found in response")
	}

	if reason == "" {
		t.Error("reason should not be empty")
	}
}

