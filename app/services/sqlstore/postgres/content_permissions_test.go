package postgres_test

import (
	"context"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
)

func TestContentReadsHonorModerationGrants(t *testing.T) {
	f := newPostWorkflow(t)
	hidden := &cmd.AddNewPost{Title: "Hidden policy target", Description: "Private content"}
	visible := &cmd.AddNewPost{Title: "Visible discussion", Description: "Public content"}
	if err := bus.Dispatch(f.ctx, hidden, visible); err != nil {
		t.Fatal(err)
	}
	if _, err := mediaFixtureSQL(`UPDATE posts SET moderation_pending=TRUE, moderation_data='{"reason":"private review"}' WHERE id=$1`, hidden.Result.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := mediaFixtureSQL(`
		INSERT INTO comments (tenant_id, post_id, user_id, content, created_at, moderation_pending)
		VALUES ($1, $2, 1, 'Hidden comment', NOW(), TRUE)
	`, f.tenant.ID, visible.Result.ID); err != nil {
		t.Fatal(err)
	}

	roles := []enum.Role{enum.RoleVisitor, enum.RoleHelper, enum.RoleModerator, enum.RoleCollaborator, enum.RoleAdministrator}
	for viewerRank, role := range roles {
		for _, granted := range []bool{false, true} {
			f.tenant.RolePermissions = entity.RolePermissions{role: {entity.ModeratePosts: granted}}
			viewer := &entity.User{ID: 2, Role: role, Status: enum.UserActive}
			ctx := context.WithValue(f.ctx, app.UserCtxKey, viewer)
			wantPost := granted || role == enum.RoleAdministrator
			post := &query.GetPostByID{PostID: hidden.Result.ID}
			err := bus.Dispatch(ctx, post)
			if wantPost {
				if err != nil || !post.Result.ModerationPending || post.Result.ModerationData == "" {
					t.Fatalf("%s grant=%v hidden post: %+v, %v", role, granted, post.Result, err)
				}
			} else if errors.Cause(err) != app.ErrNotFound {
				t.Fatalf("%s grant=%v hidden post returned %v", role, granted, err)
			}

			for authorRank, authorRole := range roles {
				if _, err := mediaFixtureSQL(`UPDATE users SET role=$1 WHERE id=1`, authorRole); err != nil {
					t.Fatal(err)
				}
				content := &query.SearchUserContent{UserID: 1, ContentType: "comments", Limit: 100}
				if err := bus.Dispatch(ctx, content); err != nil {
					t.Fatal(err)
				}
				wantComment := role == enum.RoleAdministrator || (granted && (role == enum.RoleCollaborator || authorRank < viewerRank))
				if got := len(content.Result.Comments) == 1; got != wantComment {
					t.Fatalf("%s reads %s comment grant=%v: got %v want %v", role, authorRole, granted, got, wantComment)
				}
			}
		}
	}
}
