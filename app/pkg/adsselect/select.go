package adsselect

import "math/rand"

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

// PickWeighted chooses one candidate by weight. rng must be non-nil (injected).
// Weight <= 0 contributes nothing. If total weight is 0, returns nil (no pick).
// Positive weights are capped at maxWeightContribution.
func PickWeighted(candidates []Candidate, rng *rand.Rand) *Candidate {
	if len(candidates) == 0 || rng == nil {
		return nil
	}
	total := 0
	weights := make([]int, len(candidates))
	for i := range candidates {
		w := candidates[i].Weight
		if w <= 0 {
			weights[i] = 0
			continue
		}
		if w > maxWeightContribution {
			w = maxWeightContribution
		}
		weights[i] = w
		total += w
	}
	if total <= 0 {
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
	// Defensive: floating rounding should not reach here when total > 0.
	for i := len(candidates) - 1; i >= 0; i-- {
		if weights[i] > 0 {
			c := candidates[i]
			return &c
		}
	}
	return nil
}

// SelectForInstances runs an independent weighted pick per instance after SQL
// produced candidatesByPlacement. Schedule filtering is already applied in SQL.
func SelectForInstances(
	req []InstanceReq,
	candidatesByPlacement map[string][]Candidate,
	rng *rand.Rand,
) map[string]*Candidate {
	out := make(map[string]*Candidate, len(req))
	for _, item := range req {
		cands := candidatesByPlacement[item.PlacementID]
		out[item.InstanceID] = PickWeighted(cands, rng)
	}
	return out
}
