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

		if err := updateViewerPresence(c, sse.ChannelQueue, postID); err != nil {
			return c.Failure(err)
		}

		return c.Ok(web.Map{})
	}
}

func StopViewingQueuePost() web.HandlerFunc {
	return func(c *web.Context) error {
		if err := updateViewerPresence(c, sse.ChannelQueue, 0); err != nil {
			return c.Failure(err)
		}

		return c.Ok(web.Map{})
	}
}
