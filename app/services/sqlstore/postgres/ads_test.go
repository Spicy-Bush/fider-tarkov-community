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
		ConfigVersion: create.Result.ConfigVersion + 99,
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

func TestAds_SlimCreate_NoAssignments_NotACandidate(t *testing.T) {
	SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	now := time.Now().UTC()
	create := &cmd.CreateSponsorshipCampaign{
		Name: "Slim", Advertiser: "Acme Co",
		StartAt: now.Add(-time.Hour), EndAt: now.Add(24 * time.Hour),
		Weight: 10, Locale: "all", Enabled: true,
	}
	Expect(bus.Dispatch(demoTenantCtx, create)).IsNil()
	Expect(create.Result.ConfigVersion).Equals(1)

	q := &query.GetActiveAdCandidates{
		PlacementIDs: []string{"feed_native", "sidebar_top"},
		Locale:       "all",
		Now:          now,
	}
	Expect(bus.Dispatch(demoTenantCtx, q)).IsNil()
	for _, cand := range q.Result {
		Expect(cand.CampaignID).NotEquals(create.Result.ID)
	}
}

func TestAds_GraphSave_AssignFeedNative_ThenUnassign(t *testing.T) {
	SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	now := time.Now().UTC()
	create := &cmd.CreateSponsorshipCampaign{
		Name: "FeedGraph", Advertiser: "Acme Co",
		StartAt: now.Add(-time.Hour), EndAt: now.Add(24 * time.Hour),
		Weight: 10, Locale: "all", Enabled: true,
	}
	Expect(bus.Dispatch(demoTenantCtx, create)).IsNil()
	ver := &cmd.CreateCreativeVersion{
		CampaignID: create.Result.ID, ImageURL: "https://cdn.example/feed.png",
		ClickURL: "https://example.com/feed", ConfigVersion: create.Result.ConfigVersion,
	}
	Expect(bus.Dispatch(demoTenantCtx, ver)).IsNil()
	Expect(ver.NewConfigVersion).Equals(create.Result.ConfigVersion + 1)

	save := &cmd.SaveSponsorshipCampaignGraph{
		ID: create.Result.ID, Name: create.Result.Name, Advertiser: "Acme Co",
		StartAt: create.Result.StartAt, EndAt: create.Result.EndAt,
		Weight: 10, Locale: "all", Enabled: true,
		ConfigVersion: ver.NewConfigVersion,
		Assignments: []cmd.CampaignAssignmentInput{
			{PlacementID: "feed_native", CreativeVersionID: ver.Result.ID},
		},
	}
	Expect(bus.Dispatch(demoTenantCtx, save)).IsNil()
	Expect(save.Result.ConfigVersion).Equals(ver.NewConfigVersion + 1)
	Expect(save.Result.Clicks).Equals(0)
	Expect(save.Result.CreatedAt.IsZero()).IsFalse()

	q := &query.GetActiveAdCandidates{
		PlacementIDs: []string{"feed_native"},
		Locale:       "all",
		Now:          now,
	}
	Expect(bus.Dispatch(demoTenantCtx, q)).IsNil()
	found := false
	for _, cand := range q.Result {
		if cand.CampaignID == create.Result.ID && cand.PlacementID == "feed_native" {
			found = true
		}
	}
	Expect(found).IsTrue()

	// Unassign = persist remaining set (empty): delete all rows for campaign.
	unassign := &cmd.SaveSponsorshipCampaignGraph{
		ID: create.Result.ID, Name: create.Result.Name, Advertiser: "Acme Co",
		StartAt: create.Result.StartAt, EndAt: create.Result.EndAt,
		Weight: 10, Locale: "all", Enabled: true,
		ConfigVersion: save.Result.ConfigVersion,
		Assignments:   []cmd.CampaignAssignmentInput{},
	}
	Expect(bus.Dispatch(demoTenantCtx, unassign)).IsNil()

	Expect(bus.Dispatch(demoTenantCtx, q)).IsNil()
	for _, cand := range q.Result {
		Expect(cand.CampaignID).NotEquals(create.Result.ID)
	}

	list := &query.ListCampaignAssignmentsByCampaign{CampaignID: create.Result.ID}
	Expect(bus.Dispatch(demoTenantCtx, list)).IsNil()
	Expect(len(list.Result)).Equals(0)
}

