package entity_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

func TestPostPermissionRoles(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	tenant := &entity.Tenant{Status: enum.TenantActive}

	roles := []struct {
		role     enum.Role
		edit     bool
		delete   bool
		lock     bool
		moderate bool
		tag      bool
		respond  []enum.PostStatus
	}{
		{
			role:    enum.RoleVisitor,
			respond: []enum.PostStatus{},
		},
		{
			role:    enum.RoleHelper,
			tag:     true,
			respond: []enum.PostStatus{},
		},
		{
			role:     enum.RoleModerator,
			edit:     true,
			delete:   true,
			moderate: true,
			tag:      true,
			respond:  []enum.PostStatus{enum.PostDuplicate},
		},
		{
			role:     enum.RoleCollaborator,
			edit:     true,
			delete:   true,
			lock:     true,
			moderate: true,
			tag:      true,
			respond: []enum.PostStatus{
				enum.PostOpen, enum.PostStarted, enum.PostCompleted,
				enum.PostDeclined, enum.PostPlanned, enum.PostDuplicate,
			},
		},
		{
			role:     enum.RoleAdministrator,
			edit:     true,
			delete:   true,
			lock:     true,
			moderate: true,
			tag:      true,
			respond: []enum.PostStatus{
				enum.PostOpen, enum.PostStarted, enum.PostCompleted,
				enum.PostDeclined, enum.PostPlanned, enum.PostDuplicate,
			},
		},
	}

	for _, viewer := range roles {
		for _, author := range []enum.Role{enum.RoleVisitor, enum.RoleHelper, enum.RoleModerator, enum.RoleCollaborator, enum.RoleAdministrator} {
			t.Run(viewer.role.String()+"/"+author.String(), func(t *testing.T) {
				post := &entity.Post{
					Status:    enum.PostOpen,
					CreatedAt: now,
					User:      &entity.User{ID: 2, Role: author},
				}
				user := &entity.User{ID: 1, Role: viewer.role, Status: enum.UserActive}
				want := entity.PostPermissions{
					Edit:      viewer.edit,
					Delete:    viewer.delete,
					Respond:   viewer.respond,
					Lock:      viewer.lock,
					Archive:   viewer.lock,
					Moderate:  viewer.moderate,
					Tag:       viewer.tag,
					Report:    true,
					ViewVotes: viewer.moderate,
					Vote:      true,
					Follow:    true,
				}

				if viewer.role == enum.RoleModerator && author != enum.RoleVisitor && author != enum.RoleHelper {
					want.Edit = false
					want.Delete = false
				}

				if got := post.AllowedActions(user, tenant, now); !reflect.DeepEqual(got, want) {
					t.Fatalf("permissions = %+v, want %+v", got, want)
				}
			})
		}
	}
}

func TestPostPermissionTransitions(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	user := &entity.User{ID: 1, Role: enum.RoleVisitor, Status: enum.UserActive}
	tenant := &entity.Tenant{Status: enum.TenantActive, GeneralSettings: &entity.GeneralSettings{}}
	post := &entity.Post{CreatedAt: now.Add(-time.Hour), User: user, Status: enum.PostOpen}

	if permissions := post.AllowedActions(user, tenant, now); !permissions.Edit || permissions.Report {
		t.Fatalf("own post at edit boundary: %+v", permissions)
	}

	if post.AllowedActions(user, tenant, now.Add(time.Nanosecond)).Edit {
		t.Fatal("edit permission survived its deadline")
	}

	post.LockedSettings = &entity.PostLockedSettings{Locked: true}
	if permissions := post.AllowedActions(user, tenant, now); permissions.Edit || permissions.Vote || permissions.Follow {
		t.Fatalf("locked post offers a blocked operation: %+v", permissions)
	}

	user.Role = enum.RoleCollaborator
	if permissions := post.AllowedActions(user, tenant, now); !permissions.Edit || !permissions.Vote || !permissions.Follow {
		t.Fatalf("collaborator lost locked-post access: %+v", permissions)
	}

	user.Muted = true
	if post.AllowedActions(user, tenant, now).Vote {
		t.Fatal("muted user can vote")
	}

	user.Muted = false
	for _, status := range []enum.PostStatus{enum.PostCompleted, enum.PostDeclined, enum.PostDuplicate} {
		post.Status = status
		if post.AllowedActions(user, tenant, now).Vote {
			t.Fatalf("voting allowed for status %d", status)
		}
	}

	post.Status = enum.PostDeleted
	if permissions := post.AllowedActions(user, tenant, now); permissions.Edit || permissions.Moderate || len(permissions.Respond) != 0 {
		t.Fatalf("deleted post offers an operation: %+v", permissions)
	}

	post.Status = enum.PostOpen
	for _, status := range []enum.UserStatus{0, 99, enum.UserBlocked, enum.UserDeleted} {
		user.Status = status
		if permissions := post.AllowedActions(user, tenant, now); permissions.Edit || permissions.Lock || permissions.Moderate {
			t.Fatalf("inactive user offers privileged operations: %+v", permissions)
		}
	}
}

func TestTagPermissionBoundaries(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	user := &entity.User{ID: 1, Role: enum.RoleHelper, Status: enum.UserActive}
	post := &entity.Post{CreatedAt: now.Add(-7 * 24 * time.Hour), Status: enum.PostOpen}
	public := &entity.Tag{IsPublic: true}
	private := &entity.Tag{}

	if !post.AllowedActions(user, nil, now).Tag || !public.AllowedActions(user, nil).Assign {
		t.Fatal("helper cannot assign a public tag at the post-age boundary")
	}

	if post.AllowedActions(user, nil, now.Add(time.Nanosecond)).Tag || private.AllowedActions(user, nil).Assign {
		t.Fatal("helper can assign a private tag or tag an old post")
	}

	firstTagged := now.Add(-24 * time.Hour)
	post.CreatedAt = now
	post.FirstTaggedAt = &firstTagged
	if !post.AllowedActions(user, nil, now).Tag || post.AllowedActions(user, nil, now.Add(time.Nanosecond)).Tag {
		t.Fatal("helper tagging deadline does not follow the first tag")
	}

	user.Role = enum.RoleModerator
	if !post.AllowedActions(user, nil, now.Add(30 * 24 * time.Hour)).Tag || !private.AllowedActions(user, nil).Assign {
		t.Fatal("moderator incorrectly inherits helper tag restrictions")
	}
}
