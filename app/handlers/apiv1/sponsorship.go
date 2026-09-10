package apiv1

import (
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
				Name: action.Name, Advertiser: action.Advertiser,
				StartAt: action.StartAt.UTC(), EndAt: action.EndAt.UTC(),
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
				ID: id, Name: action.Name, Advertiser: action.Advertiser,
				StartAt: action.StartAt.UTC(), EndAt: action.EndAt.UTC(),
				Weight: action.Weight, Locale: action.Locale, Enabled: action.Enabled,
				PackageID: action.PackageID, ConfigVersion: action.ConfigVersion,
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
