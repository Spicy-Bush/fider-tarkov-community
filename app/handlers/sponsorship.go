package handlers

import (
	"net/http"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func ManageSponsorshipPage() web.HandlerFunc {
	return func(c *web.Context) error {
		campaigns := &query.ListSponsorshipCampaigns{}
		packages := &query.ListSponsorshipPackages{}
		if err := bus.Dispatch(c, campaigns, packages); err != nil {
			return c.Failure(err)
		}
		return c.Page(http.StatusOK, web.Props{
			Page:  "Administration/pages/ManageSponsorship.page",
			Title: "Sponsorship - Site Settings",
			Data: web.Map{
				"campaigns": campaigns.Result,
				"packages":  packages.Result,
				"slots":     []string{"feed_native", "sidebar_top", "post_below_title", "pages_header"},
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
		get := &query.GetSponsorshipCampaignByID{ID: id}
		if err := bus.Dispatch(c, get); err != nil || get.Result == nil {
			return c.NotFound()
		}
		_ = bus.Dispatch(c, &cmd.IncrementSponsorshipClick{ID: id})
		return c.Redirect(get.Result.ClickURL)
	}
}
