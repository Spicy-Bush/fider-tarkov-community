package middlewares_test

import (
	"net/http"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/middlewares"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/mock"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestFilterContextUsesSettingsCapability(t *testing.T) {
	for _, role := range []enum.Role{enum.RoleVisitor, enum.RoleHelper, enum.RoleModerator, enum.RoleCollaborator, enum.RoleAdministrator} {
		for _, locked := range []bool{false, true} {
			tenant := &entity.Tenant{ID: 1, ProfanityWords: "private word list"}
			if locked {
				tenant.Status = enum.TenantLocked
			}
			viewer := &entity.User{ID: 2, Role: role, Tenant: tenant, Status: enum.UserActive}
			visible := ""
			status, _ := mock.NewServer().OnTenant(tenant).AsUser(viewer).
				Use(middlewares.FilterContext()).Execute(func(c *web.Context) error {
					visible = c.Tenant().ProfanityWords
					return c.Ok(web.Map{})
				})

			allowed := !locked && (role == enum.RoleCollaborator || role == enum.RoleAdministrator)
			if status != http.StatusOK || (visible != "") != allowed {
				t.Errorf("role=%s locked=%v: status=%d, visible=%q", role, locked, status, visible)
			}
			if tenant.ProfanityWords != "private word list" {
				t.Fatal("redaction mutated the shared tenant source")
			}
		}
	}
}
