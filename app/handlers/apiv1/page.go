package apiv1

import (
	"strings"

	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"github.com/Spicy-Bush/fider-tarkov-community/app/tasks"
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

func CreatePage() web.HandlerFunc {
	return func(c *web.Context) error {
		action := new(actions.CreateUpdatePage)
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			createCmd := &cmd.CreatePage{
				Title:           action.Title,
				Slug:            action.Slug,
				Content:         action.Content,
				Excerpt:         action.Excerpt,
				BannerImage:     action.BannerImage,
				Status:          entity.PageStatus(action.Status),
				Visibility:      entity.PageVisibility(action.Visibility),
				AllowedRoles:    action.AllowedRoles,
				ParentPageID:    action.ParentPageID,
				AllowComments:   action.AllowComments,
				AllowReactions:  action.AllowReactions,
				ShowTOC:         action.ShowTOC,
				ScheduledFor:    action.ScheduledFor,
				Authors:         action.Authors,
				Topics:          action.Topics,
				Tags:            action.Tags,
				MetaDescription: action.MetaDescription,
				CanonicalURL:    action.CanonicalURL,
			}

			if err := bus.Dispatch(c, createCmd); err != nil {
				return c.Failure(err)
			}

			return c.Ok(createCmd.Result)
		})
	}
}

func UpdatePage() web.HandlerFunc {
	return func(c *web.Context) error {
		pageID, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		action := new(actions.CreateUpdatePage)
		action.PageID = pageID
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			updateCmd := &cmd.UpdatePage{
				PageID:          pageID,
				Title:           action.Title,
				Slug:            action.Slug,
				Content:         action.Content,
				Excerpt:         action.Excerpt,
				BannerImage:     action.BannerImage,
				Status:          entity.PageStatus(action.Status),
				Visibility:      entity.PageVisibility(action.Visibility),
				AllowedRoles:    action.AllowedRoles,
				ParentPageID:    action.ParentPageID,
				AllowComments:   action.AllowComments,
				AllowReactions:  action.AllowReactions,
				ShowTOC:         action.ShowTOC,
				ScheduledFor:    action.ScheduledFor,
				Authors:         action.Authors,
				Topics:          action.Topics,
				Tags:            action.Tags,
				MetaDescription: action.MetaDescription,
				CanonicalURL:    action.CanonicalURL,
			}

			if err := bus.Dispatch(c, updateCmd); err != nil {
				return c.Failure(err)
			}

			getPage := &query.GetPageByID{ID: pageID}
			if err := bus.Dispatch(c, getPage); err != nil {
				return c.Failure(err)
			}

			c.Enqueue(tasks.NotifyPageSubscribers(pageID, c.User().ID))

			return c.Ok(getPage.Result)
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

func SavePageDraft() web.HandlerFunc {
	return func(c *web.Context) error {
		pageID, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		action := &actions.SavePageDraft{PageID: pageID}
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			saveCmd := &cmd.SavePageDraft{
				PageID:          pageID,
				Title:           action.Title,
				Slug:            action.Slug,
				Content:         action.Content,
				Excerpt:         action.Excerpt,
				BannerImageBKey: action.BannerImageBKey,
				MetaDescription: action.MetaDescription,
				ShowTOC:         action.ShowTOC,
				DraftData:       action.DraftData,
			}

			if err := bus.Dispatch(c, saveCmd); err != nil {
				return c.Failure(err)
			}

			return c.Ok(web.Map{})
		})
	}
}

func GetPageDraft() web.HandlerFunc {
	return func(c *web.Context) error {
		pageID, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		getDraft := &query.GetPageDraft{PageID: pageID, UserID: c.User().ID}
		if err := bus.Dispatch(c, getDraft); err != nil {
			return c.Failure(err)
		}

		return c.Ok(getDraft.Result)
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

func GetPageComments() web.HandlerFunc {
	return func(c *web.Context) error {
		pageID, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		getPage := &query.GetPageByID{ID: pageID}
		if err := bus.Dispatch(c, getPage); err != nil {
			return c.NotFound()
		}

		getComments := &query.GetCommentsByPage{Page: getPage.Result}
		if err := bus.Dispatch(c, getComments); err != nil {
			return c.Failure(err)
		}

		return c.Ok(getComments.Result)
	}
}

func AddPageComment() web.HandlerFunc {
	return func(c *web.Context) error {
		pageID, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		action := &actions.AddPageComment{PageID: pageID}
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			getPage := &query.GetPageByID{ID: pageID}
			if err := bus.Dispatch(c, getPage); err != nil {
				return c.NotFound()
			}

			if !getPage.Result.AllowComments {
				return c.BadRequest(web.Map{"message": "Comments are not allowed on this page"})
			}

			addCmd := &cmd.AddPageComment{
				Page:    getPage.Result,
				Content: action.Content,
			}

			if err := bus.Dispatch(c, addCmd); err != nil {
				return c.Failure(err)
			}

			return c.Ok(addCmd.Result)
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

func TogglePageCommentReaction() web.HandlerFunc {
	return func(c *web.Context) error {
		if c.User().IsMuted() {
			return c.BadRequest(web.Map{
				"message": "You are currently muted and cannot add reactions.",
			})
		}

		pageID, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		commentID, err := c.ParamAsInt("commentId")
		if err != nil {
			return c.NotFound()
		}

		reaction := c.Param("reaction")
		if reaction == "" {
			return c.BadRequest(web.Map{"message": "Reaction is required"})
		}

		return c.WithTransaction(func() error {
			getPage := &query.GetPageByID{ID: pageID}
			if err := bus.Dispatch(c, getPage); err != nil {
				return c.NotFound()
			}

			if !getPage.Result.AllowReactions {
				return c.BadRequest(web.Map{"message": "Reactions are not allowed on this page"})
			}

			getComment := &query.GetCommentByID{CommentID: commentID}
			if err := bus.Dispatch(c, getComment); err != nil {
				return c.NotFound()
			}

			toggleReaction := &cmd.ToggleCommentReaction{
				Comment: getComment.Result,
				Emoji:   reaction,
				User:    c.User(),
			}
			if err := bus.Dispatch(c, toggleReaction); err != nil {
				return c.Failure(err)
			}

			return c.Ok(web.Map{
				"added": toggleReaction.Result,
			})
		})
	}
}
