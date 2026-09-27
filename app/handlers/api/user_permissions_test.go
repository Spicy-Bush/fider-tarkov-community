package api_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/api"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/mock"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestUserProfileReadsUseTargetPolicy(t *testing.T) {
	for _, endpoint := range []struct {
		name    string
		handler web.HandlerFunc
	}{
		{"stats", api.GetUserProfileStats()},
		{"standing", api.GetUserProfileStanding()},
		{"search", api.SearchUserContent()},
	} {
		for _, state := range []string{"staff", "visitor", "deleted target", "self"} {
			t.Run(endpoint.name+"/"+state, func(t *testing.T) {
				viewer := &entity.User{ID: 1, Role: enum.RoleModerator, Status: enum.UserActive, Tenant: mock.DemoTenant}
				target := &entity.User{ID: 2, Role: enum.RoleVisitor, Status: enum.UserActive, Tenant: mock.DemoTenant}
				if state == "visitor" {
					viewer.Role = enum.RoleVisitor
				} else if state == "deleted target" {
					target.Status = enum.UserDeleted
				} else if state == "self" {
					viewer.ID = target.ID
					viewer.Role = enum.RoleVisitor
				}

				targetReads, contentReads := 0, 0
				bus.AddHandler(func(ctx context.Context, q *query.GetUserByID) error {
					targetReads++
					q.Result = target
					return nil
				})
				bus.AddHandler(func(ctx context.Context, q *query.GetUserProfileStats) error {
					contentReads++
					return nil
				})
				bus.AddHandler(func(ctx context.Context, q *query.GetUserProfileStanding) error {
					contentReads++
					return nil
				})
				bus.AddHandler(func(ctx context.Context, q *query.SearchUserContent) error {
					contentReads++
					return nil
				})

				status, _ := mock.NewServer().OnTenant(mock.DemoTenant).AsUser(viewer).
					AddParam("userID", target.ID).AddHeader("Accept", "application/json").Execute(endpoint.handler)
				allowed := state == "staff" || state == "self"
				if allowed && (status != http.StatusOK || contentReads != 1) {
					t.Fatalf("allowed profile read: status=%d, content reads=%d", status, contentReads)
				}
				if !allowed && (status != http.StatusNotFound || contentReads != 0) {
					t.Fatalf("denied profile read: status=%d, content reads=%d", status, contentReads)
				}
				if state == "self" && targetReads != 0 {
					t.Fatalf("self standing redundantly loaded authenticated viewer %d times", targetReads)
				}
			})
		}
	}
}
