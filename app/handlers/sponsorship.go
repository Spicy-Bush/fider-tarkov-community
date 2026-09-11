package handlers

import (
	"net/http"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func ManageSponsorshipPage() web.HandlerFunc {
	return func(c *web.Context) error {
		campaigns := &query.ListSponsorshipCampaigns{}
		packages := &query.ListSponsorshipPackages{}
		placements := &query.ListAdPlacements{}
		if err := bus.Dispatch(c, campaigns, packages, placements); err != nil {
			return c.Failure(err)
		}
		slotIDs := make([]string, 0, len(placements.Result))
		for _, p := range placements.Result {
			if p != nil && p.Enabled {
				slotIDs = append(slotIDs, p.ID)
			}
		}
		if len(slotIDs) == 0 {
			slotIDs = []string{"feed_native", "sidebar_top", "post_below_title", "pages_header"}
		}
		return c.Page(http.StatusOK, web.Props{
			Page:  "Administration/pages/ManageSponsorship.page",
			Title: "Sponsorship - Site Settings",
			Data: web.Map{
				"campaigns":  campaigns.Result,
				"packages":   packages.Result,
				"slots":      slotIDs,
				"placements": placements.Result,
			},
		})
	}
}

func AdvertisePage() web.HandlerFunc {
	return func(c *web.Context) error {
		packages := &query.ListSponsorshipPackages{}
		if err := bus.Dispatch(c, packages); err != nil {
			return c.Failure(err)
		}
		return c.Page(http.StatusOK, web.Props{
			Page:  "Advertise/Advertise.page",
			Title: "Advertise",
			Data: web.Map{
				"packages": packages.Result,
				"contact":  "contact@tarkov.community",
			},
		})
	}
}

func SponsorshipClick() web.HandlerFunc {
	return func(c *web.Context) error {
		id, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}
		vid, vErr := c.QueryParamAsInt("v")
		if vErr != nil || vid <= 0 {
			return c.NotFound()
		}

		get := &query.GetSponsorshipCampaignByID{ID: id}
		if err := bus.Dispatch(c, get); err != nil || get.Result == nil {
			return c.NotFound()
		}
		camp := get.Result
		now := time.Now().UTC()
		if !camp.Enabled || camp.StartAt.After(now) || !camp.EndAt.After(now) {
			return c.NotFound()
		}

		verQ := &query.GetCreativeVersionsByIDs{IDs: []int{vid}}
		if err := bus.Dispatch(c, verQ); err != nil {
			return c.NotFound()
		}
		ver := verQ.Result[vid]
		if ver == nil || ver.CampaignID != id {
			return c.NotFound()
		}
		if ver.ClickURL == "" || !validate.IsHTTPOrHTTPSURL(ver.ClickURL) {
			return c.NotFound()
		}

		if err := bus.Dispatch(c, &cmd.IncrementSponsorshipClick{ID: id}); err != nil {
			return c.NotFound()
		}
		return c.Redirect(ver.ClickURL)
	}
}
