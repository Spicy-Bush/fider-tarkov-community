package postgres

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

func TestPostProjectionUsesHiddenStateBeforeRedaction(t *testing.T) {
	for _, test := range []struct {
		name        string
		viewer      *entity.User
		canVote     bool
		canComment  bool
		showsHidden bool
	}{
		{name: "anonymous"},
		{name: "other visitor", viewer: &entity.User{ID: 2, Role: enum.RoleVisitor, Status: enum.UserActive}},
		{name: "author", viewer: &entity.User{ID: 1, Role: enum.RoleVisitor, Status: enum.UserActive}, canVote: true, canComment: true},
		{name: "moderator", viewer: &entity.User{ID: 2, Role: enum.RoleModerator, Status: enum.UserActive}, canVote: true, canComment: true, showsHidden: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), app.TenantCtxKey, &entity.Tenant{})
			if test.viewer != nil {
				ctx = context.WithValue(ctx, app.UserCtxKey, test.viewer)
			}

			row := dbPost{
				ID:                1,
				Number:            1,
				Status:            int(enum.PostOpen),
				CreatedAt:         time.Now(),
				ModerationPending: true,
				User:              &dbUser{ID: sql.NullInt64{Int64: 1, Valid: true}},
			}
			post := row.toModel(ctx)

			if post.Permissions.Vote != test.canVote || post.DiscussionPermissions.Comment != test.canComment {
				t.Fatalf("hidden post permissions: actions=%+v discussion=%+v", post.Permissions, post.DiscussionPermissions)
			}
			if post.ModerationPending != test.showsHidden {
				t.Fatalf("hidden marker visible=%t, want %t", post.ModerationPending, test.showsHidden)
			}
		})
	}
}

var mappedPermissionPost *entity.Post

func BenchmarkPostPermissionProjection20(b *testing.B) {
	ctx := context.WithValue(context.Background(), app.TenantCtxKey, &entity.Tenant{})
	ctx = context.WithValue(ctx, app.UserCtxKey, &entity.User{ID: 2, Role: enum.RoleVisitor, Status: enum.UserActive})
	row := dbPost{
		ID:        1,
		Number:    1,
		Title:     "Example post",
		Slug:      "example-post",
		Status:    int(enum.PostOpen),
		CreatedAt: time.Now(),
		User:      &dbUser{ID: sql.NullInt64{Int64: 1, Valid: true}},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 20; j++ {
			mappedPermissionPost = row.toModel(ctx)
		}
	}
}
