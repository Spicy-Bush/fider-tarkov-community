package handlers_test

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/sse"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestEventStreamRechecksAccessAndRecovers(t *testing.T) {
	for _, test := range []struct {
		name    string
		handler web.HandlerFunc
		message string
	}{
		{name: "queue", handler: handlers.QueueSSE(), message: sse.MsgQueuePostNew},
		{name: "reports", handler: handlers.ReportsSSE(), message: sse.MsgReportsChanged},
	} {
		t.Run(test.name, func(t *testing.T) {
			var allowed atomic.Bool
			var unavailable atomic.Bool
			allowed.Store(true)
			lookupError := errors.New("permission storage unavailable")

			bus.AddHandler(func(ctx context.Context, q *query.GetRealtimeAccess) error {
				if unavailable.Load() {
					return lookupError
				}

				q.Result = make([]query.RealtimeAccess, len(q.Viewers))
				for index := range q.Result {
					q.Result[index] = query.RealtimeAccess{Reports: allowed.Load(), Queue: allowed.Load()}
				}
				return nil
			})

			tenant := &entity.Tenant{ID: 910001, Status: enum.TenantActive}
			user := &entity.User{ID: 1, Name: "Moderator", Role: enum.RoleModerator, Status: enum.UserActive, Tenant: tenant}
			finished := make(chan error, 4)
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				ctx, err := web.NewContext(web.New(), request, writer, nil)
				if err != nil {
					finished <- err
					return
				}

				ctx.SetTenant(tenant)
				ctx.SetUser(user)
				finished <- test.handler(ctx)
			}))
			defer server.Close()

			connect := func() (*http.Response, *bufio.Reader) {
				t.Helper()
				client := &http.Client{Timeout: 3 * time.Second}
				response, err := client.Get(server.URL)
				if err != nil {
					t.Fatal(err)
				}
				if response.StatusCode != http.StatusOK {
					response.Body.Close()
					t.Fatalf("stream returned %d", response.StatusCode)
				}

				reader := bufio.NewReader(response.Body)
				line, err := reader.ReadString('\n')
				if err != nil || !strings.Contains(line, sse.MsgConnectionReady) || !strings.Contains(line, "connectionId") {
					response.Body.Close()
					t.Fatalf("missing stream identity: %q, %v", line, err)
				}
				if line, err := reader.ReadString('\n'); err != nil || line != "\n" {
					t.Fatalf("invalid greeting boundary: %q, %v", line, err)
				}

				return response, reader
			}

			for _, failure := range []string{"revoked", "database unavailable"} {
				response, reader := connect()
				sse.GetHub().BroadcastToTenant(tenant.ID, test.message, sse.QueueEventPayload{PostID: 17})
				line, err := reader.ReadString('\n')
				if err != nil || !strings.Contains(line, test.message) {
					t.Fatalf("authorized stream lost event: %q, %v", line, err)
				}
				_, _ = reader.ReadString('\n')

				if failure == "revoked" {
					allowed.Store(false)
				} else {
					unavailable.Store(true)
				}

				sse.GetHub().BroadcastToTenant(tenant.ID, test.message, sse.QueueEventPayload{PostID: 18})
				remaining, readErr := io.ReadAll(reader)
				response.Body.Close()
				if readErr != nil || len(remaining) != 0 {
					t.Fatalf("%s stream retained access: body=%q error=%v", failure, remaining, readErr)
				}

				handlerErr := <-finished
				if failure == "database unavailable" && !errors.Is(handlerErr, lookupError) {
					t.Fatalf("lost permission lookup failure: %v", handlerErr)
				}

				allowed.Store(true)
				unavailable.Store(false)
			}
		})
	}
}
