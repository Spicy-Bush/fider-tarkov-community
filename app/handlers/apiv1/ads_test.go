package apiv1_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/assets"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/apiv1"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/adsselect"
	. "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/assert"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/mock"
)

func init() {
	if assets.FS == nil {
		assets.FS = os.DirFS(env.Path("."))
	}
}

func stubPlacements() {
	bus.AddHandler(func(ctx context.Context, q *query.ListAdPlacements) error {
		q.Result = []*entity.AdPlacement{
			{ID: "feed_native", Name: "Feed", Enabled: true},
			{ID: "sidebar_top", Name: "Sidebar", Enabled: true},
		}
		return nil
	})
}

func TestSelectAds_EmptyAdvertiserMapsInstanceToNull(t *testing.T) {
	RegisterT(t)
	stubPlacements()

	bus.AddHandler(func(ctx context.Context, q *query.GetActiveAdCandidates) error {
		q.Result = []adsselect.Candidate{{
			CampaignID: 1, PlacementID: "sidebar_top", CreativeVersionID: 9,
			Weight: 10, Advertiser: "   ",
		}}
		return nil
	})
	bus.AddHandler(func(ctx context.Context, q *query.GetCreativeVersionsByIDs) error {
		q.Result = map[int]*entity.CreativeVersion{
			9: {ID: 9, CampaignID: 1, ImageURL: "https://x/a.png", ClickURL: "https://x"},
		}
		return nil
	})

	server := mock.NewServer()
	status, resp := server.
		OnTenant(mock.DemoTenant).
		WithURL("http://demo.test.fider.io/api/v1/ads/select?locale=en").
		ExecutePost(
			apiv1.SelectAds(),
			`{"slots":[{"instanceId":"sidebar","placementId":"sidebar_top"}]}`,
		)
	Expect(status).Equals(http.StatusOK)
	Expect(resp.Body.String()).ContainsSubstring(`"sidebar":null`)
}

func TestSelectAds_RejectsUnknownPlacement(t *testing.T) {
	RegisterT(t)
	stubPlacements()

	server := mock.NewServer()
	status, _ := server.
		OnTenant(mock.DemoTenant).
		ExecutePost(
			apiv1.SelectAds(),
			`{"slots":[{"instanceId":"a","placementId":"not_a_real_slot"}]}`,
		)
	Expect(status).Equals(http.StatusBadRequest)
}

func TestSelectAds_RejectsDuplicateInstanceIDs(t *testing.T) {
	RegisterT(t)
	stubPlacements()

	server := mock.NewServer()
	status, _ := server.
		OnTenant(mock.DemoTenant).
		ExecutePost(
			apiv1.SelectAds(),
			`{"slots":[
				{"instanceId":"dup","placementId":"sidebar_top"},
				{"instanceId":"dup","placementId":"feed_native"}
			]}`,
		)
	Expect(status).Equals(http.StatusBadRequest)
}

func TestSelectAds_RejectsOversizedBatch(t *testing.T) {
	RegisterT(t)
	stubPlacements()

	slots := make([]map[string]string, apiv1.MaxSelectAdsSlots+1)
	for i := range slots {
		slots[i] = map[string]string{
			"instanceId":  fmt.Sprintf("inst-%d", i),
			"placementId": "sidebar_top",
		}
	}
	payload, _ := json.Marshal(map[string]any{"slots": slots})

	server := mock.NewServer()
	status, _ := server.
		OnTenant(mock.DemoTenant).
		ExecutePost(apiv1.SelectAds(), string(payload))
	Expect(status).Equals(http.StatusBadRequest)
}

