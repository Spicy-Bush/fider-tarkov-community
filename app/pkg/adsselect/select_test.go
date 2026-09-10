package adsselect_test

import (
	"math/rand"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/adsselect"
)

func TestPickWeighted_EmptyOrNilRNG(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	if got := adsselect.PickWeighted(nil, rng); got != nil {
		t.Fatalf("nil candidates: got %+v", got)
	}
	if got := adsselect.PickWeighted([]adsselect.Candidate{}, rng); got != nil {
		t.Fatalf("empty candidates: got %+v", got)
	}
	if got := adsselect.PickWeighted([]adsselect.Candidate{{CampaignID: 1, Weight: 10}}, nil); got != nil {
		t.Fatalf("nil rng: got %+v", got)
	}
}

func TestPickWeighted_SingleAlwaysWins(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	cands := []adsselect.Candidate{{CampaignID: 7, PlacementID: "sidebar_top", CreativeVersionID: 3, Weight: 5, Advertiser: "Acme"}}
	for i := 0; i < 20; i++ {
		got := adsselect.PickWeighted(cands, rng)
		if got == nil || got.CampaignID != 7 {
			t.Fatalf("iter %d: got %+v", i, got)
		}
	}
}

func TestPickWeighted_ZeroWeightReturnsNil(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	cands := []adsselect.Candidate{
		{CampaignID: 1, Weight: 0, Advertiser: "A"},
		{CampaignID: 2, Weight: -3, Advertiser: "B"},
	}
	if got := adsselect.PickWeighted(cands, rng); got != nil {
		t.Fatalf("total weight 0 must return nil, got %+v", got)
	}
}

func TestPickWeighted_ZeroDoesNotFloorToPositive(t *testing.T) {
	// weight 0 must never win against a positive weight
	cands := []adsselect.Candidate{
		{CampaignID: 1, Weight: 0, Advertiser: "A"},
		{CampaignID: 2, Weight: 10, Advertiser: "B"},
	}
	rng := rand.New(rand.NewSource(12345))
	const n = 200
	for i := 0; i < n; i++ {
		got := adsselect.PickWeighted(cands, rng)
		if got == nil || got.CampaignID != 2 {
			t.Fatalf("iter %d: expected campaign 2 only, got %+v", i, got)
		}
	}
}

func TestPickWeighted_DistributionAmongPositive(t *testing.T) {
	cands := []adsselect.Candidate{
		{CampaignID: 1, Weight: 1, Advertiser: "A"},
		{CampaignID: 2, Weight: 99, Advertiser: "B"},
	}
	counts := map[int]int{}
	rng := rand.New(rand.NewSource(12345))
	const n = 1000
	for i := 0; i < n; i++ {
		got := adsselect.PickWeighted(cands, rng)
		if got == nil {
			t.Fatal("unexpected nil")
		}
		counts[got.CampaignID]++
	}
	if counts[2] < 900 {
		t.Fatalf("expected campaign 2 to dominate, counts=%v", counts)
	}
	if counts[1] == 0 {
		t.Fatalf("weight 1 should still give campaign 1 some wins, counts=%v", counts)
	}
}

func TestSelectForInstances_IndependentPicksPerInstance(t *testing.T) {
	cands := map[string][]adsselect.Candidate{
		"feed_native": {
			{CampaignID: 1, PlacementID: "feed_native", CreativeVersionID: 10, Weight: 1, Advertiser: "A"},
			{CampaignID: 2, PlacementID: "feed_native", CreativeVersionID: 20, Weight: 1, Advertiser: "B"},
		},
		"sidebar_top": {
			{CampaignID: 3, PlacementID: "sidebar_top", CreativeVersionID: 30, Weight: 1, Advertiser: "C"},
		},
	}
	req := []adsselect.InstanceReq{
		{InstanceID: "home-feed-1", PlacementID: "feed_native"},
		{InstanceID: "home-feed-2", PlacementID: "feed_native"},
		{InstanceID: "sidebar", PlacementID: "sidebar_top"},
		{InstanceID: "missing", PlacementID: "pages_header"},
	}
	rng := rand.New(rand.NewSource(7))
	got := adsselect.SelectForInstances(req, cands, rng)
	if len(got) != 4 {
		t.Fatalf("len=%d", len(got))
	}
	if got["sidebar"] == nil || got["sidebar"].CampaignID != 3 {
		t.Fatalf("sidebar: %+v", got["sidebar"])
	}
	if got["missing"] != nil {
		t.Fatalf("missing placement should be nil, got %+v", got["missing"])
	}
	if got["home-feed-1"] == nil || got["home-feed-2"] == nil {
		t.Fatalf("feed picks missing: %+v", got)
	}
	if got["home-feed-1"].PlacementID != "feed_native" {
		t.Fatalf("feed-1 placement: %+v", got["home-feed-1"])
	}
}

func TestPickWeighted_CapsTotalWithoutPanic(t *testing.T) {
	cands := make([]adsselect.Candidate, 0, 5)
	for i := 0; i < 5; i++ {
		cands = append(cands, adsselect.Candidate{CampaignID: i + 1, Weight: 2_000_000_000})
	}
	rng := rand.New(rand.NewSource(1))
	got := adsselect.PickWeighted(cands, rng)
	if got == nil {
		t.Fatal("expected a pick from capped positive weights")
	}
}

func TestSelectForInstances_PreservesInstanceKeys(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	got := adsselect.SelectForInstances(
		[]adsselect.InstanceReq{{InstanceID: "a", PlacementID: "x"}},
		map[string][]adsselect.Candidate{},
		rng,
	)
	if _, ok := got["a"]; !ok {
		t.Fatal("instance key a missing")
	}
	if got["a"] != nil {
		t.Fatalf("expected nil candidate, got %+v", got["a"])
	}
}