func TestAds_GraphSave_StaleConfigVersion_409RowsUnchanged(t *testing.T) {
	SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	now := time.Now().UTC()
	create := &cmd.CreateSponsorshipCampaign{
		Name: "StaleGraph", Advertiser: "Acme Co",
		StartAt: now.Add(-time.Hour), EndAt: now.Add(24 * time.Hour),
		Weight: 10, Locale: "all", Enabled: true,
		Version: &cmd.CreateCampaignVersionInput{
			ImageURL: "https://cdn.example/s.png", ClickURL: "https://example.com/s",
		},
		Assignments: []cmd.CampaignAssignmentInput{
			{PlacementID: "feed_native", CreativeVersionID: 0},
		},
	}
	Expect(bus.Dispatch(demoTenantCtx, create)).IsNil()
	Expect(create.VersionResult).IsNotNil()
	Expect(len(create.AssignmentResults)).Equals(1)
	cfg := create.Result.ConfigVersion

	stale := &cmd.SaveSponsorshipCampaignGraph{
		ID: create.Result.ID, Name: "Hijacked", Advertiser: "Acme Co",
		StartAt: create.Result.StartAt, EndAt: create.Result.EndAt,
		Weight: 99, Locale: "all", Enabled: true,
		ConfigVersion: cfg - 1, // expected <= 0 or mismatch
		Assignments:   []cmd.CampaignAssignmentInput{},
	}
	err := bus.Dispatch(demoTenantCtx, stale)
	Expect(errors.Cause(err)).Equals(app.ErrConflict)

	list := &query.ListCampaignAssignmentsByCampaign{CampaignID: create.Result.ID}
	Expect(bus.Dispatch(demoTenantCtx, list)).IsNil()
	Expect(len(list.Result)).Equals(1)
	Expect(list.Result[0].PlacementID).Equals("feed_native")

	get := &query.GetSponsorshipCampaignByID{ID: create.Result.ID}
	Expect(bus.Dispatch(demoTenantCtx, get)).IsNil()
	Expect(get.Result.Name).Equals("StaleGraph")
	Expect(get.Result.ConfigVersion).Equals(cfg)
}

func TestAds_VersionCreate_ReturnsDBConfigVersion_GraphOCC(t *testing.T) {
	SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	now := time.Now().UTC()
	create := &cmd.CreateSponsorshipCampaign{
		Name: "VerOCC", Advertiser: "Acme Co",
		StartAt: now.Add(-time.Hour), EndAt: now.Add(24 * time.Hour),
		Weight: 10, Locale: "all", Enabled: true,
	}
	Expect(bus.Dispatch(demoTenantCtx, create)).IsNil()

	ver := &cmd.CreateCreativeVersion{
		CampaignID: create.Result.ID, ImageURL: "https://cdn.example/v.png",
		ClickURL: "https://example.com/v", ConfigVersion: create.Result.ConfigVersion,
	}
	Expect(bus.Dispatch(demoTenantCtx, ver)).IsNil()

	get := &query.GetSponsorshipCampaignByID{ID: create.Result.ID}
	Expect(bus.Dispatch(demoTenantCtx, get)).IsNil()
	Expect(ver.NewConfigVersion).Equals(get.Result.ConfigVersion)

	oldToken := &cmd.SaveSponsorshipCampaignGraph{
		ID: create.Result.ID, Name: create.Result.Name, Advertiser: "Acme Co",
		StartAt: create.Result.StartAt, EndAt: create.Result.EndAt,
		Weight: 10, Locale: "all", Enabled: true,
		ConfigVersion: create.Result.ConfigVersion, // stale (pre-version)
		Assignments: []cmd.CampaignAssignmentInput{
			{PlacementID: "sidebar_top", CreativeVersionID: ver.Result.ID},
		},
	}
	Expect(errors.Cause(bus.Dispatch(demoTenantCtx, oldToken))).Equals(app.ErrConflict)

	ok := &cmd.SaveSponsorshipCampaignGraph{
		ID: create.Result.ID, Name: create.Result.Name, Advertiser: "Acme Co",
		StartAt: create.Result.StartAt, EndAt: create.Result.EndAt,
		Weight: 10, Locale: "all", Enabled: true,
		ConfigVersion: ver.NewConfigVersion,
		Assignments: []cmd.CampaignAssignmentInput{
			{PlacementID: "sidebar_top", CreativeVersionID: ver.Result.ID},
		},
	}
	Expect(bus.Dispatch(demoTenantCtx, ok)).IsNil()
	Expect(ok.Result.ConfigVersion).Equals(ver.NewConfigVersion + 1)
}

