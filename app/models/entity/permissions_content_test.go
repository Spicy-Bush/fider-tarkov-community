package entity_test

import (
	"slices"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

func TestContentModerationFollowsCurrentGrants(t *testing.T) {
	for _, role := range []enum.Role{enum.RoleVisitor, enum.RoleHelper, enum.RoleModerator, enum.RoleCollaborator} {
		for _, granted := range []bool{false, true} {
			viewer := &entity.User{ID: 10, Role: role, Status: enum.UserActive}
			tenant := &entity.Tenant{Status: enum.TenantActive, RolePermissions: entity.RolePermissions{
				role: {entity.ModeratePosts: granted},
			}}
			post := &entity.Post{ID: 1, Status: enum.PostOpen, ModerationPending: true, User: &entity.User{ID: 20, Role: enum.RoleVisitor}}
			if got := entity.PostDiscussion(post).CanView(viewer, tenant); got != granted {
				t.Errorf("%s grant=%v hidden post visibility=%v", role, granted, got)
			}

			post.ModerationPending = false
			discussion := entity.PostDiscussion(post)
			comment := &entity.Comment{User: &entity.User{ID: 20, Role: enum.RoleVisitor}, ModerationPending: true}
			want := granted && role != enum.RoleVisitor
			if got := comment.AllowedActions(viewer, discussion, tenant, time.Now()).Moderate; got != want {
				t.Errorf("%s grant=%v moderates visitor=%v, want %v", role, granted, got, want)
			}
			comment.User = viewer
			if got := comment.ContentState(viewer, discussion, tenant); got != "visible" {
				t.Errorf("%s grant=%v cannot read own comment", role, granted)
			}
			if got := comment.AllowedActions(viewer, discussion, tenant, time.Now()).Moderate; got != granted {
				t.Errorf("%s grant=%v own moderation=%v", role, granted, got)
			}
		}
	}
}

func TestModerationTargetsAcrossRoles(t *testing.T) {
	roles := []enum.Role{enum.RoleVisitor, enum.RoleHelper, enum.RoleModerator, enum.RoleCollaborator, enum.RoleAdministrator}
	for viewerRank, role := range roles {
		for authorRank, authorRole := range roles {
			for _, granted := range []bool{false, true} {
				viewer := &entity.User{ID: 10, Role: role, Status: enum.UserActive}
				tenant := &entity.Tenant{Status: enum.TenantActive, RolePermissions: entity.RolePermissions{
					role: {entity.ModeratePosts: granted},
				}}
				comment := &entity.Comment{User: &entity.User{ID: 20, Role: authorRole}, ModerationPending: true}
				discussion := entity.PostDiscussion(&entity.Post{ID: 1, Status: enum.PostOpen})
				want := role == enum.RoleAdministrator || (granted && (role == enum.RoleCollaborator || authorRank < viewerRank))
				permissions := comment.AllowedActions(viewer, discussion, tenant, time.Now())
				if permissions.Moderate != want || (comment.ContentState(viewer, discussion, tenant) == "visible") != want {
					t.Errorf("%s moderates %s grant=%v: actions=%+v, want %v", role, authorRole, granted, permissions, want)
				}
				if got := slices.Contains(entity.ModeratedContentRoles(viewer, tenant), authorRole); got != want {
					t.Errorf("SQL target list for %s/%s grant=%v: got %v want %v", role, authorRole, granted, got, want)
				}

				viewer.Status = enum.UserBlocked
				if len(entity.ModeratedContentRoles(viewer, tenant)) != 0 || comment.AllowedActions(viewer, discussion, tenant, time.Now()).Moderate {
					t.Fatal("blocked viewer retained moderation")
				}
			}
		}
	}
}

func TestDelegatedPostEditingKeepsOwnContentBoundary(t *testing.T) {
	now := time.Now()
	post := &entity.Post{Status: enum.PostOpen, CreatedAt: now, User: &entity.User{ID: 20, Role: enum.RoleVisitor}}
	for _, test := range []struct {
		role     enum.Role
		edit     bool
		moderate bool
		want     bool
	}{
		{role: enum.RoleHelper, edit: true},
		{role: enum.RoleHelper, edit: true, moderate: true, want: true},
		{role: enum.RoleHelper, moderate: true},
		{role: enum.RoleModerator, edit: true, want: true},
		{role: enum.RoleModerator, moderate: true},
	} {
		viewer := &entity.User{ID: 10, Role: test.role, Status: enum.UserActive}
		tenant := &entity.Tenant{Status: enum.TenantActive, RolePermissions: entity.RolePermissions{
			test.role: {entity.EditPosts: test.edit, entity.ModeratePosts: test.moderate},
		}}
		if got := post.AllowedActions(viewer, tenant, now).Edit; got != test.want {
			t.Errorf("%s edit=%v moderate=%v: got %v want %v", test.role, test.edit, test.moderate, got, test.want)
		}
	}
}
