package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/api"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestPostPermissionsWorkflowEditOwnership(t *testing.T) {
	f := newPostWorkflow(t)
	post := &cmd.AddNewPost{Title: "Authorization target", Description: "Original content"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name       string
		viewerRole enum.Role
		authorRole enum.Role
		own        bool
		age        time.Duration
		edit       bool
		delete     bool
	}{
		{"anonymous", 0, enum.RoleVisitor, false, 0, false, false},
		{"visitor other", enum.RoleVisitor, enum.RoleVisitor, false, 0, false, false},
		{"visitor own recent", enum.RoleVisitor, enum.RoleVisitor, true, time.Minute, true, false},
		{"visitor own expired", enum.RoleVisitor, enum.RoleVisitor, true, 2 * time.Hour, false, false},
		{"helper other", enum.RoleHelper, enum.RoleVisitor, false, 0, false, false},
		{"helper own recent", enum.RoleHelper, enum.RoleHelper, true, time.Minute, true, false},
		{"moderator visitor", enum.RoleModerator, enum.RoleVisitor, false, 2 * time.Hour, true, true},
		{"moderator helper", enum.RoleModerator, enum.RoleHelper, false, 2 * time.Hour, true, true},
		{"moderator peer", enum.RoleModerator, enum.RoleModerator, false, 0, false, false},
		{"moderator collaborator", enum.RoleModerator, enum.RoleCollaborator, false, 0, false, false},
		{"moderator administrator", enum.RoleModerator, enum.RoleAdministrator, false, 0, false, false},
		{"moderator own recent", enum.RoleModerator, enum.RoleModerator, true, time.Minute, true, false},
		{"moderator own expired", enum.RoleModerator, enum.RoleModerator, true, 2 * time.Hour, false, false},
		{"collaborator administrator", enum.RoleCollaborator, enum.RoleAdministrator, false, 2 * time.Hour, true, true},
		{"administrator collaborator", enum.RoleAdministrator, enum.RoleCollaborator, false, 2 * time.Hour, true, true},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			authorID := 2
			if test.own {
				authorID = 1
			}
			if _, err := dbx.Connection().Exec(`UPDATE users SET role = $1 WHERE id = $2`, test.authorRole, authorID); err != nil {
				t.Fatal(err)
			}
			if _, err := dbx.Connection().Exec(`UPDATE posts
				SET user_id = $1, created_at = $2, title = 'Authorization target',
					description = 'Original content', moderation_pending = FALSE
				WHERE id = $3`, authorID, time.Now().Add(-test.age), post.Result.ID); err != nil {
				t.Fatal(err)
			}

			viewer := f
			viewer.user = nil
			if test.viewerRole != 0 {
				viewer.user = &entity.User{ID: 1, Role: test.viewerRole, Status: enum.UserActive}
			}
			response, err := viewer.request(api.GetPost(), http.MethodGet, post.Result.Number, "")
			if err != nil || response.Code != http.StatusOK {
				t.Fatalf("get post: %v, status %d", err, response.Code)
			}
			var projected entity.Post
			if err := json.Unmarshal(response.Body.Bytes(), &projected); err != nil {
				t.Fatal(err)
			}
			if projected.Permissions.Edit != test.edit || projected.Permissions.Delete != test.delete {
				t.Fatalf("projected edit/delete %v/%v; want %v/%v", projected.Permissions.Edit, projected.Permissions.Delete, test.edit, test.delete)
			}

			body := fmt.Sprintf(`{
				"title":"Authorization target",
				"description":"Accepted edit",
				"permissions":{"edit":true,"delete":true},
				"post":{"id":%d,"user":{"id":1},"permissions":{"edit":true}}
			}`, post.Result.ID)
			response, err = viewer.request(api.UpdatePost(), http.MethodPut, post.Result.Number, body)
			wantStatus := http.StatusForbidden
			if test.edit {
				wantStatus = http.StatusOK
			}
			if err != nil || response.Code != wantStatus {
				t.Fatalf("edit status %d, error %v; want %d; body %s", response.Code, err, wantStatus, response.Body.String())
			}

			var description string
			if err := dbx.Connection().QueryRow(`SELECT description FROM posts WHERE id = $1`, post.Result.ID).Scan(&description); err != nil {
				t.Fatal(err)
			}
			wantDescription := "Original content"
			if test.edit {
				wantDescription = "Accepted edit"
			}
			if description != wantDescription {
				t.Fatalf("persisted description %q; want %q", description, wantDescription)
			}
		})
	}
}

