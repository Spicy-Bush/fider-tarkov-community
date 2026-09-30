package entity_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

// The fixed role rules the defaults must reproduce.
func legacyCan(user *entity.User, tenant *entity.Tenant, permission entity.Permission) bool {
	collaborator := user.Role == enum.RoleAdministrator || user.Role == enum.RoleCollaborator
	moderator := user.Role == enum.RoleModerator
	helper := user.Role == enum.RoleHelper

	switch permission {
	case entity.ManageSettings, entity.ManageProfanity, entity.CreateUsers, entity.ViewDesignSystem, entity.EnableEmailNotifications,
		entity.ManagePageTopics, entity.ManageNavigation, entity.ManageAuthentication, entity.ManageRolePermissions,
		entity.ManageInvitations, entity.ManageBilling, entity.ManageFiles, entity.ExportBackup, entity.ChangeUserRoles:
		return user.IsAdministrator()

	case entity.ReadSettings, entity.ManageAPIKeys, entity.ChangeOwnEmail,
		entity.ManageContentSettings, entity.ManagePages, entity.ManageTags, entity.ManageReportReasons,
		entity.ManageArchive, entity.ManageResponses, entity.ManageSponsorship, entity.ManageWebhooks,
		entity.DeleteUserModeration, entity.BlockUsers, entity.ChangeUserVisualRoles, entity.ReadUserEmails,
		entity.LockPosts, entity.ExportFeedback, entity.BypassContentRestrictions:
		return collaborator

	case entity.ManageMembers, entity.ManageReports, entity.ReadResponses, entity.ReadProfiles, entity.EditUserProfiles, entity.ViewPrivateTags,
		entity.ModerateUsers, entity.ExpireUserModeration, entity.DeletePosts, entity.RespondToPosts,
		entity.ModeratePosts, entity.ViewPostVotes, entity.BypassPostingRateLimits, entity.TagPostsOutsideWindow:
		return collaborator || moderator

	case entity.ManageQueue, entity.TagPosts:
		return collaborator || moderator || helper

	case entity.EditPosts:
		return true

	case entity.CreatePosts:
		if user.IsMuted() {
			return false
		}
		if tenant.GeneralSettings.PostingGloballyDisabled && !collaborator {
			return false
		}
		for _, role := range tenant.GeneralSettings.PostingDisabledFor {
			if role == user.Role.String() {
				return false
			}
		}
		return true
	}

	return false
}

func activeUser(role enum.Role) *entity.User {
	return &entity.User{ID: 1, Role: role, Status: enum.UserActive}
}

func TestVisitorCannotReceiveRolePermissionManagement(t *testing.T) {
	tenant := &entity.Tenant{Status: enum.TenantActive, RolePermissions: entity.RolePermissions{
		enum.RoleVisitor: {entity.ManageRolePermissions: true},
	}}
	if entity.Can(activeUser(enum.RoleVisitor), tenant, entity.ManageRolePermissions) {
		t.Fatal("stored Visitor override granted role management")
	}

	for _, role := range entity.PermissionRoles {
		if entity.RolePermissionLock(activeUser(role), tenant, enum.RoleVisitor, entity.ManageRolePermissions) == "" {
			t.Errorf("%s can delegate role management to Visitors", role)
		}
	}

	_, blocked := tenant.RolePermissions.ApplyChanges(activeUser(enum.RoleAdministrator), tenant, []entity.RolePermissionChange{
		{Role: enum.RoleVisitor, Permission: entity.ManageRolePermissions, Granted: true},
	})
	if blocked == "" {
		t.Fatal("Visitor grant was accepted")
	}
}

func TestDefaultRolePermissionsMatchLegacyRules(t *testing.T) {
	settings := []*entity.GeneralSettings{
		{},
		{PostingGloballyDisabled: true},
		{PostingDisabledFor: []string{"helper", "moderator"}},
	}

	for _, general := range settings {
		tenant := &entity.Tenant{ID: 1, Status: enum.TenantActive, GeneralSettings: general}
		for _, role := range entity.PermissionRoles {
			for _, muted := range []bool{false, true} {
				user := activeUser(role)
				user.Muted = muted
				for permission := range entity.PermissionsFor(user, tenant) {
					if got, want := entity.Can(user, tenant, permission), legacyCan(user, tenant, permission); got != want {
						t.Errorf("%s muted=%v settings=%+v %s: got %v, want %v", role, muted, general, permission, got, want)
					}
				}
			}
		}
	}
}

