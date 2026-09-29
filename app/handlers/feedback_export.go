package handlers

import (
	"net/http"

	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func FeedbackExportPage() web.HandlerFunc {
	return func(c *web.Context) error {
		tags := &query.GetAllTags{}
		presets := &query.ListFeedbackExportPresets{}
		if err := bus.Dispatch(c, tags, presets); err != nil {
			return c.Failure(err)
		}

		return c.Page(http.StatusOK, web.Props{
			Page:  "Administration/pages/FeedbackExport.page",
			Title: "BSG Export | Site Settings",
			Data: web.Map{
				"tags":     tags.Result,
				"presets":  presets.Result,
				"statuses": entity.FeedbackExportStatuses,
				"limits": web.Map{
					"sections": entity.MaxFeedbackExportSections,
					"picks":    entity.MaxFeedbackExportPicks,
					"pickSize": entity.MaxFeedbackExportPickSize,
					"rows":     entity.MaxFeedbackExportRows,
				},
			},
		})
	}
}

func PreviewFeedbackExport() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.PreviewFeedbackExport)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		rows := &query.SelectFeedbackExportRows{Recipe: action.Recipe, Seed: action.Seed}
		if err := bus.Dispatch(c, rows); err != nil {
			return c.Failure(err)
		}

		return c.Ok(web.Map{"sections": rows.Result})
	}
}

func CreateFeedbackExportPreset() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.CreateFeedbackExportPreset)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		create := &cmd.CreateFeedbackExportPreset{ID: action.ID, Name: action.Name, Recipe: action.Recipe}
		if err := bus.Dispatch(c, create); err != nil {
			return c.Failure(err)
		}
		return c.Ok(create.Result)
	}
}

func UpdateFeedbackExportPreset() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.UpdateFeedbackExportPreset)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		update := &cmd.UpdateFeedbackExportPreset{
			SubmissionID: action.SubmissionID,
			ID:           action.ID, Name: action.Name, Recipe: action.Recipe, Saved: *action.Saved,
		}
		if err := bus.Dispatch(c, update); err != nil {
			return c.Failure(err)
		}
		return c.Ok(update.Result)
	}
}

func DeleteFeedbackExportPreset() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.DeleteFeedbackExportPreset)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		if err := bus.Dispatch(c, &cmd.DeleteFeedbackExportPreset{ID: action.ID}); err != nil {
			return c.Failure(err)
		}
		return c.Ok(web.Map{})
	}
}
