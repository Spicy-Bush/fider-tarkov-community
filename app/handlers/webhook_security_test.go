package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/middlewares"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/webhook"
)

func TestWebhookTestRequiresAuthorizedJSONWrite(t *testing.T) {
	bus.Init(webhook.Service{})
	bus.AddHandler(func(ctx context.Context, q *query.GetWebhook) error {
		q.Result = &entity.Webhook{
			ID:         q.ID,
			Name:       "Test delivery",
			Type:       enum.WebhookNewPost,
			Status:     enum.WebhookEnabled,
			Url:        "https://provider.example/events",
			HttpMethod: http.MethodPost,
			Content:    "{}",
		}
		return nil
	})

	providerCalls := 0
	providerStatus := http.StatusOK
	bus.AddHandler(func(ctx context.Context, c *cmd.HTTPRequest) error {
		providerCalls++
		c.ResponseStatusCode = providerStatus
		return nil
	})

	handler := middlewares.Chain(
		middlewares.CSRF(),
		middlewares.RequirePermission(entity.ManageWebhooks),
	)(handlers.TestWebhook())

	request := func(t *testing.T, tenant *entity.Tenant, user *entity.User, contentType string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "http://localhost/api/admin/webhook/test/42", nil)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", contentType)
		response := httptest.NewRecorder()
		ctx, err := web.NewContext(web.New(), req, response, web.StringMap{"id": "42"})
		if err != nil {
			t.Fatal(err)
		}

		ctx.SetTenant(tenant)
		if user != nil {
			ctx.SetUser(user)
		}

		if err := handler(ctx); err != nil {
			t.Fatal(err)
		}
		return response
	}

	for _, role := range []enum.Role{
		enum.RoleVisitor,
		enum.RoleHelper,
		enum.RoleModerator,
		enum.RoleCollaborator,
		enum.RoleAdministrator,
	} {
		for _, granted := range []bool{false, true} {
			tenant := &entity.Tenant{ID: 1}
			if granted {
				tenant.RolePermissions = entity.RolePermissions{role: {entity.ManageWebhooks: true}}
			}
			user := &entity.User{ID: 1, Role: role, Status: enum.UserActive, Tenant: tenant}

			before := providerCalls
			response := request(t, tenant, user, "application/json")
			allowed := granted || role == enum.RoleCollaborator || role == enum.RoleAdministrator
			wantStatus, wantCalls := http.StatusForbidden, before
			if allowed {
				wantStatus, wantCalls = http.StatusOK, before+1
			}

			if response.Code != wantStatus || providerCalls != wantCalls {
				t.Fatalf("role=%s granted=%t: HTTP %d calls=%d, expected HTTP %d calls=%d",
					role, granted, response.Code, providerCalls, wantStatus, wantCalls)
			}
		}
	}

	tenant := &entity.Tenant{ID: 1}
	admin := &entity.User{ID: 1, Role: enum.RoleAdministrator, Status: enum.UserActive, Tenant: tenant}
	blocked := *admin
	blocked.Status = enum.UserBlocked

	for _, test := range []struct {
		name        string
		user        *entity.User
		contentType string
		status      int
	}{
		{"anonymous", nil, "application/json", http.StatusUnauthorized},
		{"blocked", &blocked, "application/json", http.StatusForbidden},
		{"no content type", admin, "", http.StatusForbidden},
		{"simple request", admin, "text/plain", http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := providerCalls
			response := request(t, tenant, test.user, test.contentType)
			if response.Code != test.status || providerCalls != before {
				t.Fatalf("HTTP %d calls=%d, expected HTTP %d and no delivery", response.Code, providerCalls-before, test.status)
			}
		})
	}

	for _, status := range []int{http.StatusServiceUnavailable, http.StatusOK} {
		providerStatus = status
		response := request(t, tenant, admin, "application/json")
		var result dto.WebhookTriggerResult
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusOK || result.Success != (status == http.StatusOK) || result.Webhook.Status != enum.WebhookEnabled {
			t.Fatalf("provider status=%d: HTTP %d result=%+v", status, response.Code, result)
		}
	}
}