func TestRolePermissionOverridesReplaceDefaults(t *testing.T) {
	tenant := &entity.Tenant{ID: 1, Status: enum.TenantActive, RolePermissions: entity.RolePermissions{
		enum.RoleHelper:        {entity.ManageReports: true},
		enum.RoleAdministrator: {entity.ManageBilling: false},
	}}

	if !entity.Can(activeUser(enum.RoleHelper), tenant, entity.ManageReports) {
		t.Error("granted override ignored")
	}
	if !entity.Can(activeUser(enum.RoleAdministrator), tenant, entity.ManageBilling) {
		t.Error("a stored override took a permission from administrators")
	}
	for _, permission := range tenant.RolePermissions.GrantedByRole()[enum.RoleAdministrator] {
		if !tenant.RolePermissions.Grants(enum.RoleAdministrator, permission) {
			t.Errorf("administrators lack %s", permission)
		}
	}
	if !entity.Can(activeUser(enum.RoleModerator), tenant, entity.ManageReports) {
		t.Error("unchanged cell lost its default")
	}

	muted := activeUser(enum.RoleHelper)
	muted.Muted = true
	tenant.RolePermissions[enum.RoleHelper][entity.CreatePosts] = true
	if entity.Can(muted, tenant, entity.CreatePosts) {
		t.Error("an override bypassed the mute on creating posts")
	}
}

func TestAuthenticationAdministrationCannotBeDelegated(t *testing.T) {
	for _, role := range entity.PermissionRoles {
		t.Run(role.String(), func(t *testing.T) {
			tenant := &entity.Tenant{RolePermissions: entity.RolePermissions{
				role: {entity.ManageAuthentication: true, entity.ManageRolePermissions: true},
			}}
			viewer := activeUser(role)

			if entity.Can(viewer, tenant, entity.ManageAuthentication) != (role == enum.RoleAdministrator) {
				t.Fatal("an override changed administrator-only authentication access")
			}
			if entity.Can(viewer, tenant, entity.ManageRolePermissions) != (role != enum.RoleVisitor) {
				t.Fatal("matrix editing delegation did not respect the role boundary")
			}

			if role != enum.RoleAdministrator {
				if entity.RolePermissionLock(activeUser(enum.RoleAdministrator), tenant, role, entity.ManageAuthentication) == "" {
					t.Fatal("authentication cell was advertised as editable")
				}
				if blocked := apply(t, tenant, activeUser(enum.RoleAdministrator), entity.RolePermissionChange{
					Role: role, Permission: entity.ManageAuthentication, Granted: true,
				}); blocked == "" {
					t.Fatal("matrix accepted an authentication grant")
				}
			}
		})
	}
}

func TestFullBackupsCannotBeDelegated(t *testing.T) {
	admin := activeUser(enum.RoleAdministrator)
	for _, role := range entity.PermissionRoles {
		t.Run(role.String(), func(t *testing.T) {
			tenant := &entity.Tenant{RolePermissions: entity.RolePermissions{
				role: {entity.ExportBackup: true, entity.ExportFeedback: true},
			}}
			if entity.Can(activeUser(role), tenant, entity.ExportBackup) != (role == enum.RoleAdministrator) {
				t.Fatal("stored override changed administrator-only backup access")
			}
			if !entity.Can(activeUser(role), tenant, entity.ExportFeedback) {
				t.Fatal("feedback export cannot be delegated independently")
			}

			if role != enum.RoleAdministrator {
				if entity.RolePermissionLock(admin, tenant, role, entity.ExportBackup) == "" {
					t.Fatal("backup permission was advertised as editable")
				}
				if blocked := apply(t, tenant, admin, entity.RolePermissionChange{
					Role: role, Permission: entity.ExportBackup, Granted: true,
				}); blocked == "" {
					t.Fatal("matrix accepted a full backup grant")
				}
			}
		})
	}
}

func apply(t *testing.T, tenant *entity.Tenant, viewer *entity.User, changes ...entity.RolePermissionChange) string {
	t.Helper()
	next, blocked := tenant.RolePermissions.ApplyChanges(viewer, tenant, changes)
	if blocked == "" {
		tenant.RolePermissions = next
	}
	return blocked
}

