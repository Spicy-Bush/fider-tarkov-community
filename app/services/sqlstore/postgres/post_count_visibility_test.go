package postgres_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/mock"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestHomeCountsFollowCurrentPostVisibility(t *testing.T) {
	f := newPostWorkflow(t)
	if _, err := mediaFixtureSQL(`
		INSERT INTO posts (tenant_id, user_id, title, slug, description, status, created_at, moderation_pending)
		SELECT $1, 1, 'Count fixture ' || n, 'count-fixture-' || n, '',
			CASE WHEN n = 5 THEN 6 ELSE n % 2 END, NOW(), n IN (3, 4)
		FROM generate_series(1, 5) n
	`, f.tenant.ID); err != nil {
		t.Fatal(err)
	}

	visitor := &entity.User{ID: 2, Role: enum.RoleVisitor, Status: enum.UserActive}
	moderator := &entity.User{ID: 2, Role: enum.RoleModerator, Status: enum.UserActive}
	for _, test := range []struct {
		name string
		user *entity.User
		want int
	}{
		{"moderator", moderator, 2},
		{"anonymous after moderator", nil, 1},
		{"other member", visitor, 1},
		{"author", &entity.User{ID: 1, Role: enum.RoleVisitor, Status: enum.UserActive}, 2},
		{"anonymous after author", nil, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := mock.NewServer().OnTenant(f.tenant).WithURL("/").AddHeader("Accept", web.PageDataContentType)
			if test.user != nil {
				server.AsUser(test.user)
			}

			status, response := server.Execute(handlers.Index())
			if status != http.StatusOK {
				t.Fatalf("home HTTP %d: %s", status, response.Body)
			}
			var page struct {
				Props struct {
					Counts map[enum.PostStatus]int `json:"countPerStatus"`
				} `json:"props"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
				t.Fatal(err)
			}
			if page.Props.Counts[enum.PostOpen] != test.want || page.Props.Counts[enum.PostStarted] != test.want || page.Props.Counts[enum.PostDeleted] != 0 {
				t.Fatalf("counts do not match visible posts: %v", page.Props.Counts)
			}
		})
	}
}
