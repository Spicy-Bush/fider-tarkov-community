package handlers_test

import (
    "context"
    "fmt"
    "net/http"
    "os"
    "testing"
    "time"

    "github.com/Spicy-Bush/fider-tarkov-community/app"
    "github.com/Spicy-Bush/fider-tarkov-community/app/assets"
    "github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
    "github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
    "github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
    "github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
    "github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
    . "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/assert"
    "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
    "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
    "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/mock"
    "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func setupAuthenticationTest(t *testing.T) {
    t.Helper()
    RegisterT(t)
    previousAssets := assets.FS
    assets.FS = os.DirFS(env.Path())
    t.Cleanup(func() {
        assets.FS = previousAssets
    })
}

func TestSignInCannotRedeemClientSuppliedKey(t *testing.T) {
    for _, emailAuthAllowed := range []bool{true, false} {
        t.Run(fmt.Sprintf("email auth %t", emailAuthAllowed), func(t *testing.T) {
            setupAuthenticationTest(t)
            server := mock.NewServer()
            tenant := *mock.DemoTenant
            tenant.IsEmailAuthAllowed = emailAuthAllowed
            user := *mock.JonSnow
            user.Tenant = &tenant

            bus.AddHandler(func(ctx context.Context, q *query.GetUserByEmail) error {
                if q.Email != user.Email {
                    return app.ErrNotFound
                }
                q.Result = &user
                return nil
            })

            var saved *cmd.SaveVerificationKey
            bus.AddHandler(func(ctx context.Context, c *cmd.SaveVerificationKey) error {
                saved = c
                return nil
            })

            body := fmt.Sprintf(`{"email":%q,"verificationkey":"chosen-token"}`, user.Email)
            code, response := server.OnTenant(&tenant).ExecutePost(handlers.SignInByEmail(), body)
            if code != http.StatusOK || saved == nil {
                t.Fatalf("sign-in request did not save a verification: status %d", code)
            }
            if response.Header().Get("Set-Cookie") != "" {
                t.Fatal("requesting a sign-in email issued a cookie")
            }
            if saved.Key == "chosen-token" || len(saved.Key) != 64 {
                t.Error("sign-in saved a client-supplied verification key")
            }

            bus.AddHandler(func(ctx context.Context, q *query.GetVerificationByKey) error {
                if q.Key != saved.Key || q.Kind != saved.Request.GetKind() {
                    return app.ErrNotFound
                }
                q.Result = &entity.EmailVerification{
                    Key: saved.Key,
                    Kind: saved.Request.GetKind(),
                    Email: saved.Request.GetEmail(),
                    ExpiresAt: time.Now().Add(saved.Duration),
                }
                return nil
            })
            bus.AddHandler(func(ctx context.Context, c *cmd.SetKeyAsVerified) error {
                return nil
            })

            code, response = mock.NewServer().OnTenant(&tenant).
                AddHeader("Accept", web.JSONContentType).
                WithURL("http://demo.test.fider.io/signin/verify?k=chosen-token").
                Execute(handlers.VerifySignInKey(enum.EmailVerificationKindSignIn))
            if code != http.StatusNotFound || response.Header().Get("Set-Cookie") != "" {
                t.Fatal("client-supplied key authenticated the account")
            }

            code, response = mock.NewServer().OnTenant(&tenant).
                WithURL("http://demo.test.fider.io/signin/verify?k=" + saved.Key).
                Execute(handlers.VerifySignInKey(enum.EmailVerificationKindSignIn))
            if code != http.StatusTemporaryRedirect {
                t.Fatalf("server-generated key did not sign in: status %d", code)
            }
            ExpectFiderAuthCookie(response, &user)
        })
    }
}

func TestSignupCannotUseClientSuppliedIdentity(t *testing.T) {
    setupAuthenticationTest(t)
    bus.AddHandler(func(ctx context.Context, q *query.IsSubdomainAvailable) error {
        q.Result = true
        return nil
    })

    var created *cmd.CreateTenant
    bus.AddHandler(func(ctx context.Context, c *cmd.CreateTenant) error {
        created = c
        c.Result = &entity.Tenant{
            ID: 1,
            Name: c.Name,
            Subdomain: c.Subdomain,
            Status: c.Status,
        }
        return nil
    })

    registered := false
    bus.AddHandler(func(ctx context.Context, c *cmd.RegisterUser) error {
        registered = true
        return nil
    })

    var saved *cmd.SaveVerificationKey
    bus.AddHandler(func(ctx context.Context, c *cmd.SaveVerificationKey) error {
        saved = c
        return nil
    })

    body := `{
        "name":"Member",
        "email":"member@example.com",
        "tenantName":"Community",
        "subdomain":"community",
        "legalAgreement":true,
        "token":"unverified-token",
        "VerificationKey":"chosen-token",
        "UserClaims":{"oauth/id":"forged","oauth/name":"Admin","oauth/email":"admin@example.com","oauth/provider":"facebook"}
    }`
    code, response := mock.NewServer().ExecutePost(handlers.CreateTenant(), body)
    if code != http.StatusOK || created == nil {
        t.Fatalf("email signup did not complete: status %d", code)
    }
    if created.Status != enum.TenantPending || registered || response.Header().Get("Set-Cookie") != "" {
        t.Fatal("client-supplied identity bypassed email verification")
    }
    if saved == nil || len(saved.Key) != 64 || saved.Request.GetEmail() != "member@example.com" {
        t.Fatal("email signup did not save a server-generated verification")
    }
}