func granted(tenant *entity.Tenant, role enum.Role) []entity.Permission {
	return tenant.RolePermissions.GrantedByRole()[role]
}

func TestApplyRolePermissionChangesFollowsRequirements(t *testing.T) {
	admin := activeUser(enum.RoleAdministrator)
	tenant := &entity.Tenant{ID: 1}

	if blocked := apply(t, tenant, admin,
		entity.RolePermissionChange{Role: enum.RoleVisitor, Permission: entity.CreatePosts},
		entity.RolePermissionChange{Role: enum.RoleVisitor, Permission: entity.EditPosts},
	); blocked != "" {
		t.Fatal(blocked)
	}
	if got := granted(tenant, enum.RoleVisitor); len(got) != 0 {
		t.Fatalf("visitor kept %v", got)
	}

	if blocked := apply(t, tenant, admin, entity.RolePermissionChange{Role: enum.RoleVisitor, Permission: entity.ChangeUserRoles, Granted: true}); blocked != "" {
		t.Fatal(blocked)
	}
	if got := fmt.Sprint(granted(tenant, enum.RoleVisitor)); got != "[changeUserRoles manageMembers readProfiles]" {
		t.Fatalf("granting change roles gave visitors %s", got)
	}

	if blocked := apply(t, tenant, admin, entity.RolePermissionChange{Role: enum.RoleVisitor, Permission: entity.ReadProfiles}); blocked != "" {
		t.Fatal(blocked)
	}
	if got := granted(tenant, enum.RoleVisitor); len(got) != 0 {
		t.Fatalf("revoking view profiles left visitors with %v", got)
	}

	apply(t, tenant, admin,
		entity.RolePermissionChange{Role: enum.RoleVisitor, Permission: entity.CreatePosts, Granted: true},
		entity.RolePermissionChange{Role: enum.RoleVisitor, Permission: entity.EditPosts, Granted: true},
	)
	if _, ok := tenant.RolePermissions[enum.RoleVisitor]; ok {
		t.Fatalf("defaults were stored as overrides: %v", tenant.RolePermissions)
	}
}

func TestApplyRolePermissionChangesRevokesBeforeGrants(t *testing.T) {
	tenant := &entity.Tenant{ID: 1}
	blocked := apply(t, tenant, activeUser(enum.RoleAdministrator),
		entity.RolePermissionChange{Role: enum.RoleVisitor, Permission: entity.ManageSettings, Granted: true},
		entity.RolePermissionChange{Role: enum.RoleVisitor, Permission: entity.ReadSettings},
	)
	if blocked != "" || !tenant.RolePermissions.Grants(enum.RoleVisitor, entity.ReadSettings) || !tenant.RolePermissions.Grants(enum.RoleVisitor, entity.ManageSettings) {
		t.Fatalf("blocked=%q overrides=%v", blocked, tenant.RolePermissions)
	}
}

func TestApplyRolePermissionChangesRespectsLocks(t *testing.T) {
	collaboratorEditor := activeUser(enum.RoleCollaborator)
	base := entity.RolePermissions{enum.RoleCollaborator: {entity.ManageRolePermissions: true}}

	for _, test := range []struct {
		name    string
		viewer  *entity.User
		change  entity.RolePermissionChange
		blocked string
	}{
		{"administrators have every permission", activeUser(enum.RoleAdministrator),
			entity.RolePermissionChange{Role: enum.RoleAdministrator, Permission: entity.ManageBilling}, "Administrators have every permission"},
		{"administrators change any other role", activeUser(enum.RoleAdministrator),
			entity.RolePermissionChange{Role: enum.RoleCollaborator, Permission: entity.ManageTags}, ""},
		{"editors only change roles below their own", collaboratorEditor,
			entity.RolePermissionChange{Role: enum.RoleCollaborator, Permission: entity.ManageTags}, "You can only change roles below your own"},
		{"editors never change administrators", collaboratorEditor,
			entity.RolePermissionChange{Role: enum.RoleAdministrator, Permission: entity.ManageTags}, "Administrators have every permission"},
		{"editors cannot grant what they lack", collaboratorEditor,
			entity.RolePermissionChange{Role: enum.RoleModerator, Permission: entity.ManageBilling, Granted: true}, "You can only change permissions you have"},
		{"editors cannot grant requirements they lack", collaboratorEditor,
			entity.RolePermissionChange{Role: enum.RoleVisitor, Permission: entity.ManagePageTopics, Granted: true}, "You can only change permissions you have"},
		{"editors can pass on what they have", collaboratorEditor,
			entity.RolePermissionChange{Role: enum.RoleHelper, Permission: entity.ManageTags, Granted: true}, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			tenant := &entity.Tenant{ID: 1, RolePermissions: base}
			next, blocked := tenant.RolePermissions.ApplyChanges(test.viewer, tenant, []entity.RolePermissionChange{test.change})
			if blocked != test.blocked {
				t.Fatalf("blocked %q, want %q", blocked, test.blocked)
			}
			if blocked != "" && next != nil {
				t.Fatal("a blocked batch returned changes")
			}
			if fmt.Sprint(tenant.RolePermissions) != fmt.Sprint(base) {
				t.Fatal("applying changes mutated the tenant's permissions")
			}
		})
	}
}

