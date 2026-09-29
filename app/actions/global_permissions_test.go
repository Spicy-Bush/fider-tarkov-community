package actions_test

import (
	"context"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
)

func TestGlobalActionPermissions(t *testing.T) {
	cases := []struct {
		name       string
		authorized func(context.Context, *entity.User) bool
		roles      []enum.Role
	}{
		{
			name:       "pages",
			authorized: (&actions.CreateUpdatePage{}).IsAuthorized,
			roles:      []enum.Role{enum.RoleCollaborator, enum.RoleAdministrator},
		},
		{
			name:       "page topic",
			authorized: (&actions.CreatePageTopic{}).IsAuthorized,
			roles:      []enum.Role{enum.RoleAdministrator},
		},
		{
			name:       "reports",
			authorized: (&actions.AssignReport{}).IsAuthorized,
			roles:      []enum.Role{enum.RoleModerator, enum.RoleCollaborator, enum.RoleAdministrator},
		},
		{
			name:       "report reasons",
			authorized: (&actions.CreateReportReason{}).IsAuthorized,
			roles:      []enum.Role{enum.RoleCollaborator, enum.RoleAdministrator},
		},
		{
			name:       "files",
			authorized: (&actions.UploadNewFile{}).IsAuthorized,
			roles:      []enum.Role{enum.RoleAdministrator},
		},
		{
			name:       "responses",
			authorized: (&actions.CreateCannedResponse{}).IsAuthorized,
			roles:      []enum.Role{enum.RoleCollaborator, enum.RoleAdministrator},
		},
		{
			name:       "tags",
			authorized: (&actions.CreateEditTag{}).IsAuthorized,
			roles:      []enum.Role{enum.RoleCollaborator, enum.RoleAdministrator},
		},
		{
			name:       "invitations",
			authorized: (&actions.InviteUsers{}).IsAuthorized,
			roles:      []enum.Role{enum.RoleAdministrator},
		},
		{
			name:       "navigation",
			authorized: (&actions.SaveNavigationLinks{}).IsAuthorized,
			roles:      []enum.Role{enum.RoleAdministrator},
		},
		{
			name:       "role permissions",
			authorized: (&actions.UpdateRolePermissions{}).IsAuthorized,
			roles:      []enum.Role{enum.RoleAdministrator},
		},
		{
			name:       "oauth",
			authorized: (&actions.CreateEditOAuthConfig{}).IsAuthorized,
			roles:      []enum.Role{enum.RoleAdministrator},
		},
		{
			name:       "sponsorship",
			authorized: (&actions.CreateSponsorshipCampaign{}).IsAuthorized,
			roles:      []enum.Role{enum.RoleCollaborator, enum.RoleAdministrator},
		},
		{
			name:       "profanity",
			authorized: (&actions.UpdateProfanityWords{}).IsAuthorized,
			roles:      []enum.Role{enum.RoleAdministrator},
		},
		{
			name:       "billing",
			authorized: (&actions.GenerateCheckoutLink{}).IsAuthorized,
			roles:      []enum.Role{enum.RoleAdministrator},
		},
		{
			name:       "settings",
			authorized: (&actions.UpdateTenantSettings{}).IsAuthorized,
			roles:      []enum.Role{enum.RoleAdministrator},
		},
		{
			name:       "banner",
			authorized: (&actions.UpdateMessageBanner{}).IsAuthorized,
			roles:      []enum.Role{enum.RoleCollaborator, enum.RoleAdministrator},
		},
		{
			name:       "webhooks",
			authorized: (&actions.CreateEditWebhook{}).IsAuthorized,
			roles:      []enum.Role{enum.RoleCollaborator, enum.RoleAdministrator},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			tenant := &entity.Tenant{ID: 1, Status: enum.TenantActive}
			ctx := context.WithValue(context.Background(), app.TenantCtxKey, tenant)
			if test.authorized(ctx, nil) {
				t.Fatal("anonymous action authorized")
			}

			for _, role := range []enum.Role{enum.RoleVisitor, enum.RoleHelper, enum.RoleModerator, enum.RoleCollaborator, enum.RoleAdministrator} {
				want := false
				for _, allowed := range test.roles {
					want = want || role == allowed
				}
				user := &entity.User{ID: 2, Role: role, Status: enum.UserActive}
				if got := test.authorized(ctx, user); got != want {
					t.Fatalf("role %s authorization %v; want %v", role, got, want)
				}
			}

			for _, status := range []enum.UserStatus{0, 99, enum.UserBlocked, enum.UserDeleted} {
				if test.authorized(ctx, &entity.User{ID: 2, Role: enum.RoleAdministrator, Status: status}) {
					t.Fatalf("administrator with status %d authorized", status)
				}
			}
			for _, status := range []enum.TenantStatus{enum.TenantLocked, enum.TenantDisabled} {
				tenant.Status = status
				if test.authorized(ctx, &entity.User{ID: 2, Role: enum.RoleAdministrator, Status: enum.UserActive}) {
					t.Fatalf("action authorized for %s tenant", status)
				}
			}
		})
	}
}

func TestPageAuthorsRequirePagePermission(t *testing.T) {
	tenant := &entity.Tenant{ID: 1, Status: enum.TenantActive}
	ctx := context.WithValue(context.Background(), app.TenantCtxKey, tenant)
	cases := []struct {
		name   string
		role   enum.Role
		status enum.UserStatus
		valid  bool
	}{
		{"visitor", enum.RoleVisitor, enum.UserActive, false},
		{"helper", enum.RoleHelper, enum.UserActive, false},
		{"moderator", enum.RoleModerator, enum.UserActive, false},
		{"collaborator", enum.RoleCollaborator, enum.UserActive, true},
		{"administrator", enum.RoleAdministrator, enum.UserActive, true},
		{"blocked administrator", enum.RoleAdministrator, enum.UserBlocked, false},
		{"deleted administrator", enum.RoleAdministrator, enum.UserDeleted, false},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			bus.AddHandler(func(ctx context.Context, q *query.GetUsersByIDs) error {
				q.Result = []*entity.User{{ID: 8, Role: test.role, Status: test.status}}
				return nil
			})
			action := &actions.CreateUpdatePage{
				Title:      "Page with an author",
				Content:    "Page content",
				Status:     string(entity.PageStatusDraft),
				Visibility: string(entity.PageVisibilityPublic),
				Authors:    []int{8},
			}

			result := action.Validate(ctx, &entity.User{ID: 1, Role: enum.RoleAdministrator, Status: enum.UserActive})
			if result.Ok != test.valid {
				t.Fatalf("valid %v, want %v: %+v", result.Ok, test.valid, result.Errors)
			}
		})
	}
}
