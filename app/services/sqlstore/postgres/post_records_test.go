package postgres_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/api"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/lib/pq"
)

func TestPostRecordSelection(t *testing.T) {
	f := newPostWorkflow(t)
	transaction, err := mediaFixtureTransaction(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer transaction.Rollback()

	rows, err := transaction.Query(`
		INSERT INTO posts (tenant_id, user_id, title, slug, description, status, created_at)
		SELECT $1, $2, 'Selected ' || status, 'selected-' || status, 'Body', status, NOW()
		FROM generate_series(0, 7) status
		RETURNING id
	`, f.tenant.ID, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}

	var ids []string
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}

		ids = append(ids, fmt.Sprint(id))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if err := transaction.Commit(); err != nil {
		t.Fatal(err)
	}

	read := func(selection string) []entity.Post {
		t.Helper()
		path := "/api/posts?view=newest&statuses=open,started,completed,declined,planned,duplicate,deleted,archived&ids=" + selection
		response, err := f.requestWithParams(api.SearchPosts(), http.MethodGet, path, "", nil)
		if err != nil || response.Code != http.StatusOK {
			t.Fatalf("read selection: %v, HTTP %d, %s", err, response.Code, response.Body)
		}
		if response.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("personalized records can be cached")
		}

		var posts []entity.Post
		if err := json.Unmarshal(response.Body.Bytes(), &posts); err != nil {
			t.Fatal(err)
		}

		return posts
	}

	posts := read(strings.Join(ids, ","))
	var statuses []enum.PostStatus
	for _, post := range posts {
		statuses = append(statuses, post.Status)
	}
	slices.Sort(statuses)
	if !slices.Equal(statuses, []enum.PostStatus{0, 1, 2, 3, 4, 5, 7}) {
		t.Fatalf("selection lost visible statuses or exposed deleted posts: %v", statuses)
	}

	if posts := read(ids[0]); len(posts) != 1 || fmt.Sprint(posts[0].ID) != ids[0] {
		t.Fatalf("selection returned unrelated posts: %+v", posts)
	}

	invalidSelections := []string{
		"", "0", "-1", "2147483648", "1,x", "1%29%3BDROP",
		"1&ids=2",
		strings.Repeat(ids[0]+",", 50) + ids[0],
	}
	for _, selection := range invalidSelections {
		response, err := f.requestWithParams(api.SearchPosts(), http.MethodGet, "/api/posts?ids="+selection, "", nil)
		if err != nil || response.Code != http.StatusBadRequest {
			t.Fatalf("invalid selection %q: %v, HTTP %d", selection, err, response.Code)
		}
	}

	if _, err := mediaFixtureSQL("UPDATE posts SET moderation_pending = TRUE WHERE id = $1", ids[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := mediaFixtureSQL(`
		UPDATE posts SET original_id = $1, response = 'Duplicate', response_date = NOW(), response_user_id = 1
		WHERE id = $2
	`, ids[0], ids[5]); err != nil {
		t.Fatal(err)
	}

	for _, role := range []enum.Role{0, enum.RoleVisitor, enum.RoleHelper, enum.RoleModerator, enum.RoleCollaborator, enum.RoleAdministrator} {
		f.user = &entity.User{ID: 2, Role: role, Status: enum.UserActive}
		if role == 0 {
			f.user = nil
		}

		posts := read(ids[0])
		staff := role == enum.RoleModerator || role == enum.RoleCollaborator || role == enum.RoleAdministrator
		if (len(posts) == 1) != staff {
			t.Fatalf("hidden post visibility for role %v: %+v", role, posts)
		}

		duplicates := read(ids[5])
		if len(duplicates) != 1 || duplicates[0].Response == nil {
			t.Fatalf("missing duplicate for role %v: %+v", role, duplicates)
		}
		if (duplicates[0].Response.Original != nil) != staff {
			t.Errorf("hidden original visibility for role %v: %+v", role, duplicates[0].Response.Original)
		}
	}

	f.user = &entity.User{ID: 1, Role: enum.RoleVisitor, Status: enum.UserActive}
	if posts := read(ids[0]); len(posts) != 1 || posts[0].ModerationPending || posts[0].ModerationData != "" {
		t.Fatalf("author learned the hidden status: %+v", posts)
	}
	if posts := read(ids[5]); len(posts) != 1 || posts[0].Response.Original == nil {
		t.Fatalf("author cannot see their own original post: %+v", posts)
	}

	otherTenant := *f.tenant
	otherTenant.ID = 2
	f.tenant = &otherTenant
	if posts := read(strings.Join(ids, ",")); len(posts) != 0 {
		t.Fatalf("another tenant received records: %+v", posts)
	}
}

func TestPostRecordSelectionUsesSearchCriteria(t *testing.T) {
	f := newPostWorkflow(t)
	var ids []int64
	err := mediaFixtureScalar(pq.Array(&ids), `
		WITH added AS (
			INSERT INTO posts (tenant_id, user_id, title, slug, description, status, created_at)
			SELECT $1, $2, CASE WHEN n <= 25 OR n > 50 THEN 'Needle' ELSE 'Other' END,
			       'selected-' || n, 'Body', 0, NOW()
			FROM generate_series(1, 55) n
			RETURNING id
		)
		SELECT array_agg(id ORDER BY id) FROM added
	`, f.tenant.ID, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}

	tag := &cmd.AddNewTag{Name: "Selected tag", Color: "cccccc", IsPublic: true}
	if err := bus.Dispatch(f.ctx, tag); err != nil {
		t.Fatal(err)
	}
	if _, err := dbx.Connection().Exec(`
		INSERT INTO post_tags (tenant_id, post_id, tag_id, created_at, created_by_id)
		SELECT $1, id, $2, NOW(), $3 FROM unnest($4::integer[]) id
	`, f.tenant.ID, tag.Result.ID, f.user.ID, pq.Array(ids[:3])); err != nil {
		t.Fatal(err)
	}
	if _, err := dbx.Connection().Exec(`
		INSERT INTO post_votes (tenant_id, post_id, user_id, vote_type, created_at)
		SELECT $1, id, 2, 1, NOW() FROM unnest($2::integer[]) id
	`, f.tenant.ID, pq.Array(ids[:2])); err != nil {
		t.Fatal(err)
	}
	if _, err := mediaFixtureSQL("UPDATE posts SET status = 1 WHERE id = $1", ids[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := mediaFixtureSQL("UPDATE posts SET created_at = NOW() - INTERVAL '30 days' WHERE id = ANY($1)", pq.Array(ids[:2])); err != nil {
		t.Fatal(err)
	}

	f.user = &entity.User{ID: 2, Role: enum.RoleVisitor, Status: enum.UserActive}
	selected := make([]string, 50)
	for index, id := range ids[:50] {
		selected[index] = fmt.Sprint(id)
	}
	path := "/api/posts?view=newest&ids=" + strings.Join(selected, ",")

	cases := []struct {
		criteria string
		count    int
	}{
		{"limit=1&offset=999", 50},
		{"query=Needle", 25},
		{"query=Needle&tags=selected-tag", 3},
		{"query=Needle&tags=untagged", 22},
		{"query=Needle&statuses=open", 24},
		{"query=Needle&date=1d", 23},
		{"query=Needle&myvotes=true", 2},
		{"query=Needle&notmyvotes=true", 23},
		{"query=Needle&myposts=true", 0},
		{"query=Needle&tags=selected-tag&tagLogic=AND&date=1d&notmyvotes=true", 1},
		{"statuses=open", 49},
		{"date=1d", 48},
		{"myvotes=true", 2},
		{"notmyvotes=true", 48},
		{"myposts=true", 0},
		{"tags=selected-tag&myvotes=true", 2},
		{"tags=selected-tag&notmyvotes=true", 1},
		{"tags=selected-tag&tagLogic=AND&date=1d&notmyvotes=true", 1},
	}
	for _, test := range cases {
		t.Run(test.criteria, func(t *testing.T) {
			response, err := f.requestWithParams(api.SearchPosts(), http.MethodGet, path+"&"+test.criteria, "", nil)
			if err != nil || response.Code != http.StatusOK {
				t.Fatalf("selection: %v, HTTP %d, %s", err, response.Code, response.Body)
			}

			var posts []entity.Post
			if err := json.Unmarshal(response.Body.Bytes(), &posts); err != nil {
				t.Fatal(err)
			}
			if len(posts) != test.count {
				t.Fatalf("got %d matching posts, want %d", len(posts), test.count)
			}
			for _, post := range posts {
				if !slices.Contains(ids[:50], int64(post.ID)) {
					t.Fatalf("unrequested post %d returned", post.ID)
				}
			}
		})
	}
}

func BenchmarkPostRecordSelection(b *testing.B) {
	f := newPostWorkflow(b)
	if _, err := mediaFixtureSQL(`
		INSERT INTO posts (tenant_id, user_id, title, slug, description, status, created_at)
		SELECT $1, $2, 'Post ' || n, 'post-' || n, repeat('Post body. ', 30), 0, NOW()
		FROM generate_series(1, 5000) n
	`, f.tenant.ID, f.user.ID); err != nil {
		b.Fatal(err)
	}

	rows, err := dbx.Connection().Query("SELECT id FROM posts WHERE tenant_id = $1 ORDER BY id DESC LIMIT 50", f.tenant.ID)
	if err != nil {
		b.Fatal(err)
	}

	var ids []string
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			b.Fatal(err)
		}

		ids = append(ids, fmt.Sprint(id))
	}
	if err := rows.Err(); err != nil {
		b.Fatal(err)
	}
	if err := rows.Close(); err != nil {
		b.Fatal(err)
	}

	cases := []struct {
		name string
		path string
	}{
		{"selected", "/api/posts?ids=" + strings.Join(ids, ",")},
		{"all", "/api/posts?view=all&limit=50"},
		{"trending", "/api/posts?view=trending&limit=50"},
	}

	for _, test := range cases {
		b.Run(test.name, func(b *testing.B) {
			response, err := f.requestWithParams(api.SearchPosts(), http.MethodGet, test.path, "", nil)
			if err != nil || response.Code != http.StatusOK {
				b.Fatalf("read posts: %v, HTTP %d", err, response.Code)
			}

			var posts []*entity.Post
			if err := json.Unmarshal(response.Body.Bytes(), &posts); err != nil {
				b.Fatal(err)
			}
			if len(posts) != 50 {
				b.Fatalf("expected 50 posts, received %d", len(posts))
			}

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				response, err := f.requestWithParams(api.SearchPosts(), http.MethodGet, test.path, "", nil)
				if err != nil || response.Code != http.StatusOK {
					b.Fatalf("read posts: %v, HTTP %d", err, response.Code)
				}
			}
			b.StopTimer()
		})
	}
}
