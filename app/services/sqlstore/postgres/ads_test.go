package postgres_test

import (
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	. "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/assert"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
)

func TestAds_UpsertAssignment_RejectsCrossCampaignVersion(t *testing.T) {
	SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	now := time.Now().UTC()
	c1 := &cmd.CreateSponsorshipCampaign{
		Name: "Camp A", Advertiser: "Acme",
		StartAt: now.Add(-time.Hour), EndAt: now.Add(24 * time.Hour),
		Weight: 10, Locale: "all", Enabled: true,
	}
	c2 := &cmd.CreateSponsorshipCampaign{
		Name: "Camp B", Advertiser: "Beta",
		StartAt: now.Add(-time.Hour), EndAt: now.Add(24 * time.Hour),
		Weight: 10, Locale: "all", Enabled: true,
	}
	Expect(bus.Dispatch(demoTenantCtx, c1)).IsNil()
	Expect(bus.Dispatch(demoTenantCtx, c2)).IsNil()

	ver := &cmd.CreateCreativeVersion{
		CampaignID: c1.Result.ID, ImageURL: "https://cdn.example/a.png",
		HTML: "", ClickURL: "https://example.com/a", ConfigVersion: c1.Result.ConfigVersion,
	}
	Expect(bus.Dispatch(demoTenantCtx, ver)).IsNil()

	// Graph save on campaign B with A's version must fail (ownership).
	save := &cmd.SaveSponsorshipCampaignGraph{
		ID: c2.Result.ID, Name: c2.Result.Name, Advertiser: c2.Result.Advertiser,
		StartAt: c2.Result.StartAt, EndAt: c2.Result.EndAt,
		Weight: c2.Result.Weight, Locale: c2.Result.Locale, Enabled: true,
		ConfigVersion: c2.Result.ConfigVersion,
		Assignments: []cmd.CampaignAssignmentInput{
			{PlacementID: "sidebar_top", CreativeVersionID: ver.Result.ID},
		},
	}
	err := bus.Dispatch(demoTenantCtx, save)
	Expect(err).IsNotNil()
	Expect(errors.Cause(err)).Equals(app.ErrNotFound)
}

func TestAds_UpdateCampaign_OCCConflict(t *testing.T) {
	SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	now := time.Now().UTC()
	create := &cmd.CreateSponsorshipCampaign{
		Name: "OCC", Advertiser: "Acme",
		StartAt: now.Add(-time.Hour), EndAt: now.Add(24 * time.Hour),
		Weight: 10, Locale: "all", Enabled: true,
	}
	Expect(bus.Dispatch(demoTenantCtx, create)).IsNil()

	upd := &cmd.UpdateSponsorshipCampaign{
		ID: create.Result.ID, Name: "OCC2", Advertiser: "Acme",
		StartAt: create.Result.StartAt, EndAt: create.Result.EndAt,
		Weight: 10, Locale: "all", Enabled: true,
		ConfigVersion: create.Result.ConfigVersion + 99, // stale
	}
	err := bus.Dispatch(demoTenantCtx, upd)
	Expect(errors.Cause(err)).Equals(app.ErrConflict)
}

func TestAds_UpdateCampaign_ReturnsPersistedClicksCreatedAt(t *testing.T) {
	SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	now := time.Now().UTC()
	create := &cmd.CreateSponsorshipCampaign{
		Name: "Clicks", Advertiser: "Acme",
		StartAt: now.Add(-time.Hour), EndAt: now.Add(24 * time.Hour),
		Weight: 10, Locale: "all", Enabled: true,
	}
	Expect(bus.Dispatch(demoTenantCtx, create)).IsNil()
	Expect(bus.Dispatch(demoTenantCtx, &cmd.IncrementSponsorshipClick{ID: create.Result.ID})).IsNil()

	upd := &cmd.UpdateSponsorshipCampaign{
		ID: create.Result.ID, Name: "Clicks2", Advertiser: "Acme",
		StartAt: create.Result.StartAt, EndAt: create.Result.EndAt,
		Weight: 10, Locale: "all", Enabled: true,
		ConfigVersion: create.Result.ConfigVersion,
	}
	Expect(bus.Dispatch(demoTenantCtx, upd)).IsNil()
	Expect(upd.Result.Clicks).Equals(1)
	Expect(upd.Result.CreatedAt.IsZero()).IsFalse()
	Expect(upd.Result.ConfigVersion).Equals(create.Result.ConfigVersion + 1)
}

func TestAds_GetActiveAdCandidates_ReturnsAssignedCampaign(t *testing.T) {
	SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	now := time.Now().UTC()
	create := &cmd.CreateSponsorshipCampaign{
		Name: "ActiveAdv", Advertiser: "Acme Co",
		StartAt: now.Add(-time.Hour), EndAt: now.Add(24 * time.Hour),
		Weight: 10, Locale: "all", Enabled: true,
	}
	Expect(bus.Dispatch(demoTenantCtx, create)).IsNil()
	ver := &cmd.CreateCreativeVersion{
		CampaignID: create.Result.ID, ImageURL: "https://cdn.example/b.png",
		ClickURL: "https://example.com/b", ConfigVersion: create.Result.ConfigVersion,
	}
	Expect(bus.Dispatch(demoTenantCtx, ver)).IsNil()
	save := &cmd.SaveSponsorshipCampaignGraph{
		ID: create.Result.ID, Name: create.Result.Name, Advertiser: "Acme Co",
		StartAt: create.Result.StartAt, EndAt: create.Result.EndAt,
		Weight: 10, Locale: "all", Enabled: true,
		ConfigVersion: create.Result.ConfigVersion + 1, // bumped by version create
		Assignments: []cmd.CampaignAssignmentInput{
			{PlacementID: "sidebar_top", CreativeVersionID: ver.Result.ID},
		},
	}
	Expect(bus.Dispatch(demoTenantCtx, save)).IsNil()

	q := &query.GetActiveAdCandidates{
		PlacementIDs: []string{"sidebar_top"},
		Locale:       "all",
		Now:          now,
	}
	Expect(bus.Dispatch(demoTenantCtx, q)).IsNil()
	Expect(len(q.Result) >= 1).IsTrue()
	Expect(q.Result[0].Advertiser).Equals("Acme Co")
	// Blank-advertiser exclusion is covered by CHECK + btrim filter + handler TestSelectAds_EmptyAdvertiserMapsInstanceToNull.
}
