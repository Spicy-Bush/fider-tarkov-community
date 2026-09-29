package postgres

import (
	"context"
	"fmt"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func BenchmarkPostRankingBackgroundWrites(b *testing.B) {
	dbx.Seed()
	b.Cleanup(dbx.Seed)
	fixture := exportSelectionFixture(b, 100000)
	if err := fixture.Commit(); err != nil {
		b.Fatal(err)
	}

	ctx := context.WithValue(context.Background(), app.TenantCtxKey, &entity.Tenant{ID: 1})
	ctx = context.WithValue(ctx, app.RequestCtxKey, web.Request{URL: &url.URL{Scheme: "http", Host: "localhost:3000"}})
	for _, writesPerSecond := range []int{0, 5, 20} {
		for _, useCache := range []bool{false, true} {
			name := "query"
			if useCache {
				name = "snapshot"
			}
			b.Run(fmt.Sprintf("%s/%d-writes", name, writesPerSecond), func(b *testing.B) {
				stop := make(chan struct{})
				done := make(chan struct{})
				var writes atomic.Int64
				go func() {
					defer close(done)
					if writesPerSecond == 0 {
						return
					}

					ticker := time.NewTicker(time.Second / time.Duration(writesPerSecond))
					defer ticker.Stop()
					for {
						select {
						case <-stop:
							return
						case <-ticker.C:
							_, err := dbx.Connection().Exec("UPDATE posts SET upvotes = upvotes + 1 WHERE tenant_id = 1 AND number = 1001")
							if err != nil {
								b.Error(err)
								return
							}
							writes.Add(1)
						}
					}
				}()
				defer func() {
					close(stop)
					<-done
				}()

				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					posts := &query.SearchPosts{View: "trending", Limit: "15"}
					var err error
					if useCache {
						err = searchPosts(ctx, posts)
					} else {
						err = loadSearchPosts(ctx, posts)
					}
					if err != nil {
						b.Fatal(err)
					}
				}
				b.StopTimer()
				b.ReportMetric(float64(writes.Load()), "writes")
			})
		}
	}
}