func TestAds_CreateWithAssignments_AppearsAsCandidate(t *testing.T) {
	SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	now := time.Now().UTC()
	create := &cmd.CreateSponsorshipCampaign{
		Name: "HouseOneShot", Advertiser: "House Adv",
		StartAt: now.Add(-time.Hour), EndAt: now.Add(24 * time.Hour),
		Weight: 50, Locale: "all", Enabled: true,
		Version: &cmd.CreateCampaignVersionInput{
			ImageURL: "https://cdn.example/house.png",
			HTML:     "",
			ClickURL: "https://example.com/house",
		},
		Assignments: []cmd.CampaignAssignmentInput{
			{PlacementID: "feed_native", CreativeVersionID: 0},
		},
	}
	Expect(bus.Dispatch(demoTenantCtx, create)).IsNil()
	Expect(create.Result.ConfigVersion).Equals(1)
	Expect(create.VersionResult).IsNotNil()
	Expect(len(create.AssignmentResults)).Equals(1)

	q := &query.GetActiveAdCandidates{
		PlacementIDs: []string{"feed_native"},
		Locale:       "all",
		Now:          now,
	}
	Expect(bus.Dispatch(demoTenantCtx, q)).IsNil()
	found := false
	for _, cand := range q.Result {
		if cand.CampaignID == create.Result.ID {
			found = true
			Expect(cand.CreativeVersionID).Equals(create.VersionResult.ID)
		}
	}
	Expect(found).IsTrue()
}

func TestAds_ExpectedConfigVersionZero_Conflict(t *testing.T) {
	SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	now := time.Now().UTC()
	create := &cmd.CreateSponsorshipCampaign{
		Name: "ZeroTok", Advertiser: "Acme",
		StartAt: now.Add(-time.Hour), EndAt: now.Add(24 * time.Hour),
		Weight: 10, Locale: "all", Enabled: true,
	}
	Expect(bus.Dispatch(demoTenantCtx, create)).IsNil()

	upd := &cmd.UpdateSponsorshipCampaign{
		ID: create.Result.ID, Name: "ZeroTok2", Advertiser: "Acme",
		StartAt: create.Result.StartAt, EndAt: create.Result.EndAt,
		Weight: 10, Locale: "all", Enabled: true,
		ConfigVersion: 0,
	}
	Expect(errors.Cause(bus.Dispatch(demoTenantCtx, upd))).Equals(app.ErrConflict)

	save := &cmd.SaveSponsorshipCampaignGraph{
		ID: create.Result.ID, Name: create.Result.Name, Advertiser: "Acme",
		StartAt: create.Result.StartAt, EndAt: create.Result.EndAt,
		Weight: 10, Locale: "all", Enabled: true,
		ConfigVersion: 0,
		Assignments:   nil,
	}
	Expect(errors.Cause(bus.Dispatch(demoTenantCtx, save))).Equals(app.ErrConflict)
}
