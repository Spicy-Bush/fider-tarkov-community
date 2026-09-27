package postgres_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/api"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestUserTargetProfileAndMutations(t *testing.T) {
	f := newPostWorkflow(t)
	for _, test := range []struct {
		name     string
		viewer   enum.Role
		target   enum.Role
		edit     bool
		block    bool
		moderate bool
		remove   bool
		role     bool
		visual   bool
	}{
		{
			name:   "visitor",
			viewer: enum.RoleVisitor,
			target: enum.RoleVisitor,
		},
		{
			name:   "helper",
			viewer: enum.RoleHelper,
			target: enum.RoleVisitor,
		},
		{
			name:     "moderator visitor",
			viewer:   enum.RoleModerator,
			target:   enum.RoleVisitor,
			edit:     true,
			moderate: true,
		},
		{
			name:     "moderator helper",
			viewer:   enum.RoleModerator,
			target:   enum.RoleHelper,
			edit:     true,
			moderate: true,
		},
		{
			name:   "moderator collaborator",
			viewer: enum.RoleModerator,
			target: enum.RoleCollaborator,
		},
		{
			name:     "collaborator helper",
			viewer:   enum.RoleCollaborator,
			target:   enum.RoleHelper,
			edit:     true,
			block:    true,
			moderate: true,
			remove:   true,
			visual:   true,
		},
		{
			name:     "collaborator moderator",
			viewer:   enum.RoleCollaborator,
			target:   enum.RoleModerator,
			edit:     true,
			moderate: true,
			remove:   true,
			visual:   true,
		},
		{
			name:   "collaborator peer",
			viewer: enum.RoleCollaborator,
			target: enum.RoleCollaborator,
			edit:   true,
			visual: true,
		},
		{
			name:     "administrator helper",
			viewer:   enum.RoleAdministrator,
			target:   enum.RoleHelper,
			edit:     true,
			block:    true,
			moderate: true,
			remove:   true,
			role:     true,
			visual:   true,
		},
		{
			name:   "administrator peer",
			viewer: enum.RoleAdministrator,
			target: enum.RoleAdministrator,
			edit:   true,
			role:   true,
			visual: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			f.user.Role = test.viewer
			if _, err := dbx.Connection().Exec(`UPDATE users SET role = $1, name = 'Original name',
				status = 1, avatar_type = 2, visual_role = 0 WHERE id = 2`, test.target); err != nil {
				t.Fatal(err)
			}

			request := func(handler web.HandlerFunc, method, body string, params web.StringMap, allowed bool) []byte {
				t.Helper()
				response, err := f.requestWithParams(handler, method, "http://localhost:3000/api/users/2", body, params)
				if err != nil {
					t.Fatal(err)
				}
				if allowed && response.Code != http.StatusOK {
					t.Fatalf("allowed mutation returned %d: %s", response.Code, response.Body.String())
				}
				if !allowed && response.Code != http.StatusUnauthorized && response.Code != http.StatusForbidden {
					t.Fatalf("forbidden mutation returned %d: %s", response.Code, response.Body.String())
				}
				return response.Body.Bytes()
			}

			response, err := f.requestWithParams(api.ListUsers(), http.MethodGet, "http://localhost:3000/api/users", "", nil)
			if err != nil || response.Code != http.StatusOK {
				t.Fatalf("member projection: %v, %s", err, response.Body.String())
			}
			var users []entity.User
			if err := json.Unmarshal(response.Body.Bytes(), &users); err != nil {
				t.Fatal(err)
			}
			var projected *entity.User
			for i := range users {
				if users[i].ID == 2 {
					projected = &users[i]
				}
			}
			if projected == nil {
				t.Fatal("member projection omitted target")
			}
			want := entity.UserPermissions{
				ReadProfile:      test.viewer != enum.RoleVisitor && test.viewer != enum.RoleHelper,
				EditName:         test.edit,
				EditAvatar:       test.edit,
				Block:            test.block,
				Moderate:         test.moderate,
				DeleteModeration: test.remove,
				ExpireModeration: test.moderate,
				ChangeRole:       test.role,
				ChangeVisualRole: test.visual,
			}
			if projected.Permissions != want {
				t.Fatalf("projected capabilities %+v, want %+v", projected.Permissions, want)
			}

			params := web.StringMap{"userID": "2"}
			request(handlers.UpdateUserName(), http.MethodPost, `{"name":"Confirmed name"}`, params, test.edit)
			nameChanged := workflowCount(t, "SELECT count(*) FROM users WHERE id = 2 AND name = 'Confirmed name'") == 1
			if nameChanged != test.edit {
				t.Fatal("name mutation disagrees with capability")
			}

			request(handlers.UpdateUserAvatar(), http.MethodPost, `{"avatarType":"letter"}`, params, test.edit)
			avatarChanged := workflowCount(t, "SELECT count(*) FROM users WHERE id = 2 AND avatar_type = 1") == 1
			if avatarChanged != test.edit {
				t.Fatal("avatar mutation disagrees with capability")
			}

			request(handlers.BlockUser(), http.MethodPut, "", params, test.block)
			blocked := workflowCount(t, "SELECT count(*) FROM users WHERE id = 2 AND status = 3") == 1
			if blocked != test.block {
				t.Fatal("block mutation disagrees with capability")
			}
			request(handlers.UnblockUser(), http.MethodDelete, "", params, test.block)
			if workflowCount(t, "SELECT count(*) FROM users WHERE id = 2 AND status = 1") != 1 {
				t.Fatal("unblock did not restore active state")
			}

			for _, handler := range []web.HandlerFunc{handlers.WarnUser(), handlers.MuteUser()} {
				request(handler, http.MethodPost, `{"reason":"Review fixture","duration":"1h"}`, params, test.moderate)
			}

			var warningID, muteID int
			if err := dbx.Connection().QueryRow(`INSERT INTO user_warnings
				(user_id, tenant_id, reason, created_by) VALUES (2, 1, 'Standing fixture', 1) RETURNING id`).Scan(&warningID); err != nil {
				t.Fatal(err)
			}
			if err := dbx.Connection().QueryRow(`INSERT INTO user_mutes
				(user_id, tenant_id, reason, created_by) VALUES (2, 1, 'Standing fixture', 1) RETURNING id`).Scan(&muteID); err != nil {
				t.Fatal(err)
			}
			params["warningID"] = fmt.Sprint(warningID)
			params["muteID"] = fmt.Sprint(muteID)
			request(handlers.ExpireWarning(), http.MethodPost, "", params, test.moderate)
			request(handlers.ExpireMute(), http.MethodPost, "", params, test.moderate)
			warningExpired := workflowCount(t, "SELECT count(*) FROM user_warnings WHERE id = $1 AND expires_at <= NOW()", warningID) == 1
			muteExpired := workflowCount(t, "SELECT count(*) FROM user_mutes WHERE id = $1 AND expires_at <= NOW()", muteID) == 1
			if warningExpired != test.moderate || muteExpired != test.moderate {
				t.Fatal("expiry mutation disagrees with capability")
			}

			request(handlers.DeleteWarning(), http.MethodDelete, "", params, test.remove)
			request(handlers.DeleteMute(), http.MethodDelete, "", params, test.remove)
			warningRemoved := workflowCount(t, "SELECT count(*) FROM user_warnings WHERE id = $1", warningID) == 0
			muteRemoved := workflowCount(t, "SELECT count(*) FROM user_mutes WHERE id = $1", muteID) == 0
			if warningRemoved != test.remove || muteRemoved != test.remove {
				t.Fatal("moderation deletion disagrees with capability")
			}

			params["visualRole"] = "moderator"
			request(handlers.ChangeUserVisualRole(), http.MethodPost, `{"userID":2}`, params, test.visual)
			params["role"] = "visitor"
			receipt := request(handlers.ChangeUserRole(), http.MethodPost, `{"userID":2}`, params, test.role)
			if test.role {
				var updated entity.User
				if err := json.Unmarshal(receipt, &updated); err != nil {
					t.Fatal(err)
				}
				if updated.ID != 2 || updated.Role != enum.RoleVisitor || !updated.Permissions.Moderate || !updated.Permissions.Block {
					t.Fatalf("role receipt omitted confirmed target capabilities: %s", receipt)
				}
			}
		})
	}
}

