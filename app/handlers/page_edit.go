package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/pagedoc"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/sse"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"github.com/Spicy-Bush/fider-tarkov-community/app/tasks"
)

func OpenPageEdit() web.HandlerFunc {
	return func(c *web.Context) error {
		var input struct {
			PageID       int    `json:"pageId"`
			SubmissionID string `json:"submissionId"`
		}
		if err := c.Bind(&input); err != nil {
			return c.HandleValidation(validate.Failed("Invalid Page editing request."))
		}
		if input.PageID < 0 || (input.PageID == 0 && !validate.ValidSubmissionID(input.SubmissionID)) {
			return c.HandleValidation(validate.Failed("Invalid Page editing identity."))
		}

		open := &cmd.OpenPageEdit{PageID: input.PageID, SubmissionID: input.SubmissionID}
		if err := bus.Dispatch(c, open); err != nil {
			return c.Failure(err)
		}
		return c.Ok(open.Result)
	}
}

func GetPageEdit() web.HandlerFunc {
	return func(c *web.Context) error {
		id, err := c.ParamAsInt("id")
		if err != nil || id <= 0 {
			return c.NotFound()
		}

		read := &query.GetPageEdit{PageID: id}
		if err := bus.Dispatch(c, read); err != nil {
			return c.Failure(err)
		}
		return c.Ok(read.Result)
	}
}

func SyncPageEdit() web.HandlerFunc {
	return func(c *web.Context) error {
		id, err := c.ParamAsInt("id")
		if err != nil || id <= 0 {
			return c.NotFound()
		}
		if len(c.Request.Body) > 2*pagedoc.MaxStateBytes {
			return c.HandleValidation(validate.Failed("The Page update is too large."))
		}

		var input struct {
			Update      []byte `json:"update"`
			StateVector []byte `json:"stateVector"`
		}
		if err := c.Bind(&input); err != nil {
			return c.HandleValidation(validate.Failed("Invalid Page update."))
		}

		sync := &cmd.SyncPageEdit{PageID: id, Update: input.Update, StateVector: input.StateVector}
		if err := bus.Dispatch(c, sync); err != nil {
			return c.Failure(err)
		}
		sse.PageEditors.Broadcast(c.Tenant().ID, id, sse.PageEvent{Type: "update", Update: sync.AcceptedUpdate})
		return c.Ok(sync.Result)
	}
}

func PublishPageEdit() web.HandlerFunc {
	return func(c *web.Context) error {
		id, err := c.ParamAsInt("id")
		if err != nil || id <= 0 {
			return c.NotFound()
		}

		var input struct {
			Status       entity.PageStatus `json:"status"`
			SubmissionID string            `json:"submissionId"`
		}
		if err := c.Bind(&input); err != nil {
			return c.HandleValidation(validate.Failed("Invalid Page publication request."))
		}

		publish := &cmd.PublishPageEdit{PageID: id, Status: input.Status, SubmissionID: input.SubmissionID}
		if err := bus.Dispatch(c, publish); err != nil {
			return c.Failure(err)
		}
		sse.PageEditors.Broadcast(c.Tenant().ID, id, sse.PageEvent{Type: "sync"})
		if !publish.Replayed {
			c.Enqueue(tasks.NotifyPageSubscribers(id, c.User().ID))
		}
		return c.Ok(publish.Result)
	}
}

func UploadPageEditBanner() web.HandlerFunc {
	return func(c *web.Context) error {
		id, err := c.ParamAsInt("id")
		if err != nil || id <= 0 {
			return c.NotFound()
		}

		var input struct {
			SubmissionID string           `json:"submissionId"`
			Image        *dto.ImageUpload `json:"image"`
		}
		if err := c.Bind(&input); err != nil {
			return c.HandleValidation(validate.Failed("Invalid Page banner request."))
		}
		if !validate.ValidSubmissionID(input.SubmissionID) || input.Image == nil || input.Image.Upload == nil {
			return c.HandleValidation(validate.Failed("Choose an image to upload."))
		}

		upload := &cmd.UploadPageEditBanner{PageID: id, SubmissionID: input.SubmissionID, Image: input.Image}
		if err := bus.Dispatch(c, upload); err != nil {
			return c.Failure(err)
		}
		return c.Ok(web.Map{"bkey": upload.Result})
	}
}

func PageEditCursor() web.HandlerFunc {
	return func(c *web.Context) error {
		id, err := c.ParamAsInt("id")
		if err != nil || id <= 0 {
			return c.NotFound()
		}
		if len(c.Request.Body) > 2048 {
			return c.HandleValidation(validate.Failed("Invalid cursor position."))
		}

		var input struct {
			ClientID uint64            `json:"clientId"`
			Field    string            `json:"field"`
			Anchor   *sse.PagePosition `json:"anchor"`
			Head     *sse.PagePosition `json:"head"`
		}
		if err := c.Bind(&input); err != nil || input.ClientID > 1<<53-1 || input.Field != "content" ||
			!input.Anchor.Valid() || !input.Head.Valid() || (input.Anchor == nil) != (input.Head == nil) {
			return c.HandleValidation(validate.Failed("Invalid cursor position."))
		}

		sse.PageEditors.Cursor(c.Tenant().ID, id, c.User().ID, input.ClientID, input.Anchor, input.Head)
		return c.NoContent(http.StatusNoContent)
	}
}

func PageEditEvents() web.HandlerFunc {
	return func(c *web.Context) error {
		id, err := c.ParamAsInt("id")
		if err != nil || id <= 0 {
			return c.NotFound()
		}
		clientID, err := strconv.ParseUint(c.QueryParam("clientId"), 10, 53)
		if err != nil {
			return c.HandleValidation(validate.Failed("Invalid editor identity."))
		}

		viewer := query.RealtimeViewer{TenantID: c.Tenant().ID, UserID: c.User().ID, PageID: id}
		access, err := checkRealtimeAccess(c, viewer)
		if err != nil {
			return c.Failure(err)
		}
		if !access.Pages {
			return c.HandleValidation(validate.Unauthorized())
		}

		client, registered := sse.PageEditors.Register(c.Tenant().ID, id, clientID, sse.PageEditor{
			ID: c.User().ID, Name: c.User().Name,
		})
		if !registered {
			return c.HandleValidation(validate.Failed("This editor identity is already connected."))
		}
		defer sse.PageEditors.Unregister(client)

		c.Response.Header().Set("Content-Type", "text/event-stream")
		c.Response.Header().Set("Cache-Control", "no-cache")
		c.Response.Header().Set("X-Accel-Buffering", "no")
		controller := http.NewResponseController(c.Response.Writer)
		if err := controller.SetWriteDeadline(time.Now().Add(15 * time.Second)); err != nil {
			return err
		}
		if _, err := fmt.Fprint(&c.Response, ": connected\n\n"); err != nil {
			return nil
		}
		if err := controller.Flush(); err != nil {
			return nil
		}

		ctx := c.Request.Original().Context()
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			var message []byte
			select {
			case data, open := <-client.Send():
				if !open {
					return nil
				}
				message = data
			case <-ticker.C:
				message = []byte(`{"type":"sync"}`)
			case <-ctx.Done():
				return nil
			}

			checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			access, err := checkRealtimeAccess(checkCtx, viewer)
			cancel()
			if err != nil || !access.Pages {
				return err
			}

			if err := controller.SetWriteDeadline(time.Now().Add(15 * time.Second)); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(&c.Response, "data: %s\n\n", message); err != nil {
				return nil
			}
			if err := controller.Flush(); err != nil {
				return nil
			}
		}
	}
}