func TestSaveCampaignGraph_StaleConfigVersion_RealJSON_409(t *testing.T) {
	RegisterT(t)

	var got *cmd.SaveSponsorshipCampaignGraph
	bus.AddHandler(func(ctx context.Context, c *cmd.SaveSponsorshipCampaignGraph) error {
		got = c
		return app.ErrConflict
	})

	body := `{
		"name":"Camp","advertiser":"Acme",
		"startAt":"2026-01-01T00:00:00Z","endAt":"2026-12-01T00:00:00Z",
		"weight":10,"locale":"all","enabled":true,
		"configVersion":3,
		"assignments":[{"placementId":"feed_native","creativeVersionId":9}]
	}`
	server := mock.NewServer()
	status, resp := server.
		OnTenant(mock.DemoTenant).
		AsUser(mock.JonSnow).
		AddParam("id", 42).
		ExecutePost(apiv1.SaveCampaignGraph(), body)

	Expect(status).Equals(http.StatusConflict)
	Expect(resp.Body.String()).ContainsSubstring(`"message":"Conflict"`)
	Expect(got).IsNotNil()
	Expect(got.ID).Equals(42)
	Expect(got.ConfigVersion).Equals(3)
	Expect(len(got.Assignments)).Equals(1)
	Expect(got.Assignments[0].PlacementID).Equals("feed_native")
}

func TestSaveCampaignGraph_ExpectedZero_RealJSON_409(t *testing.T) {
	RegisterT(t)

	bus.AddHandler(func(ctx context.Context, c *cmd.SaveSponsorshipCampaignGraph) error {
		Expect(c.ConfigVersion).Equals(0)
		return app.ErrConflict
	})

	body := `{
		"name":"Camp","advertiser":"Acme",
		"startAt":"2026-01-01T00:00:00Z","endAt":"2026-12-01T00:00:00Z",
		"weight":10,"locale":"all","enabled":true,
		"configVersion":0,
		"assignments":[]
	}`
	server := mock.NewServer()
	status, resp := server.
		OnTenant(mock.DemoTenant).
		AsUser(mock.JonSnow).
		AddParam("id", 7).
		ExecutePost(apiv1.SaveCampaignGraph(), body)
	Expect(status).Equals(http.StatusConflict)
	Expect(resp.Body.String()).ContainsSubstring(`"message":"Conflict"`)
}

func TestCreateCreativeVersion_RealJSON_ReturnsConfigVersion(t *testing.T) {
	RegisterT(t)

	bus.AddHandler(func(ctx context.Context, c *cmd.CreateCreativeVersion) error {
		Expect(c.ConfigVersion).Equals(5)
		Expect(c.CampaignID).Equals(11)
		c.Result = &entity.CreativeVersion{
			ID: 99, CampaignID: 11, VersionNo: 2,
			ImageURL: c.ImageURL, ClickURL: c.ClickURL, CreatedAt: time.Now().UTC(),
		}
		c.NewConfigVersion = 6
		return nil
	})

	body := `{
		"imageUrl":"https://cdn.example/a.png",
		"html":"",
		"clickUrl":"https://example.com/a",
		"configVersion":5
	}`
	server := mock.NewServer()
	status, resp := server.
		OnTenant(mock.DemoTenant).
		AsUser(mock.JonSnow).
		AddParam("id", 11).
		ExecutePost(apiv1.CreateCreativeVersion(), body)
	Expect(status).Equals(http.StatusOK)
	Expect(resp.Body.String()).ContainsSubstring(`"configVersion":6`)
	Expect(resp.Body.String()).ContainsSubstring(`"versionNo":2`)
}

func TestCreateCreativeVersion_MissingConfigVersion_409(t *testing.T) {
	RegisterT(t)
	bus.AddHandler(func(ctx context.Context, c *cmd.CreateCreativeVersion) error {
		Expect(c.ConfigVersion).Equals(0)
		return app.ErrConflict
	})
	server := mock.NewServer()
	status, resp := server.
		OnTenant(mock.DemoTenant).
		AsUser(mock.JonSnow).
		AddParam("id", 11).
		ExecutePost(apiv1.CreateCreativeVersion(), `{
			"imageUrl":"https://cdn.example/a.png","html":"","clickUrl":"https://example.com/a"
		}`)
	Expect(status).Equals(http.StatusConflict)
	Expect(resp.Body.String()).ContainsSubstring(`"message":"Conflict"`)
}

