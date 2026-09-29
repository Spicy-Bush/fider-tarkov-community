package api

import (
	"strings"

	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func SearchPages() web.HandlerFunc {
	return func(c *web.Context) error {
		page, _ := c.QueryParamAsInt("page")
		if page <= 0 {
			page = 1
		}

		listPages := &query.ListPages{
			Query:  c.QueryParam("q"),
			View:   c.QueryParam("view"),
			Limit:  20,
			Offset: (page - 1) * 20,
		}

		if topics := c.QueryParam("topics"); topics != "" {
			listPages.Topics = []string{topics}
		}
		if tags := c.QueryParam("tags"); tags != "" {
			tagList := strings.Split(tags, ",")
			for i := range tagList {
				tagList[i] = strings.TrimSpace(tagList[i])
			}
			listPages.Tags = tagList
		}

		if err := bus.Dispatch(c, listPages); err != nil {
			return c.Failure(err)
		}

		return c.Ok(web.Map{
			"pages":      listPages.Result,
			"totalCount": listPages.TotalCount,
			"totalPages": (listPages.TotalCount + 19) / 20,
			"page":       page,
		})
	}
}


func DeletePage() web.HandlerFunc {
	return func(c *web.Context) error {
		pageID, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		action := &actions.DeletePage{PageID: pageID}
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			if err := bus.Dispatch(c, &cmd.DeletePage{PageID: pageID}); err != nil {
				return c.Failure(err)
			}
			return c.Ok(web.Map{})
		})
	}
}

func TogglePageReaction() web.HandlerFunc {
	return func(c *web.Context) error {
		pageID, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		action := &actions.TogglePageReaction{PageID: pageID}
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			getPage := &query.GetPageByID{ID: pageID}
			if err := bus.Dispatch(c, getPage); err != nil {
				return c.NotFound()
			}

			if !getPage.Result.AllowReactions {
				return c.BadRequest(web.Map{"message": "Reactions are not allowed on this page"})
			}

			toggleCmd := &cmd.TogglePageReaction{
				Page:  getPage.Result,
				Emoji: action.Emoji,
			}

			if err := bus.Dispatch(c, toggleCmd); err != nil {
				return c.Failure(err)
			}

			return c.Ok(web.Map{"added": toggleCmd.Result})
		})
	}
}

func TogglePageSubscription() web.HandlerFunc {
	return func(c *web.Context) error {
		pageID, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		action := &actions.TogglePageSubscription{PageID: pageID}
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			toggleCmd := &cmd.TogglePageSubscription{PageID: pageID}
			if err := bus.Dispatch(c, toggleCmd); err != nil {
				return c.Failure(err)
			}

			return c.Ok(web.Map{"subscribed": toggleCmd.Result})
		})
	}
}

func CreatePageTopic() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.CreatePageTopic)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			createCmd := &cmd.CreatePageTopic{
				Name:        action.Name,
				Slug:        action.Slug,
				Description: action.Description,
				Color:       action.Color,
			}

			if err := bus.Dispatch(c, createCmd); err != nil {
				return c.Failure(err)
			}

			return c.Ok(createCmd.Result)
		})
	}
}

func UpdatePageTopic() web.HandlerFunc {
	return func(c *web.Context) error {
		topicID, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		action := &actions.UpdatePageTopic{ID: topicID}
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			updateCmd := &cmd.UpdatePageTopic{
				ID:          topicID,
				Name:        action.Name,
				Slug:        action.Slug,
				Description: action.Description,
				Color:       action.Color,
			}

			if err := bus.Dispatch(c, updateCmd); err != nil {
				return c.Failure(err)
			}

			return c.Ok(web.Map{})
		})
	}
}

func DeletePageTopic() web.HandlerFunc {
	return func(c *web.Context) error {
		topicID, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		action := &actions.DeletePageTopic{ID: topicID}
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			if err := bus.Dispatch(c, &cmd.DeletePageTopic{ID: topicID}); err != nil {
				return c.Failure(err)
			}
			return c.Ok(web.Map{})
		})
	}
}

func CreatePageTag() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.CreatePageTag)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			createCmd := &cmd.CreatePageTag{
				Name: action.Name,
				Slug: action.Slug,
			}

			if err := bus.Dispatch(c, createCmd); err != nil {
				return c.Failure(err)
			}

			return c.Ok(createCmd.Result)
		})
	}
}

func UpdatePageTag() web.HandlerFunc {
	return func(c *web.Context) error {
		tagID, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		action := &actions.UpdatePageTag{ID: tagID}
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			updateCmd := &cmd.UpdatePageTag{
				ID:   tagID,
				Name: action.Name,
				Slug: action.Slug,
			}

			if err := bus.Dispatch(c, updateCmd); err != nil {
				return c.Failure(err)
			}

			return c.Ok(web.Map{})
		})
	}
}

func DeletePageTag() web.HandlerFunc {
	return func(c *web.Context) error {
		tagID, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		action := &actions.DeletePageTag{ID: tagID}
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			if err := bus.Dispatch(c, &cmd.DeletePageTag{ID: tagID}); err != nil {
				return c.Failure(err)
			}
			return c.Ok(web.Map{})
		})
	}
}
