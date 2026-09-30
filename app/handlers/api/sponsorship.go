package api

import (
	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
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

		return c.Ok(q.Result)
	}
}

func CreateSponsorshipPackage() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.CreateSponsorshipPackage)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		create := &cmd.CreateSponsorshipPackage{
			Slug: action.Slug, Name: action.Name, Description: action.Description,
			Slots: action.Slots, DurationDays: action.DurationDays, Sort: action.Sort,
			SubmissionID: action.SubmissionID,
		}
		if err := bus.Dispatch(c, create); err != nil {
			return c.Failure(err)
		}

		return c.Ok(create.Result)
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

		update := &cmd.UpdateSponsorshipPackage{
			ID: id, Slug: action.Slug, Name: action.Name, Description: action.Description,
			Slots: action.Slots, DurationDays: action.DurationDays, Sort: action.Sort,
			SubmissionID: action.SubmissionID,
		}
		if err := bus.Dispatch(c, update); err != nil {
			return c.Failure(err)
		}

		return c.Ok(update.Result)
	}
}

func DeleteSponsorshipPackage() web.HandlerFunc {
	return func(c *web.Context) error {
		id, err := c.ParamAsInt("id")
		if err != nil {
			return c.BadRequest(web.Map{"message": "Invalid ID"})
		}
		if err := bus.Dispatch(c, &cmd.DeleteSponsorshipPackage{ID: id}); err != nil {
			return c.Failure(err)
		}

		return c.Ok(web.Map{})
	}
}
