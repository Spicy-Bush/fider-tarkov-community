package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/mock"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestEditPageOffersOnlyEligibleAuthors(t *testing.T) {
	bus.AddHandler(func(ctx context.Context, q *query.GetUserProfileStanding) error {
		return nil
	})
	bus.AddHandler(func(ctx context.Context, q *query.GetPageTopics) error {
		q.Result = []*entity.PageTopic{}
		return nil
	})
	bus.AddHandler(func(ctx context.Context, q *query.GetPageTags) error {
		q.Result = []*entity.PageTag{}
		return nil
	})
	bus.AddHandler(func(ctx context.Context, q *query.GetAllUsers) error {
		q.Result = []*entity.User{
			{ID: 1, Role: enum.RoleVisitor, Status: enum.UserActive},
			{ID: 2, Role: enum.RoleHelper, Status: enum.UserActive},
			{ID: 3, Role: enum.RoleModerator, Status: enum.UserActive},
			{ID: 4, Role: enum.RoleCollaborator, Status: enum.UserActive},
			{ID: 5, Role: enum.RoleAdministrator, Status: enum.UserActive},
			{ID: 6, Role: enum.RoleAdministrator, Status: enum.UserBlocked},
			{ID: 7, Role: enum.RoleAdministrator, Status: enum.UserDeleted},
		}
		return nil
	})

	server := mock.NewServer().
		OnTenant(&entity.Tenant{ID: 1, Status: enum.TenantActive}).
		AsUser(&entity.User{ID: 5, Role: enum.RoleAdministrator, Status: enum.UserActive}).
		AddHeader("Accept", web.PageDataContentType)
	status, response := server.Execute(handlers.EditPagePage())
	if status != http.StatusOK {
		t.Fatalf("status %d; want 200", status)
	}

	var data struct {
		Props struct {
			Users []*entity.User `json:"users"`
		} `json:"props"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Props.Users) != 2 || data.Props.Users[0].ID != 4 || data.Props.Users[1].ID != 5 {
		t.Fatalf("eligible authors %+v; want collaborator 4 and administrator 5", data.Props.Users)
	}
}
