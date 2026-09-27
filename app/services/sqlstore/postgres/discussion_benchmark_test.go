package postgres_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/api"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func BenchmarkDiscussionReadHTTP(b *testing.B) {
	f := newPostWorkflow(b)
	post := &cmd.AddNewPost{Title: "Large discussion", Description: "Comment ranking and paging"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		b.Fatal(err)
	}

	for first := 1; first <= 5000; first += 50 {
		_, err := dbx.Connection().Exec(`
            WITH roots AS (
                INSERT INTO comments (tenant_id, post_id, user_id, content, created_at)
                SELECT $1, $2, $3, 'Root comment ' || n, NOW() - n * INTERVAL '1 second'
                FROM generate_series($4::integer, $5::integer) n
                RETURNING id, tenant_id, post_id, user_id, created_at
            )
            INSERT INTO comments (tenant_id, post_id, parent_id, user_id, content, created_at)
            SELECT roots.tenant_id, roots.post_id, roots.id, roots.user_id,
                   'Reply ' || n, roots.created_at + n * INTERVAL '1 millisecond'
            FROM roots, generate_series(1, 9) n
        `, f.tenant.ID, post.Result.ID, f.user.ID, first, first+49)
		if err != nil {
			b.Fatal(err)
		}
	}

	_, err := dbx.Connection().Exec(`
        INSERT INTO reactions (comment_id, user_id, emoji, created_on)
        SELECT c.id, n, CASE WHEN c.id % 2 = 0 THEN '👍' ELSE '👎' END, NOW()
        FROM comments c, generate_series(1, 3) n
        WHERE c.post_id = $1 AND c.parent_id IS NULL AND n <= c.id % 4
    `, post.Result.ID)
	if err != nil {
		b.Fatal(err)
	}

	if _, err := dbx.Connection().Exec("ANALYZE comments; ANALYZE reactions;"); err != nil {
		b.Fatal(err)
	}

	params := web.StringMap{"number": fmt.Sprint(post.Result.Number)}
	for _, order := range []string{"liked", "disliked", "replies", "latest"} {
		b.Run(order, func(b *testing.B) {
			path := "/api/posts/" + params["number"] + "/comments?sort=" + order
			response, err := f.requestWithParams(api.ListDiscussion(), http.MethodGet, path, "", params)
			if err != nil || response.Code != http.StatusOK {
				b.Fatalf("read failed: %v, HTTP %d, %s", err, response.Code, response.Body)
			}

			var page entity.DiscussionPage
			if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
				b.Fatal(err)
			}
			if len(page.Comments) != 25 || page.Next == "" {
				b.Fatal("large discussion did not return a bounded page and continuation")
			}

			b.SetBytes(int64(response.Body.Len()))
			b.ReportAllocs()
			b.ResetTimer()

			for iteration := 0; iteration < b.N; iteration++ {
				response, err := f.requestWithParams(api.ListDiscussion(), http.MethodGet, path, "", params)
				if err != nil || response.Code != http.StatusOK {
					b.Fatalf("read failed: %v, HTTP %d", err, response.Code)
				}
			}
		})
	}
}

func BenchmarkDiscussionCreateAndRecoverHTTP(b *testing.B) {
	f := newPostWorkflow(b)
	post := &cmd.AddNewPost{Title: "Recoverable discussion", Description: "Comment creation and receipt recovery"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for iteration := 0; iteration < b.N; iteration++ {
		body := fmt.Sprintf(`{"content":"A comment whose acknowledgement may be lost.","submissionId":"benchmark-%d"}`, iteration)
		created, err := f.request(api.CreateDiscussionComment(), http.MethodPost, post.Result.Number, body)
		if err != nil || created.Code != http.StatusOK {
			b.Fatalf("creation failed: %v, HTTP %d, %s", err, created.Code, created.Body)
		}

		recovered, err := f.request(api.CreateDiscussionComment(), http.MethodPost, post.Result.Number, body)
		if err != nil || recovered.Code != http.StatusOK || recovered.Body.String() != created.Body.String() {
			b.Fatalf("receipt recovery differs: %v, HTTP %d, %s", err, recovered.Code, recovered.Body)
		}
	}

	b.StopTimer()
	if count := workflowCount(b, "SELECT COUNT(*) FROM comments"); count != b.N {
		b.Fatalf("%d submissions persisted %d comments", b.N, count)
	}
}

func BenchmarkDiscussionDeepChainHTTP(b *testing.B) {
	f := newPostWorkflow(b)
	post := &cmd.AddNewPost{Title: "Deep discussion", Description: "A chain of 5,000 replies"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		b.Fatal(err)
	}

	var parentID *int
	for batch := 0; batch < 50; batch++ {
		var lastID int
		err := dbx.Connection().QueryRow(`
            WITH nodes AS MATERIALIZED (
                SELECT nextval(pg_get_serial_sequence('comments', 'id'))::integer AS id, n
                FROM generate_series(1, 100) n
            ), inserted AS (
                INSERT INTO comments (id, tenant_id, post_id, parent_id, user_id, content, created_at)
                SELECT id, $1, $2, COALESCE(LAG(id) OVER (ORDER BY n), $4), $3, 'Deep reply ' || id, NOW()
                FROM nodes ORDER BY n
                RETURNING id
            )
            SELECT MAX(id) FROM inserted
        `, f.tenant.ID, post.Result.ID, f.user.ID, parentID).Scan(&lastID)
		if err != nil {
			b.Fatal(err)
		}

		parentID = &lastID
	}

	if _, err := dbx.Connection().Exec("ANALYZE comments"); err != nil {
		b.Fatal(err)
	}

	params := web.StringMap{"number": fmt.Sprint(post.Result.Number)}
	path := "/api/posts/" + params["number"] + "/comments?sort=replies"
	response, err := f.requestWithParams(api.ListDiscussion(), http.MethodGet, path, "", params)
	if err != nil || response.Code != http.StatusOK {
		b.Fatalf("read failed: %v, HTTP %d, %s", err, response.Code, response.Body)
	}

	var page entity.DiscussionPage
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		b.Fatal(err)
	}
	if len(page.Comments) != 1 || page.Next != "" || !page.Comments[0].HasReplies {
		b.Fatal("deep discussion did not return its single root")
	}

	b.ReportAllocs()
	b.ResetTimer()

	for iteration := 0; iteration < b.N; iteration++ {
		response, err := f.requestWithParams(api.ListDiscussion(), http.MethodGet, path, "", params)
		if err != nil || response.Code != http.StatusOK {
			b.Fatalf("read failed: %v, HTTP %d", err, response.Code)
		}
	}
}
