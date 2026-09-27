package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/api"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/postcache"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func postVisibilityViewers() []*entity.User {
	return []*entity.User{
		nil,
		{ID: 2, Role: enum.RoleVisitor, Status: enum.UserActive},
		{ID: 2, Role: enum.RoleHelper, Status: enum.UserActive},
		{ID: 2, Role: enum.RoleModerator, Status: enum.UserActive},
		{ID: 2, Role: enum.RoleCollaborator, Status: enum.UserActive},
		{ID: 2, Role: enum.RoleAdministrator, Status: enum.UserActive},
		{ID: 1, Role: enum.RoleVisitor, Status: enum.UserActive},
	}
}

func TestHiddenPostDiscussionVisibility(t *testing.T) {
	f := newPostWorkflow(t)
	post := &cmd.AddNewPost{Title: "Private original title", Description: "Owner visibility"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}

	var commentID int
	if err := dbx.Connection().QueryRow(`
		INSERT INTO comments (tenant_id, post_id, user_id, content, created_at)
		VALUES ($1, $2, 2, 'Private discussion content', NOW()) RETURNING id
	`, f.tenant.ID, post.Result.ID).Scan(&commentID); err != nil {
		t.Fatal(err)
	}

	if _, err := dbx.Connection().Exec(`
		INSERT INTO notifications (tenant_id, user_id, title, link, read, post_id, author_id, created_at, updated_at, comment_id)
		SELECT $1, id, 'Private notification', '/posts/1', FALSE, $2, 2, NOW(), NOW(), $3
		FROM users WHERE tenant_id = $1 AND id IN (1, 2)
	`, f.tenant.ID, post.Result.ID, commentID); err != nil {
		t.Fatal(err)
	}

	if _, err := dbx.Connection().Exec(`
		INSERT INTO user_settings (tenant_id, user_id, key, value)
		SELECT $1, id, 'event_notification_mention', '7'
		FROM users WHERE tenant_id = $1 AND id IN (1, 2)
	`, f.tenant.ID); err != nil {
		t.Fatal(err)
	}
	notifier := &entity.User{ID: 3, Role: enum.RoleAdministrator, Status: enum.UserActive}
	notificationCtx := context.WithValue(f.ctx, app.UserCtxKey, notifier)

	for _, status := range []enum.PostStatus{enum.PostOpen, enum.PostArchived, enum.PostDeleted} {
		for _, hidden := range []bool{false, true} {
			if _, err := dbx.Connection().Exec("UPDATE posts SET status = $1, moderation_pending = $2 WHERE id = $3", status, hidden, post.Result.ID); err != nil {
				t.Fatal(err)
			}

			for index, viewer := range postVisibilityViewers() {
				f.user = viewer
				visible := status != enum.PostDeleted && (!hidden || index >= 3)
				want := http.StatusNotFound
				if visible {
					want = http.StatusOK
				}

				for _, selection := range []string{"", "?ids=" + fmt.Sprint(commentID)} {
					response, err := f.requestWithParams(api.ListDiscussion(), http.MethodGet, "/api/posts/comments"+selection, "", web.StringMap{"number": fmt.Sprint(post.Result.Number)})
					if err != nil || response.Code != want {
						t.Errorf("status %v hidden %v viewer %d list %q: %v HTTP %d, want %d", status, hidden, index, selection, err, response.Code, want)
					}
					if index == 6 && strings.Contains(response.Body.String(), "moderationPending") {
						t.Error("post author learned the hidden status")
					}
				}

				response, err := f.requestWithParams(api.ReadDiscussionComment(), http.MethodGet, "/api/comments/context", "", web.StringMap{"id": fmt.Sprint(commentID)})
				if err != nil || response.Code != want {
					t.Errorf("status %v hidden %v viewer %d context: %v HTTP %d, want %d", status, hidden, index, err, response.Code, want)
				}

				if viewer == nil {
					continue
				}

				ctx := context.WithValue(f.ctx, app.UserCtxKey, viewer)
				count := &query.CountUnreadNotifications{}
				list := &query.GetActiveNotifications{}
				if err := bus.Dispatch(ctx, count); err != nil {
					t.Fatal(err)
				}
				if err := bus.Dispatch(ctx, list); err != nil {
					t.Fatal(err)
				}
				if (count.Result == 1) != visible || (len(list.Result) == 1) != visible || list.TotalCount != count.Result {
					t.Errorf("status %v hidden %v viewer %d notifications: count %d, rows %d", status, hidden, index, count.Result, len(list.Result))
				}

				if _, err := dbx.Connection().Exec("UPDATE users SET role = $1 WHERE id = $2", viewer.Role, viewer.ID); err != nil {
					t.Fatal(err)
				}
				for _, channel := range []enum.NotificationChannel{enum.NotificationChannelWeb, enum.NotificationChannelEmail, enum.NotificationChannelPush} {
					recipients := &query.GetCommentNotificationUsers{
						Notification: &entity.CommentNotification{
							CommentID:  commentID,
							Owner:      entity.PostDiscussion(post.Result).Owner,
							MentionIDs: []int{viewer.ID},
							Edited:     true,
						},
						Channel: channel,
						UserIDs: []int{viewer.ID},
					}
					if err := bus.Dispatch(notificationCtx, recipients); err != nil {
						t.Fatal(err)
					}
					if (len(recipients.Result) == 1) != visible {
						t.Errorf("status %v hidden %v viewer %d channel %v: %d recipients", status, hidden, index, channel, len(recipients.Result))
					}
				}
			}
		}
	}
}

func TestPostVisibilityPrecedesPagination(t *testing.T) {
	f := newPostWorkflow(t)
	if _, err := dbx.Connection().Exec(`
		INSERT INTO posts (tenant_id, user_id, title, slug, description, status, moderation_pending, created_at)
		SELECT $1, 1, 'Needle', 'needle-' || n, 'Identical searchable content', 0,
		       n > 32 OR n % 4 = 0, NOW() + n * INTERVAL '1 second'
		FROM generate_series(1, 48) n
	`, f.tenant.ID); err != nil {
		t.Fatal(err)
	}

	defer postcache.InvalidateTenantRankings(f.tenant.ID)
	for _, search := range []string{"", "&query=needle"} {
		for index, viewer := range postVisibilityViewers() {
			f.user = viewer
			postcache.InvalidateTenantRankings(f.tenant.ID)
			var expected []int
			for id := 48; id >= 1; id-- {
				if index >= 3 || (id <= 32 && id%4 != 0) {
					expected = append(expected, id)
				}
			}

			var received []int
			for offset := 0; offset < len(expected); offset += 15 {
				path := fmt.Sprintf("/api/posts?view=newest&limit=15&offset=%d%s", offset, search)
				response, err := f.requestWithParams(api.SearchPosts(), http.MethodGet, path, "", nil)
				if err != nil || response.Code != http.StatusOK {
					t.Fatalf("page: %v HTTP %d %s", err, response.Code, response.Body)
				}

				var posts []*entity.Post
				if err := json.Unmarshal(response.Body.Bytes(), &posts); err != nil {
					t.Fatal(err)
				}
				for _, post := range posts {
					received = append(received, post.ID)
				}
				if len(posts) != min(15, len(expected)-offset) {
					t.Errorf("viewer %d search %q offset %d: short page of %d", index, search, offset, len(posts))
				}
			}
			if !slices.Equal(received, expected) {
				t.Errorf("viewer %d search %q: IDs %v, want %v", index, search, received, expected)
			}
		}
	}
}
