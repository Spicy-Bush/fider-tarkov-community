package api_test

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/api"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/mock"
)

func TestPostListingLimitsDoNotDependOnEnvironment(t *testing.T) {
	previous := env.Config.Environment
	t.Cleanup(func() { env.Config.Environment = previous })

	var received int
	bus.AddHandler(func(ctx context.Context, q *query.SearchPosts) error {
		received, _ = strconv.Atoi(q.Limit)
		return nil
	})

	for _, environment := range []string{"development", "production"} {
		env.Config.Environment = environment
		for _, privileged := range []bool{false, true} {
			maximum := 15
			if privileged {
				maximum = 50
			}

			for _, limit := range []string{"", "all", "9999", "15"} {
				tenant := *mock.DemoTenant
				server := mock.NewServer().OnTenant(&tenant).WithURL("/api/posts?limit=" + limit)
				if privileged {
					tenant.RolePermissions = entity.RolePermissions{enum.RoleVisitor: {entity.ManageQueue: true}}
					server.AsUser(&entity.User{ID: 1, Role: enum.RoleVisitor, Status: enum.UserActive})
				}

				status, _ := server.Execute(api.SearchPosts())
				want := maximum
				if limit == "15" {
					want = 15
				}
				if status != http.StatusOK || received != want {
					t.Fatalf("%s privileged=%v limit=%q: status=%d rows=%d, wanted %d", environment, privileged, limit, status, received, want)
				}
			}
		}
	}
}
