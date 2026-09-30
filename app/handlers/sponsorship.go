package handlers

import (
	"net/http"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

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
				"packages":   packages.Result,
				"placements": entity.SponsorPlacements,
				"contact":    "contact@tarkov.community",
			},
		})
	}
}
