package routing

import (
	"context"
	"errors"
	"fmt"

	"dashfetchr/internal/ports"
)

// Strategy is the interface for carrier selection algorithms.
// Implementations live in this package; new ones can be added without
// touching the Engine.
type Strategy interface {
	Name() string
	Select(ctx context.Context, req ports.ShipmentRequest, candidates []Candidate) (Selection, error)
}

type Selection struct {
	Carrier   ports.CarrierPort
	Score     float64
	Reasoning string
}

var ErrNoAvailableCandidate = errors.New("routing: no available candidate")

// ----------------------------------------------------------------------------
// CheapestStrategy — minimizes price.
// ----------------------------------------------------------------------------

type CheapestStrategy struct{}

func (CheapestStrategy) Name() string { return "cheapest" }

func (CheapestStrategy) Select(_ context.Context, _ ports.ShipmentRequest, cs []Candidate) (Selection, error) {
	var best *Candidate
	for i := range cs {
		c := &cs[i]
		if !c.Available {
			continue
		}
		if best == nil || c.Estimate.Cost.Minor < best.Estimate.Cost.Minor {
			best = c
		}
	}
	if best == nil {
		return Selection{}, ErrNoAvailableCandidate
	}
	return Selection{
		Carrier:   best.Carrier,
		Score:     float64(-best.Estimate.Cost.Minor),
		Reasoning: fmt.Sprintf("cheapest at %d %s", best.Estimate.Cost.Minor, best.Estimate.Cost.Currency),
	}, nil
}

// ----------------------------------------------------------------------------
// FastestStrategy — minimizes ETA.
// ----------------------------------------------------------------------------

type FastestStrategy struct{}

func (FastestStrategy) Name() string { return "fastest" }

func (FastestStrategy) Select(_ context.Context, _ ports.ShipmentRequest, cs []Candidate) (Selection, error) {
	var best *Candidate
	for i := range cs {
		c := &cs[i]
		if !c.Available {
			continue
		}
		if best == nil || c.Estimate.EstimatedETA < best.Estimate.EstimatedETA {
			best = c
		}
	}
	if best == nil {
		return Selection{}, ErrNoAvailableCandidate
	}
	return Selection{
		Carrier:   best.Carrier,
		Score:     float64(-best.Estimate.EstimatedETA),
		Reasoning: fmt.Sprintf("fastest ETA %s", best.Estimate.EstimatedETA),
	}, nil
}

// ----------------------------------------------------------------------------
// WeightedScoreStrategy — combines cost + ETA + confidence with weights.
// ----------------------------------------------------------------------------

type WeightedScoreStrategy struct {
	CostWeight       float64 // weight applied to normalized cost (lower better)
	ETAWeight        float64 // weight applied to normalized ETA (lower better)
	ConfidenceWeight float64 // weight applied to confidence (higher better)
}

func (WeightedScoreStrategy) Name() string { return "weighted" }

func (s WeightedScoreStrategy) Select(_ context.Context, _ ports.ShipmentRequest, cs []Candidate) (Selection, error) {
	avail := filterAvailable(cs)
	if len(avail) == 0 {
		return Selection{}, ErrNoAvailableCandidate
	}

	minCost, maxCost := minMaxCost(avail)
	minETA, maxETA := minMaxETA(avail)

	var best *Candidate
	bestScore := -1.0
	bestReason := ""
	for i := range avail {
		c := &avail[i]
		nc := normalize(float64(c.Estimate.Cost.Minor), float64(minCost), float64(maxCost))
		ne := normalize(float64(c.Estimate.EstimatedETA), float64(minETA), float64(maxETA))
		score := -nc*s.CostWeight - ne*s.ETAWeight + c.Estimate.ConfidenceScore*s.ConfidenceWeight
		if best == nil || score > bestScore {
			best = c
			bestScore = score
			bestReason = fmt.Sprintf("cost=%d eta=%s conf=%.2f score=%.2f",
				c.Estimate.Cost.Minor, c.Estimate.EstimatedETA, c.Estimate.ConfidenceScore, score)
		}
	}
	return Selection{
		Carrier:   best.Carrier,
		Score:     bestScore,
		Reasoning: bestReason,
	}, nil
}

func filterAvailable(cs []Candidate) []Candidate {
	out := make([]Candidate, 0, len(cs))
	for _, c := range cs {
		if c.Available {
			out = append(out, c)
		}
	}
	return out
}

func minMaxCost(cs []Candidate) (int64, int64) {
	mn := cs[0].Estimate.Cost.Minor
	mx := mn
	for _, c := range cs[1:] {
		if c.Estimate.Cost.Minor < mn {
			mn = c.Estimate.Cost.Minor
		}
		if c.Estimate.Cost.Minor > mx {
			mx = c.Estimate.Cost.Minor
		}
	}
	return mn, mx
}

func minMaxETA(cs []Candidate) (int64, int64) {
	mn := int64(cs[0].Estimate.EstimatedETA)
	mx := mn
	for _, c := range cs[1:] {
		v := int64(c.Estimate.EstimatedETA)
		if v < mn {
			mn = v
		}
		if v > mx {
			mx = v
		}
	}
	return mn, mx
}

func normalize(v, mn, mx float64) float64 {
	if mx == mn {
		return 0
	}
	return (v - mn) / (mx - mn)
}
