package postgres_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/api"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/postcache"
	"github.com/lib/pq"
)

func TestPostRankingRespectsVisibility(t *testing.T) {
	f := newPostWorkflow(t)
	postcache.InvalidateTenantRankings(f.tenant.ID)
	t.Cleanup(func() { postcache.InvalidateTenantRankings(f.tenant.ID) })

	var ids []int64
	err := dbx.Connection().QueryRow(`
		WITH added AS (
			INSERT INTO posts (tenant_id, user_id, title, slug, description, status, created_at, moderation_pending)
			SELECT $1, 1, 'Needle ' || n, 'needle-' || n, 'Body', 0, NOW(), n > 10
			FROM generate_series(1, 30) n
			RETURNING id
		)
		SELECT array_agg(id ORDER BY id) FROM added
	`, f.tenant.ID).Scan(pq.Array(&ids))
	if err != nil {
		t.Fatal(err)
	}

	read := func(path string) []int64 {
		t.Helper()
		response, err := f.requestWithParams(api.SearchPosts(), http.MethodGet, path, "", nil)
		if err != nil || response.Code != http.StatusOK {
			t.Fatalf("search: %v, HTTP %d, %s", err, response.Code, response.Body)
		}

		var posts []entity.Post
		if err := json.Unmarshal(response.Body.Bytes(), &posts); err != nil {
			t.Fatal(err)
		}
		result := make([]int64, len(posts))
		for index, post := range posts {
			result[index] = int64(post.ID)
		}

		return result
	}

	f.user = nil
	public := slices.Clone(ids[5:10])
	slices.Reverse(public)
	if got := read("/api/posts?view=newest&limit=5"); !slices.Equal(got, public) {
		t.Fatalf("public ranking: got %v, want %v", got, public)
	}

	staff := slices.Clone(ids[25:30])
	slices.Reverse(staff)
	cases := []struct {
		name string
		user *entity.User
	}{
		{"moderator", &entity.User{ID: 2, Role: enum.RoleModerator, Status: enum.UserActive}},
		{"collaborator", &entity.User{ID: 2, Role: enum.RoleCollaborator, Status: enum.UserActive}},
		{"administrator", &entity.User{ID: 2, Role: enum.RoleAdministrator, Status: enum.UserActive}},
		{"author", &entity.User{ID: 1, Role: enum.RoleVisitor, Status: enum.UserActive}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			f.user = test.user
			if got := read("/api/posts?view=newest&limit=5"); !slices.Equal(got, staff) {
				t.Fatalf("public cache changed viewer results: got %v, want %v", got, staff)
			}
		})
	}

	_, err = dbx.Connection().Exec("UPDATE posts SET moderation_pending = TRUE WHERE id = ANY($1)", pq.Array(public))
	if err != nil {
		t.Fatal(err)
	}

	f.user = nil
	want := slices.Clone(ids[:5])
	slices.Reverse(want)
	if got := read("/api/posts?view=newest&limit=5"); !slices.Equal(got, want) {
		t.Fatalf("hidden cached rows left a short page: got %v, want %v", got, want)
	}
}
