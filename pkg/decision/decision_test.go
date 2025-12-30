package decision

import "testing"

func TestDecisionEngine(t *testing.T) {
	engine := NewEngine()

	tests := []struct {
		name     string
		req      Request
		conf     float64
		expected Tier
	}{
		{
			name: "high confidence tier0",
			req:  Request{Budget: 10.0, Priority: "normal"},
			conf: 0.80,
			expected: Tier0,
		},
		{
			name: "low confidence escalate",
			req:  Request{Budget: 10.0, Priority: "normal"},
			conf: 0.60,
			expected: Tier1,
		},
		{
			name: "budget too low",
			req:  Request{Budget: 0.3, Priority: "normal"},
			conf: 0.60,
			expected: Tier0,
		},
		{
			name: "premium user lower threshold",
			req:  Request{Budget: 10.0, Priority: "premium"},
			conf: 0.75,
			expected: Tier0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := engine.Decide(tt.req, Telemetry{}, tt.conf)
			if decision.Tier != tt.expected {
				t.Errorf("expected tier %v, got %v", tt.expected, decision.Tier)
			}
		})
	}
}

