package handlers

import (
	"net/http"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/markdown"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func ViewPage() web.HandlerFunc {
	return func(c *web.Context) error {
		slug := c.Param("slug")

		getPage := &query.GetPageBySlug{Slug: slug}
		if err := bus.Dispatch(c, getPage); err != nil {
			return c.NotFound()
		}

		page := getPage.Result

		if !page.CanView(c.User()) {
			if c.User() == nil {
				return c.Redirect("/signin?redirect=" + c.Request.URL.Path)
			}
			return c.Forbidden()
		}

		isSubscribed := &query.UserSubscribedToPage{PageID: page.ID}
		if c.User() != nil {
			bus.Dispatch(c, isSubscribed)
		}

		metaDesc := page.MetaDescription
		if metaDesc == "" {
			metaDesc = markdown.PlainText(page.Excerpt)
			if metaDesc == "" && len(page.Content) > 200 {
				metaDesc = markdown.PlainText(page.Content[:200])
			}
		}

		return c.Page(http.StatusOK, web.Props{
			Page:        "Page/ViewPage.page",
			Title:       page.Title,
			Description: metaDesc,
			Data: web.Map{
				"page":       page,
				"subscribed": isSubscribed.Result,
			},
		})
	}
}

func ListPagesPage() web.HandlerFunc {
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
			listPages.Tags = []string{tags}
		}

		getTopics := &query.GetPageTopics{}
		getTags := &query.GetPageTags{}

		if err := bus.Dispatch(c, listPages, getTopics, getTags); err != nil {
			return c.Failure(err)
		}

		return c.Page(http.StatusOK, web.Props{
			Page:  "Page/ListPages.page",
			Title: "Pages",
			Data: web.Map{
				"pages":       listPages.Result,
				"topics":      getTopics.Result,
				"tags":        getTags.Result,
				"totalCount":  listPages.TotalCount,
				"totalPages":  (listPages.TotalCount + 19) / 20,
				"page":        page,
				"query":       c.QueryParam("q"),
				"topicFilter": c.QueryParam("topics"),
				"tagFilter":   c.QueryParam("tags"),
			},
		})
	}
}

func ManagePages() web.HandlerFunc {
	return func(c *web.Context) error {
		listPages := &query.ListPages{
			Status: []entity.PageStatus{
				entity.PageStatusDraft,
				entity.PageStatusPublished,
				entity.PageStatusUnpublished,
				entity.PageStatusScheduled,
			},
			Limit:  100,
			Offset: 0,
		}

		getTopics := &query.GetPageTopics{}
		getTags := &query.GetPageTags{}

		if err := bus.Dispatch(c, listPages, getTopics, getTags); err != nil {
			return c.Failure(err)
		}

		return c.Page(http.StatusOK, web.Props{
			Page:  "Administration/pages/ManagePages.page",
			Title: "Manage Pages · Site Settings",
			Data: web.Map{
				"pages":  listPages.Result,
				"topics": getTopics.Result,
				"tags":   getTags.Result,
			},
		})
	}
}

func EditPagePage() web.HandlerFunc {
	return func(c *web.Context) error {
		pageID, _ := c.ParamAsInt("id")
		var page *entity.Page
		var draft *entity.PageDraft

		if pageID > 0 {
			getPage := &query.GetPageByID{ID: pageID}
			if err := bus.Dispatch(c, getPage); err != nil {
				return c.NotFound()
			}
			page = getPage.Result

			getDraft := &query.GetPageDraft{PageID: pageID, UserID: c.User().ID}
			bus.Dispatch(c, getDraft)
			draft = getDraft.Result
		}

		getTopics := &query.GetPageTopics{}
		getTags := &query.GetPageTags{}
		getUsers := &query.GetAllUsers{}
		bus.Dispatch(c, getTopics, getTags, getUsers)

		title := "Create Page"
		if page != nil {
			title = "Edit " + page.Title
		}

		return c.Page(http.StatusOK, web.Props{
			Page:  "Administration/pages/EditPage.page",
			Title: title,
			Data: web.Map{
				"page":   page,
				"draft":  draft,
				"topics": getTopics.Result,
				"tags":   getTags.Result,
				"roles":  []string{"visitor", "collaborator", "moderator", "administrator", "helper"},
				"users":  getUsers.Result,
			},
		})
	}
}
