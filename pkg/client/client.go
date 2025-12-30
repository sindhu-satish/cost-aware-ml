package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type WorkerClient struct {
	baseURL string
	client  *http.Client
}

func New(baseURL string) *WorkerClient {
	return &WorkerClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type InferRequest struct {
	RequestID string      `json:"request_id"`
	Payload   interface{} `json:"payload"`
}

type InferResponse struct {
	Result         string  `json:"result"`
	Confidence     float64 `json:"confidence"`
	ModelLatencyMS int     `json:"model_latency_ms"`
}

func (c *WorkerClient) Infer(req InferRequest) (*InferResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Post(c.baseURL+"/infer", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("worker error: %s", string(body))
	}

	var result InferResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

