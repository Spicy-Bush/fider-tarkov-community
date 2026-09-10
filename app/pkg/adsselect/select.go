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

// PickWeighted chooses one candidate by weight. rng must be non-nil (injected).
// Weight floor is 1. Returns nil if candidates is empty.
func PickWeighted(candidates []Candidate, rng *rand.Rand) *Candidate {
	if len(candidates) == 0 || rng == nil {
		return nil
	}
	total := 0
	for i := range candidates {
		w := candidates[i].Weight
		if w < 1 {
			w = 1
		}
		total += w
	}
	pick := rng.Intn(total)
	running := 0
	for i := range candidates {
		w := candidates[i].Weight
		if w < 1 {
			w = 1
		}
		running += w
		if pick < running {
			c := candidates[i]
			return &c
		}
	}
	c := candidates[len(candidates)-1]
	return &c
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
