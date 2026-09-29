package postgres

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/sse"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func BenchmarkRealtimeDelivery(b *testing.B) {
	for _, viewers := range []int{1, 64, 1000} {
		b.Run(fmt.Sprintf("viewers_%d", viewers), func(b *testing.B) {
			dbx.Seed()
			_, err := dbx.Connection().Exec(`
				INSERT INTO users (id, tenant_id, name, email, role, status, created_at, avatar_type, avatar_bkey)
				SELECT 10000+number, 1, 'Viewer ' || number, 'viewer-' || number || '@example.test', $2, $3, NOW(), 1, ''
				FROM generate_series(1, $1) number
			`, viewers, enum.RoleModerator, enum.UserActive)
			if err != nil {
				b.Fatal(err)
			}
			bus.Reset()
			var queries atomic.Int64
			bus.AddHandler(func(ctx context.Context, q *query.GetRealtimeAccess) error {
				queries.Add(1)
				return getRealtimeAccess(ctx, q)
			})

			tenant := &entity.Tenant{ID: 1, Status: enum.TenantActive}
			engine := web.New()
			var nextUser atomic.Int64
			var serving sync.WaitGroup
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				serving.Add(1)
				defer serving.Done()
				ctx, err := web.NewContext(engine, request, writer, nil)
				if err == nil {
					user := &entity.User{
						ID:     10000 + int(nextUser.Add(1)),
						Role:   enum.RoleModerator,
						Status: enum.UserActive,
						Tenant: tenant,
					}
					ctx.SetTenant(tenant)
					ctx.SetUser(user)
					err = handlers.QueueSSE()(ctx)
				}
				if err != nil && request.Context().Err() == nil {
					b.Error(err)
				}
			}))
			defer server.Close()

			responses := make([]*http.Response, viewers)
			readers := make([]*bufio.Reader, viewers)
			defer func() {
				for _, response := range responses {
					if response != nil {
						response.Body.Close()
					}
				}
				serving.Wait()
			}()
			client := &http.Client{Timeout: time.Minute}
			for index := range viewers {
				response, err := client.Get(server.URL)
				if err != nil {
					b.Fatal(err)
				}
				responses[index] = response
				readers[index] = bufio.NewReader(response.Body)
				line, err := readers[index].ReadString('\n')
				if err != nil {
					b.Fatal(err)
				}
				var greeting struct {
					Type    string `json:"type"`
					Payload struct {
						ConnectionID string `json:"connectionId"`
					} `json:"payload"`
				}
				if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &greeting); err != nil ||
					greeting.Type != sse.MsgConnectionReady || len(greeting.Payload.ConnectionID) != 32 {
					b.Fatalf("stream greeting: %q, %v", line, err)
				}
				if line, err := readers[index].ReadString('\n'); err != nil || line != "\n" {
					b.Fatalf("stream greeting separator: %q, %v", line, err)
				}
			}

			queries.Store(0)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				for message := range 3 {
					sse.GetHub().BroadcastToTenant(tenant.ID, sse.MsgQueuePostNew, sse.QueueEventPayload{PostID: message + 1})
				}

				var received sync.WaitGroup
				for _, reader := range readers {
					received.Add(1)
					go func() {
						defer received.Done()
						for message := range 3 {
							line, err := reader.ReadString('\n')
							if err != nil || !strings.Contains(line, fmt.Sprintf(`"postId":%d`, message+1)) {
								b.Errorf("stream event: %q, %v", line, err)
								return
							}
							_, _ = reader.ReadString('\n')
						}
					}()
				}
				received.Wait()
			}
			b.StopTimer()
			b.ReportMetric(float64(queries.Load())/float64(b.N), "queries/op")
		})
	}
}
