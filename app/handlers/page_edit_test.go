package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/middlewares"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/mock"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/sse"
	"github.com/stretchr/testify/require"
)

func TestPageEditHTTPPermissionsAndInput(t *testing.T) {
	var calls int
	bus.AddHandler(func(ctx context.Context, open *cmd.OpenPageEdit) error {
		calls++
		open.Result = &entity.PageEditSession{PageID: 12}
		return nil
	})

	for _, role := range []enum.Role{0, enum.RoleVisitor, enum.RoleHelper, enum.RoleModerator, enum.RoleCollaborator, enum.RoleAdministrator} {
		t.Run(role.String(), func(t *testing.T) {
			server := mock.NewServer().
				OnTenant(&entity.Tenant{ID: 1, Status: enum.TenantActive}).
				Use(middlewares.RequirePermission(entity.ManagePages))
			if role != 0 {
				server.AsUser(&entity.User{ID: 1, Role: role, Status: enum.UserActive})
			}

			status, _ := server.ExecutePost(handlers.OpenPageEdit(), `{"pageId":12}`)
			if role == enum.RoleCollaborator || role == enum.RoleAdministrator {
				require.Equal(t, http.StatusOK, status)
			} else if role == 0 {
				require.Equal(t, http.StatusUnauthorized, status)
			} else {
				require.Equal(t, http.StatusForbidden, status)
			}
		})
	}
	require.Equal(t, 2, calls)

	for _, body := range []string{`{"pageId":"wrong"}`, `{"submissionId":""}`, `{"pageId":-1}`} {
		status, _ := mock.NewServer().ExecutePost(handlers.OpenPageEdit(), body)
		require.Equal(t, http.StatusBadRequest, status)
	}
	require.Equal(t, 2, calls)
}

func TestPageEditHTTPDoesNotBroadcastFailedSave(t *testing.T) {
	client, ok := sse.PageEditors.Register(1, 12, 100, sse.PageEditor{ID: 1, Name: "Editor"})
	require.True(t, ok)
	defer sse.PageEditors.Unregister(client)

	unavailable := true
	acceptedUpdate := []byte("committed update")
	bus.AddHandler(func(ctx context.Context, sync *cmd.SyncPageEdit) error {
		if unavailable {
			return errors.New("database commit failed")
		}
		sync.Result = &entity.PageEditSync{Update: []byte{0, 0}, StateVector: []byte{0}}
		sync.AcceptedUpdate = acceptedUpdate
		return nil
	})
	server := func() *mock.Server {
		return mock.NewServer().
			OnTenant(&entity.Tenant{ID: 1, Status: enum.TenantActive}).
			AsUser(&entity.User{ID: 1, Role: enum.RoleAdministrator, Status: enum.UserActive}).
			AddParam("id", 12)
	}

	status, _ := server().ExecutePost(handlers.SyncPageEdit(), `{"update":"AAA="}`)
	require.Equal(t, http.StatusInternalServerError, status)
	select {
	case <-client.Send():
		t.Fatal("failed save was broadcast")
	default:
	}

	unavailable = false
	status, _ = server().ExecutePost(handlers.SyncPageEdit(), `{"update":"AAA="}`)
	require.Equal(t, http.StatusOK, status)
	select {
	case message := <-client.Send():
		var event sse.PageEvent
		require.NoError(t, json.Unmarshal(message, &event))
		require.Equal(t, "update", event.Type)
		require.Equal(t, acceptedUpdate, event.Update)
	default:
		t.Fatal("accepted save was not broadcast")
	}

	status, _ = server().ExecutePost(handlers.SyncPageEdit(), `{"update":"invalid-base64"}`)
	require.Equal(t, http.StatusBadRequest, status)
}

func TestPageEditCursorRejectsInvalidPositions(t *testing.T) {
	for _, body := range []string{
		`{"clientId":1,"field":"content","anchor":{"tname":"settings"},"head":{"tname":"settings"}}`,
		`{"clientId":1,"field":"content","anchor":null,"head":{"tname":"content"}}`,
		`{"clientId":9007199254740992,"field":"content","anchor":null,"head":null}`,
	} {
		status, _ := mock.NewServer().AddParam("id", 12).ExecutePost(handlers.PageEditCursor(), body)
		require.Equal(t, http.StatusBadRequest, status)
	}
}