type permissionModel map[enum.Role]map[entity.Permission]bool

func TestRandomRolePermissionEditsKeepInvariants(t *testing.T) {
	permissions := entity.PermissionsFor(activeUser(enum.RoleAdministrator), nil)
	all := make([]entity.Permission, 0, len(permissions))
	for permission := range permissions {
		all = append(all, permission)
	}

	for seed := int64(1); seed <= 200; seed++ {
		random := rand.New(rand.NewSource(seed))
		tenant := &entity.Tenant{ID: 1}
		viewer := activeUser(entity.PermissionRoles[random.Intn(len(entity.PermissionRoles))])
		// Map order is random; sorted so a seed replays.
		sortPermissions(all)

		for step := 0; step < 30; step++ {
			changes := make([]entity.RolePermissionChange, 1+random.Intn(5))
			for i := range changes {
				changes[i] = entity.RolePermissionChange{
					Role:       entity.PermissionRoles[random.Intn(len(entity.PermissionRoles))],
					Permission: all[random.Intn(len(all))],
					Granted:    random.Intn(2) == 0,
				}
			}

			before := snapshot(tenant, all)
			next, blocked := tenant.RolePermissions.ApplyChanges(viewer, tenant, changes)
			if blocked != "" {
				continue
			}
			tenant.RolePermissions = next
			after := snapshot(tenant, all)

			for _, role := range entity.PermissionRoles {
				for _, permission := range all {
					if after[role][permission] {
						for _, requirement := range entity.PermissionRequirements[permission] {
							if !after[role][requirement] {
								t.Fatalf("seed %d step %d: %s holds %s without %s", seed, step, role, permission, requirement)
							}
						}
					}
					if before[role][permission] != after[role][permission] && entity.RolePermissionLock(viewer, &entity.Tenant{RolePermissions: overridesOf(before)}, role, permission) != "" {
						t.Fatalf("seed %d step %d: %s changed locked %s.%s", seed, step, viewer.Role, role, permission)
					}
					if tenant.RolePermissions[role] != nil {
						if stored, ok := tenant.RolePermissions[role][permission]; ok && stored == entity.RolePermissions(nil).Grants(role, permission) {
							t.Fatalf("seed %d step %d: default stored as override for %s.%s", seed, step, role, permission)
						}
					}
				}
			}

			for _, change := range changes {
				if change.Granted && !after[change.Role][change.Permission] {
					t.Fatalf("seed %d step %d: grant of %s.%s lost", seed, step, change.Role, change.Permission)
				}
			}
		}
	}
}

func snapshot(tenant *entity.Tenant, all []entity.Permission) permissionModel {
	model := permissionModel{}
	for _, role := range entity.PermissionRoles {
		model[role] = map[entity.Permission]bool{}
		for _, permission := range all {
			model[role][permission] = tenant.RolePermissions.Grants(role, permission)
		}
	}
	return model
}

func overridesOf(model permissionModel) entity.RolePermissions {
	overrides := entity.RolePermissions{}
	for role, permissions := range model {
		overrides[role] = map[entity.Permission]bool{}
		for permission, granted := range permissions {
			overrides[role][permission] = granted
		}
	}
	return overrides
}

func sortPermissions(all []entity.Permission) {
	for i := 1; i < len(all); i++ {
		for j := i; j > 0 && all[j] < all[j-1]; j-- {
			all[j], all[j-1] = all[j-1], all[j]
		}
	}
}
