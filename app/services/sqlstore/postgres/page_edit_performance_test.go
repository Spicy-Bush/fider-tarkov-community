package postgres_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/pagedoc"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"github.com/reearth/ygo/crdt"
)

func BenchmarkPageEditSyncHTTP(b *testing.B) {
	for _, workload := range []struct {
		name      string
		markup    string
		fragments int
	}{
		{name: "plain"},
		{name: "image", markup: " ![image](/static/images/files/page-benchmark.webp)"},
		{name: "encoded_image", markup: " ![image](%2Fstatic%2Fimages%2Ffiles%2Fpage-benchmark.webp)"},
		{name: "fragmented", fragments: 500},
	} {
		b.Run(workload.name, func(b *testing.B) {
			f := newPostWorkflow(b)
			page := &cmd.CreatePage{
				Title:      "Large collaborative Page",
				Content:    strings.Repeat("a", 45_000-len(workload.markup)) + workload.markup,
				Status:     entity.PageStatusPublished,
				Visibility: entity.PageVisibilityPublic,
			}
			if err := bus.Dispatch(f.ctx, page); err != nil {
				b.Fatal(err)
			}
			open := &cmd.OpenPageEdit{PageID: page.Result.ID}
			if err := bus.Dispatch(f.ctx, open); err != nil {
				b.Fatal(err)
			}
			document := crdt.New()
			if err := crdt.ApplyUpdateV1(document, open.Result.State, nil); err != nil {
				b.Fatal(err)
			}
			initial := document.StateVector()
			for index := range workload.fragments {
				document.Transact(func(transaction *crdt.Transaction) {
					text := transaction.GetText("content")
					position := index * 83
					text.Delete(transaction, position, 1)
					text.Insert(transaction, position, "b", nil)
				})
			}
			if workload.fragments > 0 {
				update := crdt.EncodeStateAsUpdateV1(document, initial)
				if err := bus.Dispatch(f.ctx, &cmd.SyncPageEdit{PageID: page.Result.ID, Update: update}); err != nil {
					b.Fatal(err)
				}
			}

			params := web.StringMap{"id": fmt.Sprint(page.Result.ID)}
			b.ReportAllocs()
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				b.StopTimer()
				vector := document.StateVector()
				encodedVector := crdt.EncodeStateVectorV1(document)
				document.Transact(func(transaction *crdt.Transaction) {
					text := transaction.GetText("content")
					text.Delete(transaction, 0, 1)
					text.Insert(transaction, 0, string(rune('c'+index%2)), nil)
				})
				body, err := json.Marshal(map[string]any{
					"update":      crdt.EncodeStateAsUpdateV1(document, vector),
					"stateVector": encodedVector,
				})
				if err != nil {
					b.Fatal(err)
				}

				b.StartTimer()
				response, err := f.requestWithParams(handlers.SyncPageEdit(), http.MethodPost,
					"/api/pages/draft", string(body), params)
				if err != nil || response.Code != http.StatusOK {
					b.Fatalf("sync status=%d error=%v body=%s", response.Code, err, response.Body)
				}
			}
			b.StopTimer()

			current := &query.GetPageEdit{PageID: page.Result.ID}
			if err := bus.Dispatch(f.ctx, current); err != nil {
				b.Fatal(err)
			}
			materialized, err := pagedoc.Read(current.Result.State)
			if err != nil || materialized.Content != document.GetText("content").ToString() {
				b.Fatalf("measured edits did not persist exactly: %v", err)
			}
		})
	}
}
