package adsselect

import (
	"math"
	"math/rand"
)

// Candidate is a schedule-filtered campaign assignment row ready for weighted pick.
type Candidate struct {
	CampaignID        int
	PlacementID       string
	CreativeVersionID int
	Weight            int
	Advertiser        string
}

// InstanceReq is one page-owned selection request item.
type InstanceReq struct {
	InstanceID  string
	PlacementID string
}

// maxWeightContribution caps a single candidate's contribution to the pick total
// so Intn cannot overflow on pathological admin input.
const maxWeightContribution = 1_000_000

// maxTotalWeight caps the summed weight used for Intn.
const maxTotalWeight = 1_000_000_000

type preparedPlacement struct {
	cands   []Candidate
	weights []int
	total   int
}

func prepareWeights(candidates []Candidate) (weights []int, total int) {
	weights = make([]int, len(candidates))
	for i := range candidates {
		w := candidates[i].Weight
		if w <= 0 {
			continue
		}
		if w > maxWeightContribution {
			w = maxWeightContribution
		}
		if total >= maxTotalWeight {
			break
		}
		if w > maxTotalWeight-total {
			w = maxTotalWeight - total
		}
		if total > math.MaxInt-w {
			w = math.MaxInt - total
		}
		if w <= 0 {
			break
		}
		weights[i] = w
		total += w
	}
	return weights, total
}

func pickPrepared(candidates []Candidate, weights []int, total int, rng *rand.Rand) *Candidate {
	if len(candidates) == 0 || rng == nil || total <= 0 {
		return nil
	}
	pick := rng.Intn(total)
	running := 0
	for i := range candidates {
		running += weights[i]
		if weights[i] > 0 && pick < running {
			c := candidates[i]
			return &c
		}
	}
	for i := len(candidates) - 1; i >= 0; i-- {
		if weights[i] > 0 {
			c := candidates[i]
			return &c
		}
	}
	return nil
}

// PickWeighted chooses one candidate by weight. rng must be non-nil (injected).
// Weight <= 0 contributes nothing. If total weight is 0, returns nil (no pick).
// Positive weights are capped at maxWeightContribution; the sum at maxTotalWeight.
func PickWeighted(candidates []Candidate, rng *rand.Rand) *Candidate {
	weights, total := prepareWeights(candidates)
	return pickPrepared(candidates, weights, total, rng)
}

// SelectForInstances runs an independent weighted pick per instance after SQL
// produced candidatesByPlacement. Weight totals are computed once per placement.
func SelectForInstances(
	req []InstanceReq,
	candidatesByPlacement map[string][]Candidate,
	rng *rand.Rand,
) map[string]*Candidate {
	prepared := make(map[string]preparedPlacement, len(candidatesByPlacement))
	for pid, cands := range candidatesByPlacement {
		weights, total := prepareWeights(cands)
		prepared[pid] = preparedPlacement{cands: cands, weights: weights, total: total}
	}
	out := make(map[string]*Candidate, len(req))
	for _, item := range req {
		p := prepared[item.PlacementID]
		out[item.InstanceID] = pickPrepared(p.cands, p.weights, p.total, rng)
	}
	return out
}
