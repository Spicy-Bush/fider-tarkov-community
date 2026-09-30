package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/adsselect"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/log"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/rand"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func SponsorClick() web.HandlerFunc {
	return func(c *web.Context) error {
		click, err := adsselect.ReadClick(c.QueryParam("token"), env.Config.JWTSecret, c.Tenant().ID, time.Now())
		if err != nil {
			return c.NotFound()
		}

		if err := bus.Dispatch(c, &cmd.RecordSponsorClick{Click: *click}); err != nil {
			log.Error(c, err)
		}

		c.Response.Header().Set("Cache-Control", "private, no-store")
		c.Response.Header().Set("Referrer-Policy", "no-referrer")
		return c.Redirect(click.Destination)
	}
}

func sponsorPage(c *web.Context, page adsselect.Page) (string, error) {
	page.ID = rand.String(32)
	page.TenantID = c.Tenant().ID
	page.Session = adsselect.SessionKey(c.SessionID())
	page.Expires = time.Now().Add(24 * time.Hour).Unix()

	return page.Token(env.Config.JWTSecret)
}

func sponsorPlacements(c *web.Context) []entity.SponsorPlacement {
	q := &query.GetSponsorPlacements{}
	if err := bus.Dispatch(c, q); err != nil {
		log.Error(c, err)
		return []entity.SponsorPlacement{}
	}

	return q.Result
}

func SponsorManagementPage() web.HandlerFunc {
	return func(c *web.Context) error {
		q := &query.GetSponsorManagement{}
		q.Browse.Search = strings.TrimSpace(c.QueryParam("search"))
		if len(q.Browse.Search) > 100 {
			return c.Failure(validate.Failed("Search must be 100 characters or fewer."))
		}

		for name, target := range map[string]*int{
			"page": &q.Browse.Page, "campaign": &q.Browse.CampaignID, "artworkPage": &q.Browse.ArtworkPage,
		} {
			if value := c.QueryParam(name); value != "" {
				number, err := strconv.Atoi(value)
				if err != nil || number < 1 || (name != "campaign" && number > 1000000) {
					return c.Failure(validate.Failed("Choose a valid " + name + "."))
				}
				*target = number
			}
		}

		packages := &query.ListSponsorshipPackages{}
		if err := bus.Dispatch(c, q, packages); err != nil {
			return c.Failure(err)
		}

		if packages.Result == nil {
			packages.Result = []*entity.SponsorshipPackage{}
		}

		return c.Page(http.StatusOK, web.Props{
			Page:  "Administration/pages/ManageSponsorship.page",
			Title: "Sponsorship",
			Data:  web.Map{"management": q.Result, "packages": packages.Result, "edit": c.QueryParam("edit") == "1"},
		})
	}
}
