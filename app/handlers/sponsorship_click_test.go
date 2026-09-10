package handlers_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	. "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/assert"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/mock"
)

func liveCampaign(id int) *entity.SponsorshipCampaign {
	now := time.Now().UTC()
	return &entity.SponsorshipCampaign{
		ID: id, Name: "Camp", Advertiser: "Acme",
		Enabled: true, StartAt: now.Add(-time.Hour), EndAt: now.Add(time.Hour),
	}
}

func stubClick(camp *entity.SponsorshipCampaign, ver *entity.CreativeVersion, increment *bool) {
	bus.AddHandler(func(ctx context.Context, q *query.GetSponsorshipCampaignByID) error {
		if camp == nil || q.ID != camp.ID {
			return app.ErrNotFound
		}
		q.Result = camp
		return nil
	})
	bus.AddHandler(func(ctx context.Context, q *query.GetCreativeVersionsByIDs) error {
		q.Result = map[int]*entity.CreativeVersion{}
		if ver != nil {
			q.Result[ver.ID] = ver
		}
		return nil
	})
	bus.AddHandler(func(ctx context.Context, c *cmd.IncrementSponsorshipClick) error {
		if increment != nil {
			*increment = true
		}
		return nil
	})
}

func TestSponsorshipClick_HappyPath_RedirectsAndIncrements(t *testing.T) {
	RegisterT(t)
	incremented := false
	camp := liveCampaign(7)
	ver := &entity.CreativeVersion{ID: 3, CampaignID: 7, ClickURL: "https://example.com/dest"}
	stubClick(camp, ver, &incremented)

	server := mock.NewServer()
	status, resp := server.
		OnTenant(mock.DemoTenant).
		WithURL("http://demo.test.fider.io/ads/click/7?v=3").
		AddParam("id", 7).
		Execute(handlers.SponsorshipClick())

	Expect(status).Equals(http.StatusTemporaryRedirect)
	Expect(resp.Header().Get("Location")).Equals("https://example.com/dest")
	Expect(incremented).IsTrue()
}

func TestSponsorshipClick_MissingV_404NoIncrement(t *testing.T) {
	RegisterT(t)
	incremented := false
	camp := liveCampaign(7)
	ver := &entity.CreativeVersion{ID: 3, CampaignID: 7, ClickURL: "https://example.com/dest"}
	stubClick(camp, ver, &incremented)

	server := mock.NewServer()
	status, resp := server.
		OnTenant(mock.DemoTenant).
		WithURL("http://demo.test.fider.io/ads/click/7").
		AddParam("id", 7).
		Execute(handlers.SponsorshipClick())

	Expect(status).Equals(http.StatusNotFound)
	Expect(resp.Header().Get("Location")).Equals("")
	Expect(incremented).IsFalse()
}

func TestSponsorshipClick_ForeignVersion_404NoIncrement(t *testing.T) {
	RegisterT(t)
	incremented := false
	camp := liveCampaign(7)
	ver := &entity.CreativeVersion{ID: 99, CampaignID: 8, ClickURL: "https://example.com/other"}
	stubClick(camp, ver, &incremented)

	server := mock.NewServer()
	status, _ := server.
		OnTenant(mock.DemoTenant).
		WithURL("http://demo.test.fider.io/ads/click/7?v=99").
		AddParam("id", 7).
		Execute(handlers.SponsorshipClick())

	Expect(status).Equals(http.StatusNotFound)
	Expect(incremented).IsFalse()
}

func TestSponsorshipClick_Disabled_404NoIncrement(t *testing.T) {
	RegisterT(t)
	incremented := false
	camp := liveCampaign(7)
	camp.Enabled = false
	ver := &entity.CreativeVersion{ID: 3, CampaignID: 7, ClickURL: "https://example.com/dest"}
	stubClick(camp, ver, &incremented)

	server := mock.NewServer()
	status, _ := server.
		OnTenant(mock.DemoTenant).
		WithURL("http://demo.test.fider.io/ads/click/7?v=3").
		AddParam("id", 7).
		Execute(handlers.SponsorshipClick())

	Expect(status).Equals(http.StatusNotFound)
	Expect(incremented).IsFalse()
}

func TestSponsorshipClick_OutOfSchedule_404NoIncrement(t *testing.T) {
	RegisterT(t)
	incremented := false
	camp := liveCampaign(7)
	now := time.Now().UTC()
	camp.StartAt = now.Add(-48 * time.Hour)
	camp.EndAt = now.Add(-24 * time.Hour)
	ver := &entity.CreativeVersion{ID: 3, CampaignID: 7, ClickURL: "https://example.com/dest"}
	stubClick(camp, ver, &incremented)

	server := mock.NewServer()
	status, _ := server.
		OnTenant(mock.DemoTenant).
		WithURL("http://demo.test.fider.io/ads/click/7?v=3").
		AddParam("id", 7).
		Execute(handlers.SponsorshipClick())

	Expect(status).Equals(http.StatusNotFound)
	Expect(incremented).IsFalse()
}

func TestSponsorshipClick_Deleted_404NoIncrement(t *testing.T) {
	RegisterT(t)
	incremented := false
	stubClick(nil, &entity.CreativeVersion{ID: 3, CampaignID: 7, ClickURL: "https://example.com/dest"}, &incremented)

	server := mock.NewServer()
	status, _ := server.
		OnTenant(mock.DemoTenant).
		WithURL("http://demo.test.fider.io/ads/click/7?v=3").
		AddParam("id", 7).
		Execute(handlers.SponsorshipClick())

	Expect(status).Equals(http.StatusNotFound)
	Expect(incremented).IsFalse()
}

func TestSponsorshipClick_BadURL_404NoIncrement(t *testing.T) {
	RegisterT(t)
	incremented := false
	camp := liveCampaign(7)
	ver := &entity.CreativeVersion{ID: 3, CampaignID: 7, ClickURL: "javascript:alert(1)"}
	stubClick(camp, ver, &incremented)

	server := mock.NewServer()
	status, _ := server.
		OnTenant(mock.DemoTenant).
		WithURL("http://demo.test.fider.io/ads/click/7?v=3").
		AddParam("id", 7).
		Execute(handlers.SponsorshipClick())

	Expect(status).Equals(http.StatusNotFound)
	Expect(incremented).IsFalse()
}
