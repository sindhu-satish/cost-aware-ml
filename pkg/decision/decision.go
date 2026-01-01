package decision

type Tier string

const (
	Tier0 Tier = "tier0"
	Tier1 Tier = "tier1"
	Tier2 Tier = "tier2"
)

type Request struct {
	RequestID    string
	UserID       string
	TenantID     string
	Input        interface{}
	Priority     string
	MaxLatencyMS int
	MaxCostCents float64
	Budget       float64
}

type TierConfig struct {
	Name                Tier
	BaseCostCents       float64
	TimeoutMS           int
	DefaultConfThreshold float64
	Enabled             bool
}

type Telemetry struct {
	P99LatencyMS map[Tier]int
	ErrorRate    map[Tier]float64
	QueueDepth   map[Tier]int
}

type Decision struct {
	Tier            Tier
	Reason          string
	EstimatedCost   float64
	EstimatedLatency int
	ConfidenceThreshold float64
}

type Engine struct {
	tiers map[Tier]TierConfig
}

func NewEngine() *Engine {
	return &Engine{
		tiers: map[Tier]TierConfig{
			Tier0: {Name: Tier0, BaseCostCents: 0.5, TimeoutMS: 50, DefaultConfThreshold: 0.75, Enabled: true},
			Tier1: {Name: Tier1, BaseCostCents: 2.0, TimeoutMS: 200, DefaultConfThreshold: 0.85, Enabled: true},
			Tier2: {Name: Tier2, BaseCostCents: 5.0, TimeoutMS: 500, DefaultConfThreshold: 0.95, Enabled: true},
		},
	}
}

func (e *Engine) Decide(req Request, telemetry Telemetry, tier0Confidence float64) Decision {
	if !e.tiers[Tier0].Enabled {
		return Decision{Tier: Tier1, Reason: "tier0 disabled"}
	}

	budget := req.Budget
	if budget == 0 {
		budget = req.MaxCostCents
	}
	if budget == 0 {
		budget = 10.0
	}

	confThreshold := e.tiers[Tier0].DefaultConfThreshold
	if req.Priority == "premium" {
		confThreshold = 0.70
	}

	if tier0Confidence >= confThreshold {
		return Decision{
			Tier:            Tier0,
			Reason:          "confidence_met",
			EstimatedCost:   e.tiers[Tier0].BaseCostCents,
			EstimatedLatency: e.tiers[Tier0].TimeoutMS,
			ConfidenceThreshold: confThreshold,
		}
	}

	if budget < e.tiers[Tier1].BaseCostCents || !e.tiers[Tier1].Enabled {
		return Decision{
			Tier:            Tier0,
			Reason:          "budget_too_low",
			EstimatedCost:   e.tiers[Tier0].BaseCostCents,
			EstimatedLatency: e.tiers[Tier0].TimeoutMS,
			ConfidenceThreshold: confThreshold,
		}
	}

	if tier0Confidence < confThreshold && budget >= e.tiers[Tier1].BaseCostCents {
		latencyMS := e.tiers[Tier1].TimeoutMS
		if telemetry.P99LatencyMS[Tier1] > 0 {
			latencyMS = telemetry.P99LatencyMS[Tier1]
		}
		if req.MaxLatencyMS > 0 && latencyMS > req.MaxLatencyMS {
			return Decision{
				Tier:            Tier0,
				Reason:          "latency_slo_violation",
				EstimatedCost:   e.tiers[Tier0].BaseCostCents,
				EstimatedLatency: e.tiers[Tier0].TimeoutMS,
				ConfidenceThreshold: confThreshold,
			}
		}

		if telemetry.ErrorRate[Tier1] > 0.1 {
			return Decision{
				Tier:            Tier0,
				Reason:          "tier1_high_error_rate",
				EstimatedCost:   e.tiers[Tier0].BaseCostCents,
				EstimatedLatency: e.tiers[Tier0].TimeoutMS,
				ConfidenceThreshold: confThreshold,
			}
		}

		return Decision{
			Tier:            Tier1,
			Reason:          "escalated_low_confidence",
			EstimatedCost:   e.tiers[Tier1].BaseCostCents,
			EstimatedLatency: latencyMS,
			ConfidenceThreshold: e.tiers[Tier1].DefaultConfThreshold,
		}
	}

	if budget >= e.tiers[Tier2].BaseCostCents && e.tiers[Tier2].Enabled {
		if telemetry.ErrorRate[Tier2] > 0.15 {
			return Decision{
				Tier:            Tier1,
				Reason:          "tier2_high_error_rate",
				EstimatedCost:   e.tiers[Tier1].BaseCostCents,
				EstimatedLatency: e.tiers[Tier1].TimeoutMS,
				ConfidenceThreshold: e.tiers[Tier1].DefaultConfThreshold,
			}
		}

		latencyMS := e.tiers[Tier2].TimeoutMS
		if telemetry.P99LatencyMS[Tier2] > 0 {
			latencyMS = telemetry.P99LatencyMS[Tier2]
		}

		return Decision{
			Tier:            Tier2,
			Reason:          "high_budget",
			EstimatedCost:   e.tiers[Tier2].BaseCostCents,
			EstimatedLatency: latencyMS,
			ConfidenceThreshold: e.tiers[Tier2].DefaultConfThreshold,
		}
	}

	return Decision{
		Tier:            Tier0,
		Reason:          "default",
		EstimatedCost:   e.tiers[Tier0].BaseCostCents,
		EstimatedLatency: e.tiers[Tier0].TimeoutMS,
		ConfidenceThreshold: confThreshold,
	}
}

func (e *Engine) Escalate(currentTier Tier, confidence float64, budget float64) (Tier, string) {
	if currentTier == Tier0 && confidence < 0.75 && budget >= 2.0 {
		return Tier1, "escalate_to_tier1"
	}
	if currentTier == Tier1 && confidence < 0.85 && budget >= 5.0 {
		return Tier2, "escalate_to_tier2"
	}
	return currentTier, "no_escalation"
}

