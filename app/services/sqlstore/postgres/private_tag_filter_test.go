package postgres_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/api"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/mock"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestHomeTagsFollowCurrentViewerPermissions(t *testing.T) {
	f := newPostWorkflow(t)
	private := &cmd.AddNewTag{Name: "Private home tag", Color: "123456"}
	public := &cmd.AddNewTag{Name: "Public home tag", Color: "654321", IsPublic: true}
	if err := bus.Dispatch(f.ctx, private, public); err != nil {
		t.Fatal(err)
	}

	viewer := *f.user
	viewer.Role = enum.RoleCollaborator
	for _, choice := range []struct {
		name          string
		authenticated bool
		viewPrivate   bool
		assign        bool
	}{
		{name: "permitted editor", authenticated: true, viewPrivate: true, assign: true},
		{name: "anonymous viewer"},
		{name: "public tag editor", authenticated: true, assign: true},
		{name: "private tag reader", authenticated: true, viewPrivate: true},
		{name: "revoked editor", authenticated: true},
		{name: "restored editor", authenticated: true, viewPrivate: true, assign: true},
	} {
		t.Run(choice.name, func(t *testing.T) {
			f.tenant.RolePermissions = entity.RolePermissions{
				enum.RoleCollaborator: {
					entity.ViewPrivateTags: choice.viewPrivate,
					entity.TagPosts:        choice.assign,
				},
			}
			server := mock.NewServer().OnTenant(f.tenant).WithURL("/").AddHeader("Accept", web.PageDataContentType)
			if choice.authenticated {
				server.AsUser(&viewer)
			}

			status, response := server.Execute(handlers.Index())
			if status != http.StatusOK {
				t.Fatalf("home status=%d: %s", status, response.Body)
			}

			var page struct {
				Props struct {
					Tags []*entity.Tag `json:"tags"`
				} `json:"props"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
				t.Fatal(err)
			}

			foundPublic := false
			foundPrivate := false
			for _, tag := range page.Props.Tags {
				if tag.ID != public.Result.ID && tag.ID != private.Result.ID {
					continue
				}
				foundPublic = foundPublic || tag.ID == public.Result.ID
				foundPrivate = foundPrivate || tag.ID == private.Result.ID
				if tag.Permissions.Assign != choice.assign {
					t.Fatalf("tag %s assignment=%t, want %t", tag.Name, tag.Permissions.Assign, choice.assign)
				}
			}
			if !foundPublic || foundPrivate != choice.viewPrivate {
				t.Fatalf("public=%t private=%t, want public=true private=%t", foundPublic, foundPrivate, choice.viewPrivate)
			}
		})
	}
}

func TestPrivateTagFiltersDoNotRevealHiddenMembership(t *testing.T) {
	f := newPostWorkflow(t)
	private := &cmd.AddNewTag{Name: "Confidential", Color: "123456"}
	public := &cmd.AddNewTag{Name: "Public", Color: "654321", IsPublic: true}

	privatePost := &cmd.AddNewPost{Title: "Private tag only", Description: "Public content"}
	publicPost := &cmd.AddNewPost{Title: "Public tag only", Description: "Public content"}
	bothPost := &cmd.AddNewPost{Title: "Both tags", Description: "Public content"}
	untaggedPost := &cmd.AddNewPost{Title: "No tags", Description: "Public content"}
	if err := bus.Dispatch(f.ctx, private, public, privatePost, publicPost, bothPost, untaggedPost); err != nil {
		t.Fatal(err)
	}

	if err := bus.Dispatch(f.ctx,
		&cmd.AssignTag{Tag: private.Result, Post: privatePost.Result},
		&cmd.AssignTag{Tag: public.Result, Post: publicPost.Result},
		&cmd.AssignTag{Tag: private.Result, Post: bothPost.Result},
		&cmd.AssignTag{Tag: public.Result, Post: bothPost.Result},
	); err != nil {
		t.Fatal(err)
	}

	viewer := *f.user
	viewer.Role = enum.RoleCollaborator
	f.user = &viewer

	for _, visible := range []bool{true, false} {
		f.tenant.RolePermissions = entity.RolePermissions{
			enum.RoleCollaborator: {
				entity.ViewPrivateTags: visible,
				entity.ManageTags:      false,
			},
		}

		for _, test := range []struct {
			name    string
			tags    []string
			logic   string
			allowed []int
			revoked []int
		}{
			{
				name:    "private",
				tags:    []string{private.Result.Slug},
				logic:   "OR",
				allowed: []int{privatePost.Result.ID, bothPost.Result.ID},
			},
			{
				name:    "public",
				tags:    []string{public.Result.Slug},
				logic:   "OR",
				allowed: []int{publicPost.Result.ID, bothPost.Result.ID},
				revoked: []int{publicPost.Result.ID, bothPost.Result.ID},
			},
			{
				name:    "either",
				tags:    []string{private.Result.Slug, public.Result.Slug},
				logic:   "OR",
				allowed: []int{privatePost.Result.ID, publicPost.Result.ID, bothPost.Result.ID},
				revoked: []int{publicPost.Result.ID, bothPost.Result.ID},
			},
			{
				name:    "both",
				tags:    []string{private.Result.Slug, public.Result.Slug},
				logic:   "AND",
				allowed: []int{bothPost.Result.ID},
			},
		} {
			want := test.revoked
			if visible {
				want = test.allowed
			}

			response, err := f.requestWithParams(api.SearchPosts(), http.MethodGet,
				"http://localhost:3000/api/posts?tags="+strings.Join(test.tags, ",")+"&tagLogic="+test.logic, "", nil)
			if err != nil || response.Code != http.StatusOK {
				t.Fatalf("search %s visible=%v: status=%d body=%s err=%v", test.name, visible, response.Code, response.Body.String(), err)
			}

			var posts []*entity.Post
			if err := json.Unmarshal(response.Body.Bytes(), &posts); err != nil {
				t.Fatal(err)
			}

			ids := make([]int, len(posts))
			for i, post := range posts {
				ids[i] = post.ID
			}
			slices.Sort(ids)

			if !slices.Equal(ids, want) {
				t.Errorf("search %s visible=%v returned %v, want %v", test.name, visible, ids, want)
			}
		}

		response, err := f.requestWithParams(handlers.ListArchivablePosts(), http.MethodGet,
			"http://localhost:3000/api/admin/archive/posts?tags="+private.Result.Slug, "", nil)
		if err != nil || response.Code != http.StatusOK {
			t.Fatalf("archive visible=%v: status=%d body=%s err=%v", visible, response.Code, response.Body.String(), err)
		}

		var result struct {
			Posts []*entity.Post `json:"posts"`
			Total int            `json:"total"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}

		want := 0
		if visible {
			want = 2
		}

		if len(result.Posts) != want || result.Total != want {
			t.Errorf("archive visible=%v returned %d rows / %d total, want %d", visible, len(result.Posts), result.Total, want)
		}

		response, err = f.requestWithParams(api.SearchPosts(), http.MethodGet,
			"http://localhost:3000/api/posts?tags=untagged&includeCount=true&date=1d", "", nil)
		if err != nil || response.Code != http.StatusOK {
			t.Fatalf("untagged visible=%v: status=%d body=%s err=%v", visible, response.Code, response.Body.String(), err)
		}

		var untagged []*entity.Post
		if err := json.Unmarshal(response.Body.Bytes(), &untagged); err != nil {
			t.Fatal(err)
		}

		privateIncluded := slices.ContainsFunc(untagged, func(post *entity.Post) bool {
			return post.ID == privatePost.Result.ID
		})
		if privateIncluded == visible {
			t.Errorf("untagged visible=%v revealed hidden tag membership", visible)
		}

		if !slices.ContainsFunc(untagged, func(post *entity.Post) bool {
			return post.ID == untaggedPost.Result.ID
		}) {
			t.Error("untagged filter omitted a post without tags")
		}

		if slices.ContainsFunc(untagged, func(post *entity.Post) bool {
			return post.ID == publicPost.Result.ID || post.ID == bothPost.Result.ID
		}) {
			t.Error("untagged filter included a visibly tagged post")
		}

		if response.Header().Get("X-Total-Count") != strconv.Itoa(len(untagged)) {
			t.Errorf("untagged count %s differs from %d visible posts", response.Header().Get("X-Total-Count"), len(untagged))
		}
	}
}
