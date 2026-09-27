package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/mock"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestServerCapabilityDataProjection(t *testing.T) {
	bus.AddHandler(func(ctx context.Context, q *query.GetUserProfileStanding) error {
		return nil
	})
	bus.AddHandler(func(ctx context.Context, q *query.GetAllUsers) error {
		q.Result = []*entity.User{{ID: 2, Name: "Member", Email: "private@example.invalid"}}
		return nil
	})
	bus.AddHandler(func(ctx context.Context, q *query.GetAllUserProviders) error {
		return nil
	})
	bus.AddHandler(func(ctx context.Context, q *query.GetPostByNumber) error {
		q.Result = &entity.Post{ID: 1, Number: 1, Slug: "fixture", Title: "Fixture", Status: enum.PostOpen}
		return nil
	})
	bus.AddHandler(func(ctx context.Context, q *query.GetPostAttachments) error {
		return nil
	})
	bus.AddHandler(func(ctx context.Context, q *query.GetAllTags) error {
		return nil
	})
	bus.AddHandler(func(ctx context.Context, q *query.UserSubscribedTo) error {
		return nil
	})
	bus.AddHandler(func(ctx context.Context, q *query.GetReportReasons) error {
		return nil
	})
	bus.AddHandler(func(ctx context.Context, q *query.GetUserReportStatus) error {
		return nil
	})
	bus.AddHandler(func(ctx context.Context, q *query.ListPostVotes) error {
		q.Result = []*entity.Vote{{User: &entity.VoteUser{ID: 2}, VoteType: enum.VoteTypeDown}}
		return nil
	})
	bus.AddHandler(func(ctx context.Context, q *query.CountUnreadNotifications) error {
		q.Result = 5
		return nil
	})

	for _, test := range []struct {
		role    enum.Role
		email   bool
		votes   bool
		reports bool
		queue   bool
	}{
		{role: enum.RoleVisitor},
		{role: enum.RoleHelper, queue: true},
		{role: enum.RoleModerator, votes: true, reports: true, queue: true},
		{role: enum.RoleCollaborator, email: true, votes: true, reports: true, queue: true},
		{role: enum.RoleAdministrator, email: true, votes: true, reports: true, queue: true},
	} {
		for _, locked := range []bool{false, true} {
			name := test.role.String()
			if locked {
				name += "/locked"
			}
			t.Run(name, func(t *testing.T) {
				tenant := *mock.DemoTenant
				if locked {
					tenant.Status = enum.TenantLocked
				}
				viewer := &entity.User{ID: 1, Role: test.role, Tenant: &tenant, Status: enum.UserActive}
				server := func() *mock.Server {
					return mock.NewServer().OnTenant(&tenant).AsUser(viewer)
				}

				status, response := server().AddHeader("Accept", web.PageDataContentType).Execute(handlers.ManageMembers())
				if status != http.StatusOK {
					t.Fatalf("members response=%d", status)
				}
				var members struct {
					Props struct {
						Users []struct {
							Email string `json:"email"`
						} `json:"users"`
					} `json:"props"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &members); err != nil {
					t.Fatal(err)
				}
				if len(members.Props.Users) != 1 || (members.Props.Users[0].Email != "") != (test.email && !locked) {
					t.Fatal("member email projection disagrees with capability")
				}

				status, response = server().AddHeader("Accept", web.PageDataContentType).
					AddParam("number", 1).AddParam("slug", "fixture").Execute(handlers.PostDetails())
				if status != http.StatusOK {
					t.Fatalf("post response=%d", status)
				}
				var post struct {
					Props struct {
						Votes []*entity.Vote `json:"votes"`
					} `json:"props"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &post); err != nil {
					t.Fatal(err)
				}
				if len(post.Props.Votes) != 1 || (post.Props.Votes[0].VoteType == enum.VoteTypeDown) != (test.votes && !locked) {
					t.Fatal("vote direction projection disagrees with capability")
				}

				reportReads, queueReads := 0, 0
				bus.AddHandler(func(ctx context.Context, q *query.CountPendingReports) error {
					reportReads++
					q.Result = 3
					return nil
				})
				bus.AddHandler(func(ctx context.Context, q *query.CountUntaggedPosts) error {
					queueReads++
					q.Result = 7
					return nil
				})
				status, response = server().Execute(handlers.TotalUnreadNotifications())
				var counts map[string]int
				if err := json.Unmarshal(response.Body.Bytes(), &counts); err != nil {
					t.Fatal(err)
				}
				if status != http.StatusOK || counts["total"] != 5 {
					t.Fatalf("unread response: status=%d, counts=%v", status, counts)
				}
				if (reportReads == 1) != (test.reports && !locked) || (counts["pendingReports"] == 3) != (test.reports && !locked) {
					t.Fatal("report count projection disagrees with capability")
				}
				if (queueReads == 1) != (test.queue && !locked) || (counts["queueCount"] == 7) != (test.queue && !locked) {
					t.Fatal("queue count projection disagrees with capability")
				}
			})
		}
	}
}