func TestUserTargetCommandsRejectBypassingHandlers(t *testing.T) {
	f := newPostWorkflow(t)
	f.user.Role = enum.RoleVisitor
	for _, command := range []any{
		&cmd.BlockUser{UserID: 2},
		&cmd.UnblockUser{UserID: 2},
		&cmd.WarnUser{UserID: 2, Reason: "Bypass"},
		&cmd.MuteUser{UserID: 2, Reason: "Bypass"},
		&cmd.DeleteWarning{UserID: 2, WarningID: 1},
		&cmd.DeleteMute{UserID: 2, MuteID: 1},
		&cmd.ExpireWarning{UserID: 2, WarningID: 1},
		&cmd.ExpireMute{UserID: 2, MuteID: 1},
		&cmd.ChangeUserRole{UserID: 2, Role: enum.RoleAdministrator},
		&cmd.ChangeUserVisualRole{UserID: 2, VisualRole: enum.VisualRoleAdministrator},
		&cmd.SaveProfileName{UserID: 2, Name: "Bypass"},
		&cmd.SaveProfileAvatar{UserID: 2, AvatarType: enum.AvatarTypeLetter},
		&cmd.RegenerateAPIKey{},
	} {
		t.Run(fmt.Sprintf("%T", command), func(t *testing.T) {
			err := bus.Dispatch(f.ctx, command)
			failure, ok := errors.Cause(err).(*validate.Result)
			if !ok || failure.Authorized {
				t.Fatalf("expected authorization failure, got %v", err)
			}
		})
	}
	if workflowCount(t, "SELECT count(*) FROM users WHERE id = 2 AND name = 'Arya Stark' AND role = 1 AND status = 1") != 1 {
		t.Fatal("rejected commands changed the target")
	}
	if workflowCount(t, "SELECT count(*) FROM user_warnings") != 0 || workflowCount(t, "SELECT count(*) FROM user_mutes") != 0 {
		t.Fatal("rejected commands created moderation records")
	}
}