func TestPostPermissionsWorkflowVisibilityAndLocks(t *testing.T) {
	f := newPostWorkflow(t)
	post := &cmd.AddNewPost{Title: "Visibility target", Description: "Content"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}
	if _, err := dbx.Connection().Exec(`UPDATE users SET role = $1 WHERE id = 1`, enum.RoleVisitor); err != nil {
		t.Fatal(err)
	}
	if _, err := dbx.Connection().Exec(`UPDATE posts SET moderation_pending = TRUE WHERE id = $1`, post.Result.ID); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name   string
		user   *entity.User
		status int
		hidden bool
		edit   bool
	}{
		{
			name:   "anonymous",
			status: http.StatusNotFound,
		},
		{
			name:   "outsider",
			user:   &entity.User{ID: 2, Role: enum.RoleVisitor, Status: enum.UserActive},
			status: http.StatusNotFound,
		},
		{
			name:   "author",
			user:   &entity.User{ID: 1, Role: enum.RoleVisitor, Status: enum.UserActive},
			status: http.StatusOK,
			edit:   true,
		},
		{
			name:   "moderator",
			user:   &entity.User{ID: 2, Role: enum.RoleModerator, Status: enum.UserActive},
			status: http.StatusOK,
			hidden: true,
			edit:   true,
		},
	} {
		t.Run("hidden/"+test.name, func(t *testing.T) {
			viewer := f
			viewer.user = test.user
			response, err := viewer.request(api.GetPost(), http.MethodGet, post.Result.Number, "")
			if err != nil || response.Code != test.status {
				t.Fatalf("get hidden post: %v, status %d; want %d", err, response.Code, test.status)
			}
			if test.status != http.StatusOK {
				return
			}
			var projected entity.Post
			if err := json.Unmarshal(response.Body.Bytes(), &projected); err != nil {
				t.Fatal(err)
			}
			if projected.ModerationPending != test.hidden || projected.Permissions.Edit != test.edit {
				t.Fatalf("hidden flag %v, edit %v; want %v, %v", projected.ModerationPending, projected.Permissions.Edit, test.hidden, test.edit)
			}
		})
	}

	if _, err := dbx.Connection().Exec(`UPDATE posts
		SET moderation_pending = FALSE, locked_settings = '{"locked":true}'
		WHERE id = $1`, post.Result.ID); err != nil {
		t.Fatal(err)
	}
	for _, role := range []enum.Role{enum.RoleVisitor, enum.RoleHelper, enum.RoleModerator, enum.RoleCollaborator, enum.RoleAdministrator} {
		t.Run("locked/"+role.String(), func(t *testing.T) {
			viewer := f
			viewer.user = &entity.User{ID: 1, Role: role, Status: enum.UserActive}
			response, err := viewer.request(api.GetPost(), http.MethodGet, post.Result.Number, "")
			if err != nil || response.Code != http.StatusOK {
				t.Fatalf("get locked post: %v, status %d", err, response.Code)
			}
			var projected entity.Post
			if err := json.Unmarshal(response.Body.Bytes(), &projected); err != nil {
				t.Fatal(err)
			}
			allowed := role == enum.RoleCollaborator || role == enum.RoleAdministrator
			if projected.Permissions.Edit != allowed || projected.Permissions.Follow != allowed || projected.Permissions.Vote != allowed {
				t.Fatalf("locked permissions %+v; edit/follow/vote should be %v", projected.Permissions, allowed)
			}

			body := fmt.Sprintf(`{"revision":%d,"permissions":{"vote":true}}`, projected.VoteRevision)
			response, err = viewer.request(api.AddVote(), http.MethodPost, post.Result.Number, body)
			wantStatus := http.StatusForbidden
			if allowed {
				wantStatus = http.StatusOK
			}
			if err != nil || response.Code != wantStatus {
				t.Fatalf("locked vote: %v, status %d; want %d", err, response.Code, wantStatus)
			}
		})
	}

	if _, err := dbx.Connection().Exec(`UPDATE posts SET status = $1 WHERE id = $2`, enum.PostDeleted, post.Result.ID); err != nil {
		t.Fatal(err)
	}
	for _, role := range []enum.Role{enum.RoleVisitor, enum.RoleHelper, enum.RoleModerator, enum.RoleCollaborator, enum.RoleAdministrator} {
		viewer := f
		viewer.user = &entity.User{ID: 1, Role: role, Status: enum.UserActive}
		response, err := viewer.request(api.GetPost(), http.MethodGet, post.Result.Number, "")
		if err != nil || response.Code != http.StatusNotFound {
			t.Fatalf("%s read deleted post: %v, status %d", role, err, response.Code)
		}
	}
}

