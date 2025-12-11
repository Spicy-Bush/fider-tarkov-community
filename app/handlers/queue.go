package handlers

import (
	"net/http"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/sse"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func PostQueuePage() web.HandlerFunc {
	return func(c *web.Context) error {
		getAllTags := &query.GetAllTags{}
		if err := bus.Dispatch(c, getAllTags); err != nil {
			return c.Failure(err)
		}

		return c.Page(http.StatusOK, web.Props{
			Page:  "Administration/pages/PostQueue.page",
			Title: "Post Queue - Site Settings",
			Data: web.Map{
				"tags":    getAllTags.Result,
				"viewers": sse.GetHub().GetAllQueueViewers(c.Tenant().ID),
			},
		})
	}
}

func QueuePostHeartbeat() web.HandlerFunc {
	return func(c *web.Context) error {
		postID, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		sse.GetHub().UpdateQueuePresence(c.Tenant().ID, c.User().ID, c.User().Name, postID)

		return c.Ok(web.Map{})
	}
}

func StopViewingQueuePost() web.HandlerFunc {
	return func(c *web.Context) error {
		sse.GetHub().UpdateQueuePresence(c.Tenant().ID, c.User().ID, c.User().Name, 0)

		return c.Ok(web.Map{})
	}
}

