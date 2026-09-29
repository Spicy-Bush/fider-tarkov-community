package postgres_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestSignInVerificationBelongsToTenant(t *testing.T) {
	f := newPostWorkflow(t)
	foreign := &cmd.CreateTenant{Name: "Another site", Subdomain: "verification-owner", Status: enum.TenantActive}
	if err := bus.Dispatch(f.ctx, foreign); err != nil {
		t.Fatal(err)
	}

	foreignContext := context.WithValue(f.ctx, app.TenantCtxKey, foreign.Result)
	key := "foreign-tenant-signin"
	if err := bus.Dispatch(foreignContext, &cmd.SaveVerificationKey{
		Key:      key,
		Duration: time.Hour,
		Request:  &actions.SignInByEmail{Email: f.user.Email},
	}); err != nil {
		t.Fatal(err)
	}

	lookup := &query.GetVerificationByKey{Kind: enum.EmailVerificationKindSignIn, Key: key}
	if err := bus.Dispatch(f.ctx, lookup); errors.Cause(err) != app.ErrNotFound || lookup.Result != nil {
		t.Errorf("another tenant's verification was returned: result=%v, error=%v", lookup.Result, err)
	}

	for _, role := range []enum.Role{enum.RoleVisitor, enum.RoleHelper, enum.RoleModerator, enum.RoleCollaborator, enum.RoleAdministrator} {
		t.Run(role.String(), func(t *testing.T) {
			if _, err := dbx.Connection().Exec("UPDATE users SET role = $1 WHERE tenant_id = $2 AND id = $3", role, f.tenant.ID, 1); err != nil {
				t.Fatal(err)
			}

			f.user = nil
			response, err := f.requestWithParams(handlers.VerifySignInKey(enum.EmailVerificationKindSignIn), http.MethodGet,
				"http://localhost:3000/signin/verify?k="+key, "", nil)
			if err != nil || response.Code != http.StatusNotFound {
				t.Fatalf("foreign verification: error=%v, HTTP %d", err, response.Code)
			}
			for _, cookie := range response.Result().Cookies() {
				if cookie.Name == web.CookieAuthName {
					t.Fatal("foreign verification issued an authentication cookie")
				}
			}
		})
	}

	if err := bus.Dispatch(foreignContext, lookup); err != nil || lookup.Result == nil || lookup.Result.VerifiedAt != nil {
		t.Fatalf("rejection changed the owning tenant's verification: %v, %v", lookup.Result, err)
	}

	f.user = nil
	f.tenant = foreign.Result
	response, err := f.requestWithParams(handlers.VerifySignInKey(enum.EmailVerificationKindSignIn), http.MethodGet,
		"http://localhost:3000/signin/verify?k="+key, "", nil)
	if err != nil || response.Code != http.StatusOK {
		t.Fatalf("owning tenant could not complete the sign-in flow: error=%v, HTTP %d", err, response.Code)
	}
}

func TestPrivateTenantProfileCompletionRequiresInvitation(t *testing.T) {
	f := newPostWorkflow(t)
	f.user = nil
	f.tenant.IsPrivate = true
	if _, err := dbx.Connection().Exec("UPDATE tenants SET is_private = TRUE WHERE id = $1", f.tenant.ID); err != nil {
		t.Fatal(err)
	}

	if err := bus.Dispatch(f.ctx, &cmd.SaveVerificationKey{
		Key:      "uninvited-signin",
		Duration: time.Hour,
		Request:  &actions.SignInByEmail{Email: "uninvited@example.invalid"},
	}); err != nil {
		t.Fatal(err)
	}

	body := `{"kind":1,"key":"uninvited-signin","name":"New member"}`
	response, err := f.requestWithParams(handlers.CompleteSignInProfile(), http.MethodPost,
		"http://localhost:3000/api/signin/complete", body, nil)
	if err != nil || response.Code != http.StatusForbidden {
		t.Errorf("uninvited profile: error=%v, HTTP %d", err, response.Code)
	}
	if count := workflowCount(t, "SELECT COUNT(*) FROM users WHERE email = $1", "uninvited@example.invalid"); count != 0 {
		t.Fatal("an uninvited user joined the private tenant")
	}

	f.tenant.IsPrivate = false
	if _, err := dbx.Connection().Exec("UPDATE tenants SET is_private = FALSE WHERE id = $1", f.tenant.ID); err != nil {
		t.Fatal(err)
	}

	response, err = f.requestWithParams(handlers.CompleteSignInProfile(), http.MethodPost,
		"http://localhost:3000/api/signin/complete", body, nil)
	if err != nil || response.Code != http.StatusOK {
		t.Fatalf("the same verification could not proceed once signup was allowed: error=%v, HTTP %d", err, response.Code)
	}

	f.tenant.IsPrivate = true
	if _, err := dbx.Connection().Exec("UPDATE tenants SET is_private = TRUE WHERE id = $1", f.tenant.ID); err != nil {
		t.Fatal(err)
	}

	if err := bus.Dispatch(f.ctx, &cmd.SaveVerificationKey{
		Key:      "invited-signin",
		Duration: time.Hour,
		Request:  &actions.UserInvitation{Email: "invited@example.invalid"},
	}); err != nil {
		t.Fatal(err)
	}

	response, err = f.requestWithParams(handlers.CompleteSignInProfile(), http.MethodPost,
		"http://localhost:3000/api/signin/complete", `{"kind":4,"key":"invited-signin","name":"Invited member"}`, nil)
	if err != nil || response.Code != http.StatusOK {
		t.Fatalf("an invited user could not join the private tenant: error=%v, HTTP %d", err, response.Code)
	}

	for _, kind := range []enum.EmailVerificationKind{0, enum.EmailVerificationKindSignUp, enum.EmailVerificationKindChangeEmail, 99} {
		action := &actions.CompleteProfile{Kind: kind, Key: "valid-format", Name: "New member"}
		if result := action.Validate(f.ctx, nil); result.Ok {
			t.Errorf("profile completion accepted verification kind %d", kind)
		}
	}
}