func TestPostPermissionsWorkflowResponseStatuses(t *testing.T) {
	f := newPostWorkflow(t)
	original := &cmd.AddNewPost{Title: "Response original", Description: "Original"}
	post := &cmd.AddNewPost{Title: "Response target", Description: "Target"}
	if err := bus.Dispatch(f.ctx, original, post); err != nil {
		t.Fatal(err)
	}

	for _, role := range []enum.Role{enum.RoleVisitor, enum.RoleHelper, enum.RoleModerator, enum.RoleCollaborator, enum.RoleAdministrator} {
		t.Run(role.String(), func(t *testing.T) {
			viewer := f
			viewer.user = &entity.User{ID: 1, Role: role, Status: enum.UserActive}
			response, err := viewer.request(api.GetPost(), http.MethodGet, post.Result.Number, "")
			if err != nil || response.Code != http.StatusOK {
				t.Fatalf("get response target: %v, status %d", err, response.Code)
			}
			var projected entity.Post
			if err := json.Unmarshal(response.Body.Bytes(), &projected); err != nil {
				t.Fatal(err)
			}
			allowed := []enum.PostStatus{}
			if role == enum.RoleModerator {
				allowed = []enum.PostStatus{enum.PostDuplicate}
			} else if role == enum.RoleCollaborator || role == enum.RoleAdministrator {
				allowed = []enum.PostStatus{
					enum.PostOpen, enum.PostStarted, enum.PostCompleted,
					enum.PostDeclined, enum.PostPlanned, enum.PostDuplicate,
				}
			}
			if !reflect.DeepEqual(projected.Permissions.Respond, allowed) {
				t.Fatalf("response options %v; want %v", projected.Permissions.Respond, allowed)
			}

			for _, status := range []enum.PostStatus{enum.PostCompleted, enum.PostDuplicate, enum.PostDeleted, enum.PostArchived} {
				if _, err := dbx.Connection().Exec(`UPDATE posts
					SET status = $1, original_id = NULL, response = NULL WHERE id = $2`, enum.PostOpen, post.Result.ID); err != nil {
					t.Fatal(err)
				}
				body := fmt.Sprintf(`{"status":%q,"originalNumber":%d,"permissions":{"respond":["completed","deleted"]}}`, status.Name(), original.Result.Number)
				response, err := viewer.request(api.SetResponse(), http.MethodPut, post.Result.Number, body)
				wantStatus := http.StatusBadRequest
				if role == enum.RoleVisitor || role == enum.RoleHelper {
					wantStatus = http.StatusForbidden
				} else if slices.Contains(allowed, status) {
					wantStatus = http.StatusOK
				}
				if err != nil || response.Code != wantStatus {
					t.Fatalf("response %s: %v, status %d; want %d; body %s", status.Name(), err, response.Code, wantStatus, response.Body.String())
				}
				var persisted enum.PostStatus
				if err := dbx.Connection().QueryRow(`SELECT status FROM posts WHERE id = $1`, post.Result.ID).Scan(&persisted); err != nil {
					t.Fatal(err)
				}
				wantPersisted := enum.PostOpen
				if wantStatus == http.StatusOK {
					wantPersisted = status
				}
				if persisted != wantPersisted {
					t.Fatalf("response persisted %v; want %v", persisted, wantPersisted)
				}
			}
		})
	}
}

