package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

func TestNotificationChannelPermissionsPreserveExistingPreferences(t *testing.T) {
	f := newPostWorkflow(t)
	key := enum.NotificationEventMention.UserSettingsKeyName

	for _, role := range []enum.Role{
		enum.RoleVisitor,
		enum.RoleHelper,
		enum.RoleModerator,
		enum.RoleCollaborator,
		enum.RoleAdministrator,
	} {
		t.Run(role.String(), func(t *testing.T) {
			f.user.Role = role
			if _, err := dbx.Connection().Exec("DELETE FROM user_settings WHERE user_id = $1", f.user.ID); err != nil {
				t.Fatal(err)
			}

			for _, next := range []string{"3", "7", "5"} {
				err := bus.Dispatch(f.ctx, &cmd.UpdateCurrentUserSettings{Settings: map[string]string{key: next}})
				if err != nil {
					t.Fatalf("preserving or disabling existing email channels %s: %v", next, err)
				}
			}

			err := bus.Dispatch(f.ctx, &cmd.UpdateCurrentUserSettings{Settings: map[string]string{key: "7"}})
			want := "5"
			if role == enum.RoleAdministrator {
				if err != nil {
					t.Fatal(err)
				}
				want = "7"
			} else {
				var rejection *validate.Result
				if !errors.As(err, &rejection) || rejection.Authorized || rejection.Err != nil {
					t.Fatalf("nonadministrator email opt-in was not denied by policy: %v", err)
				}
			}

			settings := &query.GetCurrentUserSettings{}
			if err := bus.Dispatch(f.ctx, settings); err != nil {
				t.Fatal(err)
			}
			if settings.Result[key] != want {
				t.Fatalf("stored channels %s, want %s", settings.Result[key], want)
			}
		})
	}

	for _, invalid := range []string{"-1", "8", "email", "3.0"} {
		t.Run(fmt.Sprintf("invalid_%s", invalid), func(t *testing.T) {
			err := bus.Dispatch(f.ctx, &cmd.UpdateCurrentUserSettings{Settings: map[string]string{
				key:                    invalid,
				"unrelated_preference": "must not persist",
			}})
			var rejection *validate.Result
			if !errors.As(err, &rejection) || !rejection.Authorized || rejection.Err != nil || len(rejection.Errors) == 0 {
				t.Fatalf("invalid channels did not produce a validation error: %v", err)
			}
			if workflowCount(t, "SELECT count(*) FROM user_settings WHERE user_id = $1 AND key = $2", f.user.ID, "unrelated_preference") != 0 {
				t.Fatal("rejected settings partially persisted")
			}
		})
	}

	anonymous := context.WithValue(f.ctx, app.UserCtxKey, (*entity.User)(nil))
	err := bus.Dispatch(anonymous, &cmd.UpdateCurrentUserSettings{Settings: map[string]string{key: "1"}})
	var rejection *validate.Result
	if !errors.As(err, &rejection) || rejection.Authorized || rejection.Err != nil {
		t.Fatalf("anonymous settings mutation was not denied by policy: %v", err)
	}
}