func TestUserTargetSelfAndRefreshedStanding(t *testing.T) {
	f := newPostWorkflow(t)
	for _, role := range []enum.Role{enum.RoleVisitor, enum.RoleHelper, enum.RoleModerator, enum.RoleCollaborator, enum.RoleAdministrator} {
		t.Run(role.String(), func(t *testing.T) {
			f.user.Role = role
			f.user.Muted = true

			for _, handler := range []web.HandlerFunc{handlers.BlockUser(), handlers.UnblockUser(), handlers.ExpireWarning(), handlers.ExpireMute()} {
				response, err := f.requestWithParams(handler, http.MethodPost, "http://localhost:3000/api/users/1", "",
					web.StringMap{"userID": "1", "warningID": "1", "muteID": "1"})
				if err != nil || response.Code != http.StatusForbidden {
					t.Fatalf("self moderation returned %d, %v", response.Code, err)
				}
			}

			response, err := f.requestWithParams(handlers.UpdateUserName(), http.MethodPost,
				"http://localhost:3000/api/user/name", `{"name":"Own profile"}`, nil)
			if err != nil || response.Code != http.StatusOK {
				t.Fatalf("muted self profile edit returned %d, %v", response.Code, err)
			}

			response, err = f.requestWithParams(api.GetUserProfileStanding(), http.MethodGet,
				"http://localhost:3000/api/user/profile/1/standing", "", web.StringMap{"userID": "1"})
			if err != nil || response.Code != http.StatusOK {
				t.Fatalf("standing read returned %d, %v", response.Code, err)
			}
			var standing struct {
				SessionPermissions map[entity.Permission]bool `json:"sessionPermissions"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &standing); err != nil {
				t.Fatal(err)
			}
			if allowed, present := standing.SessionPermissions[entity.CreatePosts]; !present || allowed {
				t.Fatalf("muted standing omitted or granted post capability: %s", response.Body.String())
			}

			f.user.Status = enum.UserBlocked
			response, err = f.requestWithParams(handlers.UpdateUserName(), http.MethodPost,
				"http://localhost:3000/api/user/name", `{"name":"Blocked update"}`, nil)
			if err != nil || response.Code != http.StatusForbidden {
				t.Fatalf("blocked profile edit returned %d, %v", response.Code, err)
			}
			f.user.Status = enum.UserActive
		})
	}

	actor := &query.GetUserByID{UserID: f.user.ID}
	if err := bus.Dispatch(f.ctx, actor); err != nil {
		t.Fatal(err)
	}
	if actor.Result.Name != "Own profile" || actor.Result.Status != enum.UserActive {
		t.Fatalf("rejected self moderation or blocked edit changed actor: %+v", actor.Result)
	}
}

func TestUserModerationUsesRouteTarget(t *testing.T) {
	f := newPostWorkflow(t)
	for _, handler := range []web.HandlerFunc{handlers.WarnUser(), handlers.MuteUser()} {
		response, err := f.requestWithParams(handler, http.MethodPost,
			"http://localhost:3000/api/admin/users/1/warn",
			`{"userID":2,"reason":"Different authorized target","duration":"1h"}`,
			web.StringMap{"userID": "1"})
		if err != nil || response.Code != http.StatusForbidden {
			t.Fatalf("body replaced route authorization target: status=%d, err=%v", response.Code, err)
		}
	}

	if workflowCount(t, "SELECT count(*) FROM user_warnings") != 0 || workflowCount(t, "SELECT count(*) FROM user_mutes") != 0 {
		t.Fatal("forged moderation request changed standing")
	}
}