func TestPostPermissionsWorkflowHelperTagDeadlines(t *testing.T) {
	f := newPostWorkflow(t)
	post := &cmd.AddNewPost{Title: "Helper tag deadline", Description: "Target"}
	public := &cmd.AddNewTag{Name: "Target public tag", IsPublic: true}
	private := &cmd.AddNewTag{Name: "Target private tag", IsPublic: false}
	first := &cmd.AddNewTag{Name: "First public tag", IsPublic: true}
	if err := bus.Dispatch(f.ctx, post, public, private, first); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name     string
		age      time.Duration
		firstAge time.Duration
		private  bool
		canTag   bool
	}{
		{"new untagged", time.Hour, 0, false, true},
		{"before post deadline", 7*24*time.Hour - time.Minute, 0, false, true},
		{"after post deadline", 7*24*time.Hour + time.Minute, 0, false, false},
		{"before first tag deadline", 3 * 24 * time.Hour, 24*time.Hour - time.Minute, false, true},
		{"after first tag deadline", 3 * 24 * time.Hour, 24*time.Hour + time.Minute, false, false},
		{"private target", time.Hour, 0, true, true},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if _, err := dbx.Connection().Exec(`DELETE FROM post_tags WHERE post_id = $1`, post.Result.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := dbx.Connection().Exec(`UPDATE posts SET created_at = $1 WHERE id = $2`, time.Now().Add(-test.age), post.Result.ID); err != nil {
				t.Fatal(err)
			}
			if test.firstAge != 0 {
				if _, err := dbx.Connection().Exec(`INSERT INTO post_tags
					(post_id, tag_id, tenant_id, created_by_id, created_at) VALUES ($1, $2, $3, $4, $5)`,
					post.Result.ID, first.Result.ID, f.tenant.ID, f.user.ID, time.Now().Add(-test.firstAge)); err != nil {
					t.Fatal(err)
				}
			}

			viewer := f
			viewer.user = &entity.User{ID: 2, Role: enum.RoleHelper, Status: enum.UserActive}
			response, err := viewer.request(api.GetPost(), http.MethodGet, post.Result.Number, "")
			if err != nil || response.Code != http.StatusOK {
				t.Fatalf("get tag target: %v, status %d", err, response.Code)
			}
			var projected entity.Post
			if err := json.Unmarshal(response.Body.Bytes(), &projected); err != nil {
				t.Fatal(err)
			}
			if projected.Permissions.Tag != test.canTag {
				t.Fatalf("post tag permission %v; want %v", projected.Permissions.Tag, test.canTag)
			}

			tag := public.Result
			if test.private {
				tag = private.Result
			}
			params := web.StringMap{"number": fmt.Sprint(post.Result.Number), "slug": tag.Slug}
			body := `{"permissions":{"tag":true},"tag":{"isPublic":true,"permissions":{"assign":true}}}`
			response, err = viewer.requestWithParams(api.AssignTag(), http.MethodPost, "/api/posts/tags", body, params)
			wantStatus := http.StatusForbidden
			if test.canTag && !test.private {
				wantStatus = http.StatusOK
			}
			if err != nil || response.Code != wantStatus {
				t.Fatalf("assign tag: %v, status %d; want %d; body %s", err, response.Code, wantStatus, response.Body.String())
			}
			assigned := workflowCount(t, `SELECT COUNT(*) FROM post_tags WHERE post_id = $1 AND tag_id = $2`, post.Result.ID, tag.ID)
			if (assigned == 1) != (wantStatus == http.StatusOK) {
				t.Fatalf("persisted target tags %d after status %d", assigned, response.Code)
			}
		})
	}
}

