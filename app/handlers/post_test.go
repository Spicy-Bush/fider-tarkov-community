package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	. "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/assert"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/mock"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestIndexUsesURLFilters(t *testing.T) {
	bus.AddHandler(func(ctx context.Context, q *query.CountPostPerStatus) error {
		return nil
	})
	bus.AddHandler(func(ctx context.Context, q *query.GetAllTags) error {
		return nil
	})
	bus.AddHandler(func(ctx context.Context, q *query.GetUserProfileStanding) error {
		return nil
	})

	cases := []struct {
		name   string
		url    string
		user   *entity.User
		search query.SearchPosts
	}{
		{
			name: "anonymous defaults",
			url:  "/",
			search: query.SearchPosts{
				View:     "trending",
				Limit:    "20",
				Tags:     []string{},
				Statuses: []enum.PostStatus{},
				TagLogic: "OR",
			},
		},
		{
			name: "authenticated defaults",
			url:  "/",
			user: mock.JonSnow,
			search: query.SearchPosts{
				View:       "trending",
				Limit:      "20",
				Tags:       []string{},
				Statuses:   []enum.PostStatus{},
				TagLogic:   "OR",
				NotMyVotes: true,
			},
		},
		{
			name: "text query",
			url:  "/?query=missing+item",
			search: query.SearchPosts{
				Query:    "missing item",
				View:     "trending",
				Limit:    "20",
				Tags:     []string{},
				Statuses: []enum.PostStatus{},
				TagLogic: "OR",
			},
		},
		{
			name: "combined filters",
			url:  "/?view=newest&tags=one&tags=two&statuses=planned&date=7d&taglogic=AND&myvotes=true",
			search: query.SearchPosts{
				View:        "newest",
				Limit:       "20",
				Tags:        []string{"one", "two"},
				Statuses:    []enum.PostStatus{enum.PostPlanned},
				Date:        "7d",
				TagLogic:    "AND",
				MyVotesOnly: true,
			},
		},
		{
			name: "untagged",
			url:  "/?tags=untagged&tags=one",
			search: query.SearchPosts{
				View:     "trending",
				Limit:    "20",
				Statuses: []enum.PostStatus{},
				TagLogic: "OR",
				Untagged: true,
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var received *query.SearchPosts
			bus.AddHandler(func(ctx context.Context, q *query.SearchPosts) error {
				received = q
				return nil
			})

			server := mock.NewServer().OnTenant(mock.DemoTenant).
				WithURL(test.url).
				AddCookie("pfilter", "1").
				AddHeader("Accept", web.PageDataContentType)
			if test.user != nil {
				server.AsUser(test.user)
			}

			status, response := server.Execute(handlers.Index())

			if status != http.StatusOK || !reflect.DeepEqual(received, &test.search) {
				t.Fatalf("status %d, search %+v; wanted %+v", status, received, test.search)
			}

			var page struct {
				Props struct {
					InitialFilters map[string]any `json:"initialFilters"`
				} `json:"props"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
				t.Fatal(err)
			}

			filters := page.Props.InitialFilters
			if filters["query"] != received.Query || filters["view"] != received.View ||
				filters["date"] != received.Date || filters["tagLogic"] != received.TagLogic ||
				filters["myVotes"] != received.MyVotesOnly || filters["myPosts"] != received.MyPostsOnly ||
				filters["notMyVotes"] != received.NotMyVotes || filters["limit"] != float64(20) {
				t.Fatalf("page criteria differ from its rows: %+v", filters)
			}
			if filters["tags"] == nil || filters["statuses"] == nil {
				t.Fatal("filter collections must be arrays")
			}
			if received.Untagged && !reflect.DeepEqual(filters["tags"], []any{"untagged", "one"}) {
				t.Fatalf("page lost the untagged selection: %+v", filters["tags"])
			}
		})
	}
}

func TestIndexHandler(t *testing.T) {
	RegisterT(t)

	bus.AddHandler(func(ctx context.Context, q *query.CountPostPerStatus) error {
		return nil
	})

	bus.AddHandler(func(ctx context.Context, q *query.GetAllTags) error {
		return nil
	})

	bus.AddHandler(func(ctx context.Context, q *query.SearchPosts) error {
		return nil
	})

	bus.AddHandler(func(ctx context.Context, q *query.GetUserProfileStanding) error {
		return nil
	})

	server := mock.NewServer()
	code, _ := server.OnTenant(mock.DemoTenant).
		AsUser(mock.JonSnow).
		Execute(handlers.Index())

	Expect(code).Equals(http.StatusOK)
}

func TestDetailsHandler(t *testing.T) {
	RegisterT(t)

	post := &entity.Post{Number: 1, Title: "My Post Title", Slug: "my-post-title"}

	bus.AddHandler(func(ctx context.Context, q *query.GetPostByNumber) error {
		q.Result = post
		return nil
	})

	bus.AddHandler(func(ctx context.Context, q *query.GetPostAttachments) error {
		return nil
	})

	bus.AddHandler(func(ctx context.Context, q *query.ListPostVotes) error {
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

	bus.AddHandler(func(ctx context.Context, q *query.GetUserProfileStanding) error {
		return nil
	})

	server := mock.NewServer()

	code, _ := server.
		OnTenant(mock.DemoTenant).
		AsUser(mock.JonSnow).
		AddParam("number", post.Number).
		AddParam("slug", post.Slug).
		Execute(handlers.PostDetails())

	Expect(code).Equals(http.StatusOK)
}

func TestDetailsHandler_RedirectOnDifferentSlu(t *testing.T) {
	RegisterT(t)

	post := &entity.Post{Number: 1, Title: "My Post Title", Slug: "my-post-title"}

	bus.AddHandler(func(ctx context.Context, q *query.GetPostByNumber) error {
		q.Result = post
		return nil
	})

	bus.AddHandler(func(ctx context.Context, q *query.GetAllTags) error {
		return nil
	})

	bus.AddHandler(func(ctx context.Context, q *query.GetPostAttachments) error {
		return nil
	})

	bus.AddHandler(func(ctx context.Context, q *query.GetReportReasons) error {
		return nil
	})

	bus.AddHandler(func(ctx context.Context, q *query.UserSubscribedTo) error {
		return nil
	})

	bus.AddHandler(func(ctx context.Context, q *query.ListPostVotes) error {
		return nil
	})

	bus.AddHandler(func(ctx context.Context, q *query.GetUserReportStatus) error {
		return nil
	})

	bus.AddHandler(func(ctx context.Context, q *query.GetUserProfileStanding) error {
		return nil
	})

	server := mock.NewServer()

	code, _ := server.
		OnTenant(mock.DemoTenant).
		AsUser(mock.JonSnow).
		AddParam("number", post.Number).
		AddParam("slug", "some-other-slug").
		Execute(handlers.PostDetails())

	Expect(code).Equals(http.StatusOK)
}

func TestDetailsHandler_NotFound(t *testing.T) {
	RegisterT(t)

	bus.AddHandler(func(ctx context.Context, q *query.GetPostByNumber) error {
		return app.ErrNotFound
	})

	bus.AddHandler(func(ctx context.Context, q *query.GetUserProfileStanding) error {
		return nil
	})

	server := mock.NewServer()
	code, _ := server.
		OnTenant(mock.DemoTenant).
		AsUser(mock.JonSnow).
		AddParam("number", "99").
		Execute(handlers.PostDetails())

	Expect(code).Equals(http.StatusNotFound)
}
