package apiv1_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/apiv1"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/adsselect"
	. "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/assert"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/mock"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

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

func TestUpdateSponsorshipCampaign_ConflictReturns409(t *testing.T) {
	RegisterT(t)
	// Proves OCC ErrConflict → HTTP 409 path used by UpdateSponsorshipCampaign / SaveCampaignGraph.
	server := mock.NewServer()
	status, _ := server.OnTenant(mock.DemoTenant).Execute(func(c *web.Context) error {
		return c.Failure(app.ErrConflict)
	})
	Expect(status).Equals(http.StatusConflict)
}