func TestCreateSponsorshipCampaign_WithVersionAssignments_RealJSON(t *testing.T) {
	RegisterT(t)

	bus.AddHandler(func(ctx context.Context, c *cmd.CreateSponsorshipCampaign) error {
		Expect(c.Version).IsNotNil()
		Expect(c.Version.ClickURL).Equals("https://example.com/h")
		Expect(len(c.Assignments)).Equals(1)
		Expect(c.Assignments[0].PlacementID).Equals("feed_native")
		c.Result = &entity.SponsorshipCampaign{
			ID: 55, Name: c.Name, Advertiser: c.Advertiser,
			StartAt: c.StartAt, EndAt: c.EndAt, Weight: c.Weight, Locale: c.Locale,
			Enabled: c.Enabled, ConfigVersion: 1, CreatedAt: time.Now().UTC(),
		}
		c.VersionResult = &entity.CreativeVersion{
			ID: 70, CampaignID: 55, VersionNo: 1,
			ImageURL: c.Version.ImageURL, ClickURL: c.Version.ClickURL,
		}
		c.AssignmentResults = []*entity.CampaignAssignment{{
			ID: 1, CampaignID: 55, PlacementID: "feed_native", CreativeVersionID: 70,
		}}
		return nil
	})

	body := `{
		"name":"House","advertiser":"House Adv",
		"startAt":"2026-01-01T00:00:00Z","endAt":"2026-12-01T00:00:00Z",
		"weight":10,"locale":"all","enabled":true,
		"version":{"imageUrl":"https://cdn.example/h.png","html":"","clickUrl":"https://example.com/h"},
		"assignments":[{"placementId":"feed_native","creativeVersionId":0}]
	}`
	server := mock.NewServer()
	status, resp := server.
		OnTenant(mock.DemoTenant).
		AsUser(mock.JonSnow).
		ExecutePost(apiv1.CreateSponsorshipCampaign(), body)
	Expect(status).Equals(http.StatusOK)
	Expect(resp.Body.String()).ContainsSubstring(`"configVersion":1`)
	Expect(resp.Body.String()).ContainsSubstring(`"feed_native"`)
	Expect(resp.Body.String()).ContainsSubstring(`"campaign"`)
}

func TestUpdateAdPlacement_PatchesFields(t *testing.T) {
	RegisterT(t)
	bus.AddHandler(func(ctx context.Context, c *cmd.UpdateAdPlacement) error {
		c.Result = &entity.AdPlacement{
			ID:            c.ID,
			Name:          "Sidebar",
			Enabled:       true,
			AdSenseSlotID: c.AdSenseSlotID,
			AdSenseFormat: c.AdSenseFormat,
			EmptyPolicy:   c.EmptyPolicy,
		}
		return nil
	})

	server := mock.NewServer()
	status, resp := server.
		OnTenant(mock.DemoTenant).
		AsUser(mock.JonSnow).
		AddParam("id", "sidebar_top").
		ExecutePost(
			apiv1.UpdateAdPlacement(),
			`{"adsenseSlotId":"999001","adsenseFormat":"rectangle","emptyPolicy":"reserve"}`,
		)
	Expect(status).Equals(http.StatusOK)
	Expect(resp.Body.String()).ContainsSubstring(`"adsenseSlotId":"999001"`)
	Expect(resp.Body.String()).ContainsSubstring(`"emptyPolicy":"reserve"`)
}

func TestUpdateAdPlacement_RejectsBadPolicy(t *testing.T) {
	RegisterT(t)
	server := mock.NewServer()
	status, _ := server.
		OnTenant(mock.DemoTenant).
		AsUser(mock.JonSnow).
		AddParam("id", "sidebar_top").
		ExecutePost(
			apiv1.UpdateAdPlacement(),
			`{"adsenseSlotId":"1","adsenseFormat":"auto","emptyPolicy":"nope"}`,
		)
	Expect(status).Equals(http.StatusBadRequest)
}

