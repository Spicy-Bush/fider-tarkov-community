package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/sse"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func updateViewerPresence(c *web.Context, channel sse.Channel, itemID int) error {
	var input struct {
		ConnectionID string `json:"connectionId"`
	}
	if err := c.Bind(&input); err != nil || len(input.ConnectionID) != 32 {
		return validate.Failed("Invalid connection identity.")
	}
	if !sse.GetHub().UpdatePresence(c.Tenant().ID, c.User().ID, input.ConnectionID, channel, itemID) {
		return app.ErrNotFound
	}
	return nil
}

func sseHandler(channel sse.Channel) web.HandlerFunc {
	return func(c *web.Context) error {
		c.Response.Header().Set("Content-Type", "text/event-stream")
		c.Response.Header().Set("Cache-Control", "no-cache")
		c.Response.Header().Set("Connection", "keep-alive")
		c.Response.Header().Set("X-Accel-Buffering", "no")

		rc := http.NewResponseController(c.Response.Writer)
		if err := rc.SetWriteDeadline(time.Now().Add(15 * time.Second)); err != nil {
			return fmt.Errorf("failed to set stream write deadline: %w", err)
		}

		client := sse.NewClient(c.Tenant().ID, c.User().ID, c.User().Name, channel)
		hub := sse.GetHub()
		hub.Register(client)
		defer hub.Unregister(client)

		ctx := c.Request.Original().Context()

		greeting, _ := json.Marshal(sse.Message{
			Type:    sse.MsgConnectionReady,
			Payload: web.Map{"connectionId": client.ID()},
		})
		if _, err := fmt.Fprintf(&c.Response, "data: %s\n\n", greeting); err != nil {
			return nil
		}
		if err := rc.Flush(); err != nil {
			return nil
		}

		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			var message []byte
			select {
			case msg, ok := <-client.Send():
				if !ok {
					return nil
				}
				message = msg
			case <-ticker.C:
			case <-ctx.Done():
				return nil
			}

			checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			access, err := checkRealtimeAccess(checkCtx, query.RealtimeViewer{
				TenantID: c.Tenant().ID,
				UserID:   c.User().ID,
			})
			cancel()
			if err != nil {
				return err
			}
			if (channel == sse.ChannelQueue && !access.Queue) || (channel == sse.ChannelReports && !access.Reports) {
				return nil
			}

			if err := rc.SetWriteDeadline(time.Now().Add(15 * time.Second)); err != nil {
				return err
			}
			if message == nil {
				_, err = fmt.Fprint(&c.Response, ": ping\n\n")
			} else {
				_, err = fmt.Fprintf(&c.Response, "data: %s\n\n", message)
			}
			if err != nil {
				return nil
			}
			if err := rc.Flush(); err != nil {
				return nil
			}
		}
	}
}

func ReportsSSE() web.HandlerFunc {
	return sseHandler(sse.ChannelReports)
}

func QueueSSE() web.HandlerFunc {
	return sseHandler(sse.ChannelQueue)
}
