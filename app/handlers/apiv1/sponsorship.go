package apiv1

import (
	"fmt"
	"strings"

	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func ListSponsorshipPackages() web.HandlerFunc {
	return func(c *web.Context) error {
		q := &query.ListSponsorshipPackages{}
		if err := bus.Dispatch(c, q); err != nil {
			return c.Failure(err)
		}
		if q.Result == nil {
			q.Result = []*entity.SponsorshipPackage{}
		}
		return c.Ok(q.Result)
	}
}

func CreateSponsorshipPackage() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.CreateSponsorshipPackage)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}
		return c.WithTransaction(func() error {
			create := &cmd.CreateSponsorshipPackage{
				Slug: action.Slug, Name: action.Name, Description: action.Description,
				Slots: action.Slots, DurationDays: action.DurationDays, Sort: action.Sort,
			}
			if err := bus.Dispatch(c, create); err != nil {
				return c.Failure(err)
			}
			return c.Ok(create.Result)
		})
	}
}

func UpdateSponsorshipPackage() web.HandlerFunc {
	return func(c *web.Context) error {
		id, err := c.ParamAsInt("id")
		if err != nil {
			return c.BadRequest(web.Map{"message": "Invalid ID"})
		}
		action := new(actions.UpdateSponsorshipPackage)
		action.ID = id
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}
		return c.WithTransaction(func() error {
			update := &cmd.UpdateSponsorshipPackage{
				ID: id, Slug: action.Slug, Name: action.Name, Description: action.Description,
				Slots: action.Slots, DurationDays: action.DurationDays, Sort: action.Sort,
			}
			if err := bus.Dispatch(c, update); err != nil {
				return c.Failure(err)
			}
			return c.Ok(update.Result)
		})
	}
}

func DeleteSponsorshipPackage() web.HandlerFunc {
	return func(c *web.Context) error {
		id, err := c.ParamAsInt("id")
		if err != nil {
			return c.BadRequest(web.Map{"message": "Invalid ID"})
		}
		return c.WithTransaction(func() error {
			if err := bus.Dispatch(c, &cmd.DeleteSponsorshipPackage{ID: id}); err != nil {
				return c.Failure(err)
			}
			return c.Ok(web.Map{})
		})
	}
}

func ListSponsorshipCampaigns() web.HandlerFunc {
	return func(c *web.Context) error {
		q := &query.ListSponsorshipCampaigns{}
		if err := bus.Dispatch(c, q); err != nil {
			return c.Failure(err)
		}
		if q.Result == nil {
			q.Result = []*entity.SponsorshipCampaign{}
		}
		return c.Ok(q.Result)
	}
}

func CreateSponsorshipCampaign() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.CreateSponsorshipCampaign)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}
		if action.Locale == "" {
			action.Locale = "all"
		}
		return c.WithTransaction(func() error {
			create := &cmd.CreateSponsorshipCampaign{
				Name: action.Name, Slots: action.Slots,
				CreativeImageURL: action.CreativeImageURL, CreativeHTML: action.CreativeHTML,
				ClickURL: action.ClickURL, StartAt: action.StartAt.UTC(), EndAt: action.EndAt.UTC(),
				Weight: action.Weight, Locale: action.Locale, Enabled: action.Enabled,
				PackageID: action.PackageID,
			}
			if err := bus.Dispatch(c, create); err != nil {
				return c.Failure(err)
			}
			return c.Ok(create.Result)
		})
	}
}

func UpdateSponsorshipCampaign() web.HandlerFunc {
	return func(c *web.Context) error {
		id, err := c.ParamAsInt("id")
		if err != nil {
			return c.BadRequest(web.Map{"message": "Invalid ID"})
		}
		action := new(actions.UpdateSponsorshipCampaign)
		action.ID = id
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}
		if action.Locale == "" {
			action.Locale = "all"
		}
		return c.WithTransaction(func() error {
			update := &cmd.UpdateSponsorshipCampaign{
				ID: id, Name: action.Name, Slots: action.Slots,
				CreativeImageURL: action.CreativeImageURL, CreativeHTML: action.CreativeHTML,
				ClickURL: action.ClickURL, StartAt: action.StartAt.UTC(), EndAt: action.EndAt.UTC(),
				Weight: action.Weight, Locale: action.Locale, Enabled: action.Enabled,
				PackageID: action.PackageID,
			}
			if err := bus.Dispatch(c, update); err != nil {
				return c.Failure(err)
			}
			return c.Ok(update.Result)
		})
	}
}

func DeleteSponsorshipCampaign() web.HandlerFunc {
	return func(c *web.Context) error {
		id, err := c.ParamAsInt("id")
		if err != nil {
			return c.BadRequest(web.Map{"message": "Invalid ID"})
		}
		return c.WithTransaction(func() error {
			if err := bus.Dispatch(c, &cmd.DeleteSponsorshipCampaign{ID: id}); err != nil {
				return c.Failure(err)
			}
			return c.Ok(web.Map{})
		})
	}
}

func publicCampaign(slot string, camp *entity.SponsorshipCampaign) entity.PublicSponsorshipCampaign {
	return entity.PublicSponsorshipCampaign{
		ID: camp.ID, Name: camp.Name, SlotID: slot,
		CreativeImageURL: camp.CreativeImageURL, CreativeHTML: camp.CreativeHTML,
		ClickPath: fmt.Sprintf("/ads/click/%d", camp.ID),
	}
}

// GetActiveSponsorship:
//   ?slot=feed_native&locale=en  -> single campaign or {}
//   ?slots=feed_native,sidebar_top&locale=en -> { "feed_native": {...}|null, ... }
func GetActiveSponsorship() web.HandlerFunc {
	return func(c *web.Context) error {
		locale := c.QueryParam("locale")
		if locale == "" {
			locale = "all"
		}
		slotsParam := strings.TrimSpace(c.QueryParam("slots"))
		if slotsParam != "" {
			raw := strings.Split(slotsParam, ",")
			slotIDs := make([]string, 0, len(raw))
			seen := map[string]bool{}
			for _, s := range raw {
				s = strings.TrimSpace(s)
				if s == "" || seen[s] {
					continue
				}
				seen[s] = true
				slotIDs = append(slotIDs, s)
			}
			q := &query.GetActiveSponsorshipForSlots{SlotIDs: slotIDs, Locale: locale}
			if err := bus.Dispatch(c, q); err != nil {
				return c.Failure(err)
			}
			out := web.Map{}
			for _, slot := range slotIDs {
				camp := q.Result[slot]
				if camp == nil {
					out[slot] = nil
				} else {
					out[slot] = publicCampaign(slot, camp)
				}
			}
			return c.Ok(out)
		}

		slotID := c.QueryParam("slot")
		q := &query.GetActiveSponsorshipForSlot{SlotID: slotID, Locale: locale}
		if err := bus.Dispatch(c, q); err != nil {
			return c.Failure(err)
		}
		if q.Result == nil {
			return c.Ok(web.Map{})
		}
		return c.Ok(publicCampaign(slotID, q.Result))
	}
}