func TestPostPermissionsWorkflowRejectsMalformedResponse(t *testing.T) {
	f := newPostWorkflow(t)
	post := &cmd.AddNewPost{Title: "Response input boundary", Description: "Target"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}
	if _, err := dbx.Connection().Exec(`UPDATE posts SET status = $1 WHERE id = $2`, enum.PostStarted, post.Result.ID); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name string
		body string
	}{
		{name: "malformed JSON", body: `{"status":`},
		{name: "unknown status", body: `{"status":"not-a-status"}`},
		{name: "numeric status", body: `{"status":0}`},
		{name: "missing status", body: `{}`},
		{name: "null status", body: `{"status":null}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			response, err := f.request(api.SetResponse(), http.MethodPut, post.Result.Number, test.body)
			if err != nil || response.Code != http.StatusBadRequest {
				t.Fatalf("invalid response: %v, status %d; want 400", err, response.Code)
			}
			var status enum.PostStatus
			if err := dbx.Connection().QueryRow(`SELECT status FROM posts WHERE id = $1`, post.Result.ID).Scan(&status); err != nil {
				t.Fatal(err)
			}
			if status != enum.PostStarted {
				t.Fatalf("invalid response changed status to %v", status)
			}
		})
	}
}

func TestPostPermissionsWorkflowEmbeddedPostsUseCurrentViewer(t *testing.T) {
	f := newPostWorkflow(t)
	post := &cmd.AddNewPost{Title: "Embedded authorization target", Description: "Target"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}
	page := &cmd.CreatePage{
		Title:      "Embedded permission context",
		Slug:       "embedded-permission-context",
		Content:    fmt.Sprintf("<post id=%d />", post.Result.ID),
		Status:     entity.PageStatusPublished,
		Visibility: entity.PageVisibilityPublic,
	}
	if err := bus.Dispatch(f.ctx, page); err != nil {
		t.Fatal(err)
	}

	visitor := &entity.User{ID: 2, Role: enum.RoleVisitor, Status: enum.UserActive}
	ctx := context.WithValue(f.ctx, app.UserCtxKey, visitor)
	assertViewerPermissions := func(t *testing.T) {
		t.Helper()
		getPage := &query.GetPageByID{ID: page.Result.ID}
		getPost := &query.GetPostByID{PostID: post.Result.ID}
		if err := bus.Dispatch(ctx, getPage, getPost); err != nil {
			t.Fatal(err)
		}
		if len(getPage.Result.EmbeddedPosts) != 1 {
			t.Fatalf("embedded posts %d; want one", len(getPage.Result.EmbeddedPosts))
		}
		embedded := getPage.Result.EmbeddedPosts[0]
		if !reflect.DeepEqual(embedded.Permissions, getPost.Result.Permissions) {
			t.Fatalf("embedded permissions %+v; current viewer permissions %+v", embedded.Permissions, getPost.Result.Permissions)
		}
		if !embedded.Permissions.Vote || embedded.Permissions.Edit {
			t.Fatalf("visitor must be able to vote and unable to edit: %+v", embedded.Permissions)
		}
	}

	t.Run("cache created by administrator", assertViewerPermissions)

	legacyPost, err := json.Marshal(post.Result)
	if err != nil {
		t.Fatal(err)
	}
	var legacyFields map[string]any
	if err := json.Unmarshal(legacyPost, &legacyFields); err != nil {
		t.Fatal(err)
	}
	delete(legacyFields, "permissions")
	delete(legacyFields, "discussionPermissions")
	legacyCache, err := json.Marshal(map[string]any{
		"posts":   []any{legacyFields},
		"postIds": []int{post.Result.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dbx.Connection().Exec(`UPDATE pages SET cached_embedded_data = $1::jsonb WHERE id = $2`, string(legacyCache), page.Result.ID); err != nil {
		t.Fatal(err)
	}

	t.Run("cache predates permission projection", assertViewerPermissions)

	if _, err := dbx.Connection().Exec(`UPDATE posts SET moderation_pending = TRUE WHERE id = $1`, post.Result.ID); err != nil {
		t.Fatal(err)
	}
	getPage := &query.GetPageByID{ID: page.Result.ID}
	if err := bus.Dispatch(ctx, getPage); err != nil {
		t.Fatal(err)
	}
	if len(getPage.Result.EmbeddedPosts) != 0 {
		t.Fatal("embedded cache exposed a post hidden from the current viewer")
	}
}

func TestPostPermissionsWorkflowEmbeddedSelectionRemainsStable(t *testing.T) {
	f := newPostWorkflow(t)
	first := &cmd.AddNewPost{Title: "First selected post", Description: "First"}
	second := &cmd.AddNewPost{Title: "Second selected post", Description: "Second"}
	if err := bus.Dispatch(f.ctx, first, second); err != nil {
		t.Fatal(err)
	}
	page := &cmd.CreatePage{
		Title:      "Selected posts",
		Slug:       "selected-posts",
		Content:    `<table type=posts filters="status:open" limit=2 />`,
		Status:     entity.PageStatusPublished,
		Visibility: entity.PageVisibilityPublic,
	}
	if err := bus.Dispatch(f.ctx, page); err != nil {
		t.Fatal(err)
	}

	var cacheJSON []byte
	if err := dbx.Connection().QueryRow(`SELECT cached_embedded_data FROM pages WHERE id = $1`, page.Result.ID).Scan(&cacheJSON); err != nil {
		t.Fatal(err)
	}
	var cache map[string]json.RawMessage
	if err := json.Unmarshal(cacheJSON, &cache); err != nil {
		t.Fatal(err)
	}
	if _, containsViewerPosts := cache["posts"]; containsViewerPosts {
		t.Fatal("embedded selection cache retained viewer-specific Post records")
	}

	newMatch := &cmd.AddNewPost{Title: "Later matching post", Description: "New result"}
	if err := bus.Dispatch(f.ctx, newMatch); err != nil {
		t.Fatal(err)
	}
	selection := []int{second.Result.ID, first.Result.ID, second.Result.ID}
	selectedIDs, err := json.Marshal(selection)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dbx.Connection().Exec(`UPDATE pages
		SET cached_embedded_data = jsonb_set(cached_embedded_data, '{postIds}', $1::jsonb)
		WHERE id = $2`, string(selectedIDs), page.Result.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := dbx.Connection().Exec(`UPDATE posts SET status = $1 WHERE id = $2`, enum.PostArchived, first.Result.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := dbx.Connection().Exec(`UPDATE posts SET status = $1 WHERE id = $2`, enum.PostDuplicate, second.Result.ID); err != nil {
		t.Fatal(err)
	}

	byID := &query.GetPageByID{ID: page.Result.ID}
	bySlug := &query.GetPageBySlug{Slug: page.Slug}
	if err := bus.Dispatch(f.ctx, byID, bySlug); err != nil {
		t.Fatal(err)
	}
	for _, projection := range []*entity.Page{byID.Result, bySlug.Result} {
		ids := make([]int, len(projection.EmbeddedPosts))
		for i, post := range projection.EmbeddedPosts {
			ids[i] = post.ID
		}
		if !slices.Equal(ids, selection) {
			t.Fatalf("embedded selection %v; want stored order %v", ids, selection)
		}
		if projection.EmbeddedPosts[0].Status != enum.PostDuplicate || projection.EmbeddedPosts[1].Status != enum.PostArchived {
			t.Fatal("embedded selected posts did not acquire current statuses")
		}
	}
}

func TestPostPermissionsWorkflowLockingUserProjection(t *testing.T) {
	f := newPostWorkflow(t)
	post := &cmd.AddNewPost{Title: "Locked post", Description: "Projection target"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}
	if err := bus.Dispatch(f.ctx, &cmd.LockPost{Post: post.Result, LockMessage: "Discussion closed"}); err != nil {
		t.Fatal(err)
	}

	var encoded []byte
	if err := dbx.Connection().QueryRow(`SELECT locked_settings FROM posts WHERE id = $1`, post.Result.ID).Scan(&encoded); err != nil {
		t.Fatal(err)
	}
	var original entity.PostLockedSettings
	if err := json.Unmarshal(encoded, &original); err != nil {
		t.Fatal(err)
	}
	if _, err := dbx.Connection().Exec(`UPDATE users
		SET name = $1, avatar_type = $2, avatar_bkey = $3 WHERE id = $4`,
		"Current staff name", enum.AvatarTypeCustom, "current-avatar", f.user.ID); err != nil {
		t.Fatal(err)
	}

	for _, viewer := range []*entity.User{
		nil,
		{ID: 2, Role: enum.RoleVisitor, Status: enum.UserActive},
		{ID: 2, Role: enum.RoleModerator, Status: enum.UserActive},
		f.user,
	} {
		name := "anonymous"
		if viewer != nil {
			name = viewer.Role.String()
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.WithValue(f.ctx, app.UserCtxKey, viewer)
			getPost := &query.GetPostByID{PostID: post.Result.ID}
			if err := bus.Dispatch(ctx, getPost); err != nil {
				t.Fatal(err)
			}
			getUser := &query.GetUserByID{UserID: f.user.ID}
			if err := bus.Dispatch(ctx, getUser); err != nil {
				t.Fatal(err)
			}

			settings := getPost.Result.LockedSettings
			if settings == nil || !settings.Locked || settings.LockMessage != original.LockMessage || !settings.LockedAt.Equal(original.LockedAt) {
				t.Fatalf("lock details changed: %+v", settings)
			}
			lockedBy := settings.LockedBy
			current := getUser.Result
			if lockedBy == nil {
				t.Fatal("locking user was omitted")
			}
			if lockedBy.ID != current.ID || lockedBy.Name != current.Name || lockedBy.Role != current.Role || lockedBy.Status != current.Status {
				t.Fatalf("locking identity differs from current user: got %+v, want %+v", lockedBy, current)
			}
			if lockedBy.AvatarURL != current.AvatarURL || lockedBy.Permissions != current.Permissions {
				t.Fatalf("locking user display or grants differ: got %+v, want %+v", lockedBy, current)
			}
		})
	}

	if _, err := dbx.Connection().Exec(`UPDATE users SET status = $1 WHERE id = $2`, enum.UserDeleted, 3); err != nil {
		t.Fatal(err)
	}
	for _, missingIdentity := range []struct {
		name string
		id   int
	}{
		{"deleted user", 3},
		{"foreign tenant", 5},
		{"missing user", 999999},
	} {
		t.Run(missingIdentity.name, func(t *testing.T) {
			if _, err := dbx.Connection().Exec(`UPDATE posts
				SET locked_settings = jsonb_set(locked_settings, '{lockedBy,id}', to_jsonb($1::integer))
				WHERE id = $2`, missingIdentity.id, post.Result.ID); err != nil {
				t.Fatal(err)
			}
			getPost := &query.GetPostByID{PostID: post.Result.ID}
			if err := bus.Dispatch(f.ctx, getPost); err != nil {
				t.Fatal(err)
			}
			lockedBy := getPost.Result.LockedSettings.LockedBy
			if lockedBy == nil || lockedBy.ID != missingIdentity.id || lockedBy.Name != "" || lockedBy.Permissions != (entity.UserPermissions{}) {
				t.Fatalf("unavailable locking identity projected data or grants: %+v", lockedBy)
			}
		})
	}
}

func BenchmarkPageEmbeddedPostProjection(b *testing.B) {
	f := newPostWorkflow(b)
	rows, err := dbx.Connection().Query(`INSERT INTO posts
		(tenant_id, user_id, title, slug, description, status, created_at)
		SELECT $1, $2, 'Embedded benchmark ' || n, 'embedded-benchmark-' || n,
			repeat('Page preview content. ', 10), 0, NOW()
		FROM generate_series(1, 100) n
		RETURNING id`, f.tenant.ID, f.user.ID)
	if err != nil {
		b.Fatal(err)
	}
	ids := make([]int, 0, 100)
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			b.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		b.Fatal(err)
	}
	if err := rows.Err(); err != nil {
		b.Fatal(err)
	}

	page := &cmd.CreatePage{
		Title:      "Embedded benchmark page",
		Slug:       "embedded-benchmark-page",
		Content:    "A page with a materialized embedded selection",
		Status:     entity.PageStatusPublished,
		Visibility: entity.PageVisibilityPublic,
	}
	if err := bus.Dispatch(f.ctx, page); err != nil {
		b.Fatal(err)
	}

	for _, test := range []struct {
		name   string
		count  int
		locked bool
	}{
		{"unlocked-0", 0, false},
		{"unlocked-20", 20, false},
		{"unlocked-100", 100, false},
		{"locked-20", 20, true},
		{"locked-100", 100, true},
	} {
		b.Run(test.name, func(b *testing.B) {
			var lockedSettings any
			if test.locked {
				encoded, err := json.Marshal(map[string]any{
					"locked":      true,
					"lockedAt":    time.Now(),
					"lockedBy":    map[string]int{"id": f.user.ID},
					"lockMessage": "Discussion closed",
				})
				if err != nil {
					b.Fatal(err)
				}
				lockedSettings = string(encoded)
			}
			if _, err := dbx.Connection().Exec(`UPDATE posts SET locked_settings = $1::jsonb
				WHERE tenant_id = $2 AND slug LIKE 'embedded-benchmark-%'`, lockedSettings, f.tenant.ID); err != nil {
				b.Fatal(err)
			}

			cache, err := json.Marshal(map[string]any{"postIds": ids[:test.count]})
			if err != nil {
				b.Fatal(err)
			}
			if _, err := dbx.Connection().Exec(`UPDATE pages SET cached_embedded_data = $1::jsonb WHERE id = $2`, string(cache), page.Result.ID); err != nil {
				b.Fatal(err)
			}

			getPage := &query.GetPageByID{ID: page.Result.ID}
			if err := bus.Dispatch(f.ctx, getPage); err != nil {
				b.Fatal(err)
			}
			if len(getPage.Result.EmbeddedPosts) != test.count {
				b.Fatalf("loaded %d embedded posts; want %d", len(getPage.Result.EmbeddedPosts), test.count)
			}
			for _, post := range getPage.Result.EmbeddedPosts {
				if post.IsLocked() != test.locked {
					b.Fatalf("post %d lock state %v; want %v", post.ID, post.IsLocked(), test.locked)
				}
				if test.locked && (post.LockedSettings.LockedBy == nil || post.LockedSettings.LockedBy.ID != f.user.ID) {
					b.Fatalf("post %d did not hydrate the locking user", post.ID)
				}
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				getPage := &query.GetPageByID{ID: page.Result.ID}
				if err := bus.Dispatch(f.ctx, getPage); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
