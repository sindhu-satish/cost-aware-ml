package events

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/nats-io/nats.go"
)

type EventPublisher struct {
	nc *nats.Conn
}

type DecisionEvent struct {
	EventType      string    `json:"event_type"`
	RequestID      string    `json:"request_id"`
	UserID         string    `json:"user_id"`
	TenantID       string    `json:"tenant_id"`
	Tier           string    `json:"tier"`
	Reason         string    `json:"reason"`
	Budget         float64   `json:"budget"`
	EstimatedCost  float64   `json:"estimated_cost_cents"`
	Confidence     float64   `json:"confidence,omitempty"`
	LatencyMS      int       `json:"latency_ms,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
}

func NewPublisher(natsURL string) (*EventPublisher, error) {
	nc, err := nats.Connect(natsURL)
	if err != nil {
		return nil, err
	}

	return &EventPublisher{nc: nc}, nil
}

func (p *EventPublisher) PublishDecision(ctx context.Context, event DecisionEvent) error {
	event.EventType = "decision"
	event.Timestamp = time.Now()

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	subject := "inference.decisions"
	if event.TenantID != "" {
		subject = "inference.decisions." + event.TenantID
	}

	err = p.nc.Publish(subject, data)
	if err != nil {
		log.Printf("failed to publish event: %v", err)
		return err
	}

	return nil
}

func (p *EventPublisher) Close() {
	if p.nc != nil {
		p.nc.Close()
	}
}

