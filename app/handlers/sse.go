package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/sse"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func sseHandler(channel sse.Channel) web.HandlerFunc {
	return func(c *web.Context) error {
		c.Response.Header().Set("Content-Type", "text/event-stream")
		c.Response.Header().Set("Cache-Control", "no-cache")
		c.Response.Header().Set("Connection", "keep-alive")
		c.Response.Header().Set("X-Accel-Buffering", "no")

		rc := http.NewResponseController(c.Response.Writer)
		if err := rc.SetWriteDeadline(time.Time{}); err != nil {
			return c.Failure(fmt.Errorf("failed to disable write deadline: %w", err))
		}

		flusher, ok := c.Response.Writer.(http.Flusher)
		if !ok {
			return c.Failure(fmt.Errorf("streaming not supported"))
		}

		client := sse.NewClient(c.Tenant().ID, c.User().ID, c.User().Name, channel)
		hub := sse.GetHub()
		hub.Register(client)
		defer hub.Unregister(client)

		ctx := c.Request.Original().Context()

		fmt.Fprint(c.Response.Writer, ": ping\n\n")
		flusher.Flush()

		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case msg, ok := <-client.Send():
				if !ok {
					return nil
				}
				if _, err := fmt.Fprintf(c.Response.Writer, "data: %s\n\n", msg); err != nil {
					return nil
				}
				flusher.Flush()
			case <-ticker.C:
				if _, err := fmt.Fprint(c.Response.Writer, ": ping\n\n"); err != nil {
					return nil
				}
				flusher.Flush()
			case <-ctx.Done():
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
