package entity_test

import (
	"slices"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

func TestConfiguredContentCapabilitiesPreserveTargetRanks(t *testing.T) {
	now := time.Now()
	for _, role := range entity.PermissionRoles {
		for _, granted := range []bool{false, true} {
			user := &entity.User{ID: 1, Role: role, Status: enum.UserActive}
			tenant := &entity.Tenant{Status: enum.TenantActive, GeneralSettings: &entity.GeneralSettings{PostingGloballyDisabled: true}, RolePermissions: entity.RolePermissions{
				role: {entity.BypassContentRestrictions: granted, entity.BypassPostingRateLimits: granted, entity.TagPostsOutsideWindow: granted, entity.TagPosts: true, entity.ModeratePosts: true, entity.ManageQueue: true},
			}}
			want := granted || role == enum.RoleAdministrator
			if entity.Can(user, tenant, entity.CreatePosts) != want || entity.CanBypassPostingRateLimits(user, tenant) != want {
				t.Fatalf("configured capability ignored for %v/%t", role, granted)
			}
			post := &entity.Post{Status: enum.PostOpen, CreatedAt: now.Add(-8 * 24 * time.Hour)}
			if post.AllowedActions(user, tenant, now).Tag != want {
				t.Fatalf("tagging window override ignored for %v/%t", role, granted)
			}
			if (entity.DefaultQueueDate(user, tenant) == "") != want {
				t.Fatalf("queue default disagreed with configured scope for %v/%t", role, granted)
			}
			if role == enum.RoleHelper && slices.Contains(entity.ModeratedContentRoles(user, tenant), enum.RoleAdministrator) {
				t.Fatal("a content restriction bypass raised moderation target rank")
			}
		}
	}
}
