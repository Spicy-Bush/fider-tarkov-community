package api

import (
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/adsselect"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/proxy"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func UploadSponsorImage() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.UploadSponsorImage)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		command := &cmd.UploadSponsorImage{Name: action.Name, Content: action.File.Upload.Content, SubmissionID: action.SubmissionID}
		if err := bus.Dispatch(c, command); err != nil {
			return c.Failure(err)
		}

		return c.Ok(command.Result)
	}
}

func SaveSponsorCampaign() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.SaveSponsorCampaign)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		command := &cmd.SaveSponsorCampaign{
			BaseCampaign: action.BaseCampaign,
			BaseCreative: action.BaseCreative,
			Campaign:     action.Campaign,
			Creative:     action.Creative,
			SubmissionID: action.SubmissionID,
		}
		if err := bus.Dispatch(c, command); err != nil {
			return c.Failure(err)
		}

		if command.DraftCampaign != nil {
			return c.Ok(web.Map{
				"kind":     "conflict",
				"fields":   command.Conflicts,
				"problems": append([]string{}, command.Problems...),
				"saved":    web.Map{"campaign": command.Result, "creative": command.CreativeResult},
				"draft":    web.Map{"campaign": command.DraftCampaign, "creative": command.DraftCreative},
			})
		}

		return c.Ok(web.Map{"kind": "saved", "saved": web.Map{"campaign": command.Result, "creative": command.CreativeResult}})
	}
}

func SaveSponsorPlacement() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.SaveSponsorPlacement)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		command := &cmd.SaveSponsorPlacement{Placement: action.Placement, SubmissionID: action.SubmissionID}
		if err := bus.Dispatch(c, command); err != nil {
			return c.Failure(err)
		}

		return c.Ok(command.Result)
	}
}

func DeleteSponsorCampaign() web.HandlerFunc {
	return func(c *web.Context) error {
		id, err := c.ParamAsInt("id")
		if err != nil || id < 1 {
			return c.BadRequest(web.Map{"message": "Choose a valid campaign."})
		}

		if err := bus.Dispatch(c, &cmd.DeleteSponsorCampaign{ID: id}); err != nil {
			return c.Failure(err)
		}

		return c.NoContent(http.StatusNoContent)
	}
}

func DeleteSponsorCreative() web.HandlerFunc {
	return func(c *web.Context) error {
		id, err := c.ParamAsInt("id")
		if err != nil || id < 1 {
			return c.BadRequest(web.Map{"message": "Choose valid artwork."})
		}

		if err := bus.Dispatch(c, &cmd.DeleteSponsorCreative{ID: id}); err != nil {
			return c.Failure(err)
		}

		return c.NoContent(http.StatusNoContent)
	}
}

func trustedSponsorCountry(request *http.Request, trustedNetwork string) string {
	country := proxy.Header(request, "CF-IPCountry", []string{trustedNetwork})
	if len(country) != 2 || country == "XX" || country[0] < 'A' || country[0] > 'Z' || country[1] < 'A' || country[1] > 'Z' {
		return ""
	}

	return country
}

func AllocateSponsors() web.HandlerFunc {
	return func(c *web.Context) error {
		var body struct {
			PageToken     string                      `json:"pageToken"`
			Context       entity.SponsorContext       `json:"context"`
			Opportunities []entity.SponsorOpportunity `json:"opportunities"`
		}
		if err := c.Bind(&body); err != nil {
			return c.BadRequest(web.Map{"message": "Invalid sponsorship request."})
		}

		viewer := body.Context
		page, err := adsselect.ReadPage(body.PageToken, env.Config.JWTSecret, c.Tenant().ID, c.SessionID(), time.Now())
		if err != nil || page.Kind != viewer.PageType || page.ContentID != viewer.ID {
			return c.Forbidden()
		}

		if !slices.Contains([]string{"home", "post", "page"}, viewer.PageType) || !slices.Contains([]string{"en", "ru"}, viewer.Language) || !slices.Contains([]string{"desktop", "mobile"}, viewer.Device) || viewer.ID < 0 || len(body.Opportunities) > 32 {
			return c.BadRequest(web.Map{"message": "Invalid sponsorship context."})
		}

		seen := make(map[string]bool)
		placements := &query.GetSponsorPlacements{}
		if err := bus.Dispatch(c, placements); err != nil {
			return c.Failure(err)
		}
		for _, opportunity := range body.Opportunities {
			placementIndex := slices.IndexFunc(placements.Result, func(p entity.SponsorPlacement) bool { return p.ID == opportunity.PlacementID })
			if placementIndex < 0 || seen[opportunity.InstanceID] {
				return c.BadRequest(web.Map{"message": "Choose unique instances of supported placements."})
			}

			placement := placements.Result[placementIndex]
			if placement.Device != viewer.Device || (placement.PageType != "all" && placement.PageType != page.Kind) {
				return c.Forbidden()
			}
			if placement.Position == "feed" {
				grant := page
				if opportunity.PageToken != "" {
					grant, err = adsselect.ReadPage(opportunity.PageToken, env.Config.JWTSecret, c.Tenant().ID, c.SessionID(), time.Now())
					if err != nil || grant.ID != page.ID || grant.Kind != "home" {
						return c.Forbidden()
					}
				}

				postID, err := strconv.Atoi(strings.TrimPrefix(opportunity.InstanceID, "feed-"))
				if err != nil || opportunity.InstanceID != "feed-"+strconv.Itoa(postID) || !slices.Contains(grant.PostIDs, postID) {
					return c.Forbidden()
				}
			} else if opportunity.InstanceID != placement.ID {
				return c.Forbidden()
			}

			seen[opportunity.InstanceID] = true
		}

		viewer.Country = trustedSponsorCountry(c.Request.Original(), env.Config.SponsorCountryProxy)
		command := &cmd.AllocateSponsors{
			PageID: page.ID, ExpiresAt: time.Unix(page.Expires, 0),
			Context: viewer, Opportunities: body.Opportunities,
		}
		if err := bus.Dispatch(c, command); err != nil {
			return c.Failure(err)
		}

		c.Response.Header().Set("Cache-Control", "private, no-store")
		return c.Ok(command.Result)
	}
}

func SponsorReport() web.HandlerFunc {
	return func(c *web.Context) error {
		id, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		q := &query.GetSponsorReport{CampaignID: id}
		if err := bus.Dispatch(c, q); err != nil {
			return c.Failure(err)
		}

		return c.Ok(q)
	}
}

func SaveSponsorExclusion() web.HandlerFunc {
	return func(c *web.Context) error {
		var body struct {
			SubmissionID string                  `json:"submissionId"`
			Exclusion    entity.SponsorExclusion `json:"exclusion"`
			Excluded     bool                    `json:"excluded"`
		}
		if err := c.Bind(&body); err != nil {
			return c.BadRequest(web.Map{"message": "Invalid exclusion."})
		}

		command := &cmd.SaveSponsorExclusion{Exclusion: body.Exclusion, Excluded: body.Excluded, SubmissionID: body.SubmissionID}
		if err := bus.Dispatch(c, command); err != nil {
			return c.Failure(err)
		}

		return c.Ok(command.Result)
	}
}
