package postgres_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	api "github.com/Spicy-Bush/fider-tarkov-community/app/handlers/apiv1"
	"github.com/Spicy-Bush/fider-tarkov-community/app/middlewares"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/jwt"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestUserStorageMuteSnapshot(t *testing.T) {
	f := newPostWorkflow(t)
	key := &cmd.RegenerateAPIKey{}
	if err := bus.Dispatch(f.ctx, key); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name     string
		tenantID int
		expires  any
		muted    bool
	}{
		{"permanent", 1, nil, true},
		{"active", 1, time.Now().Add(time.Hour), true},
		{"expired", 1, time.Now().Add(-time.Hour), false},
		{"other tenant", 2, nil, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := dbx.Connection().Exec("DELETE FROM user_mutes"); err != nil {
				t.Fatal(err)
			}

			_, err := dbx.Connection().Exec(`INSERT INTO user_mutes
                (user_id, tenant_id, reason, expires_at, created_by)
                VALUES ($1, $2, 'test', $3, $1)`, f.user.ID, test.tenantID, test.expires)
			if err != nil {
				t.Fatal(err)
			}

			byID := &query.GetUserByID{UserID: f.user.ID}
			byKey := &query.GetUserByAPIKey{APIKey: key.Result}
			if err := bus.Dispatch(f.ctx, byID, byKey); err != nil {
				t.Fatal(err)
			}

			if byID.Result.IsMuted() != test.muted || byKey.Result.IsMuted() != test.muted {
				t.Fatalf("wrong mute snapshot: cookie=%v API key=%v", byID.Result.IsMuted(), byKey.Result.IsMuted())
			}
		})
	}
}

func TestAuthenticatedVotePolicy(t *testing.T) {
	f := newPostWorkflow(t)
	post := &cmd.AddNewPost{Title: "Authenticated vote policy", Description: "Permission matrix"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}

	token, err := jwt.Encode(jwt.FiderClaims{UserID: 2, UserName: "Arya Stark"})
	if err != nil {
		t.Fatal(err)
	}

	for _, role := range []enum.Role{enum.RoleVisitor, enum.RoleHelper, enum.RoleModerator, enum.RoleCollaborator, enum.RoleAdministrator} {
		if _, err := dbx.Connection().Exec("UPDATE users SET role = $1 WHERE id = 2", role); err != nil {
			t.Fatal(err)
		}

		for _, state := range []string{"open", "locked", "closed", "hidden", "muted", "deleted", "blocked", "anonymous", "other tenant"} {
			t.Run(role.String()+"/"+state, func(t *testing.T) {
				if _, err := dbx.Connection().Exec("DELETE FROM post_votes; DELETE FROM post_vote_revisions; DELETE FROM user_mutes"); err != nil {
					t.Fatal(err)
				}

				userStatus := enum.UserActive
				if state == "blocked" {
					userStatus = enum.UserBlocked
				}
				if _, err := dbx.Connection().Exec("UPDATE users SET status = $1 WHERE id = 2", userStatus); err != nil {
					t.Fatal(err)
				}

				status := enum.PostOpen
				if state == "closed" {
					status = enum.PostCompleted
				} else if state == "deleted" {
					status = enum.PostDeleted
				}

				_, err := dbx.Connection().Exec(`UPDATE posts SET status = $1,
                    locked_settings = jsonb_build_object('locked', $2::boolean), moderation_pending = $3
                    WHERE id = $4`, status, state == "locked", state == "hidden", post.Result.ID)
				if err != nil {
					t.Fatal(err)
				}

				if state == "muted" {
					_, err := dbx.Connection().Exec(`INSERT INTO user_mutes
                        (user_id, tenant_id, reason, created_by) VALUES (2, 1, 'test', 1)`)
					if err != nil {
						t.Fatal(err)
					}
				}

				want := http.StatusOK
				if state == "muted" || state == "closed" || (state == "locked" && role != enum.RoleAdministrator && role != enum.RoleCollaborator) {
					want = http.StatusForbidden
				}
				if state == "deleted" || (state == "hidden" && (role == enum.RoleVisitor || role == enum.RoleHelper)) {
					want = http.StatusNotFound
				}
				if state == "blocked" || state == "anonymous" || state == "other tenant" {
					want = http.StatusUnauthorized
				}

				request := httptest.NewRequest(http.MethodPost, "/api/posts/1/up", strings.NewReader(`{"revision":0}`))
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("Accept", "application/json")
				if state != "anonymous" {
					credential := token
					if state == "other tenant" {
						credential, err = jwt.Encode(jwt.FiderClaims{UserID: 5, UserName: "Tony Stark"})
						if err != nil {
							t.Fatal(err)
						}
					}

					request.AddCookie(&http.Cookie{Name: web.CookieAuthName, Value: credential})
				}

				response := httptest.NewRecorder()
				ctx, err := web.NewContext(f.engine, request, response, web.StringMap{"number": "1"})
				if err != nil {
					t.Fatal(err)
				}
				ctx.SetTenant(f.tenant)

				err = middlewares.User()(api.AddVote())(ctx)
				if err != nil || response.Code != want {
					t.Fatalf("authenticated vote: %v %d %s; want %d", err, response.Code, response.Body, want)
				}

				votes := workflowCount(t, "SELECT COUNT(*) FROM post_votes")
				revisions := workflowCount(t, "SELECT COUNT(*) FROM post_vote_revisions")
				if (want == http.StatusOK && (votes != 1 || revisions != 1)) || (want != http.StatusOK && (votes != 0 || revisions != 0)) {
					t.Fatalf("unexpected stored effects: votes=%d revisions=%d", votes, revisions)
				}
			})
		}
	}
}

func TestUserStorageMuteLookupFailureAndRecovery(t *testing.T) {
	f := newPostWorkflow(t)
	key := &cmd.RegenerateAPIKey{}
	if err := bus.Dispatch(f.ctx, key); err != nil {
		t.Fatal(err)
	}

	_, err := dbx.Connection().Exec(`INSERT INTO user_mutes
        (user_id, tenant_id, reason, created_by) VALUES ($1, $2, 'active', $1)`, f.user.ID, f.tenant.ID)
	if err != nil {
		t.Fatal(err)
	}

	for _, credential := range []string{"cookie", "API key"} {
		t.Run(credential, func(t *testing.T) {
			authenticate := func(parent context.Context) (bool, error) {
				request := httptest.NewRequest(http.MethodGet, "/api/posts", nil).WithContext(parent)
				if credential == "cookie" {
					token, err := jwt.Encode(jwt.FiderClaims{UserID: f.user.ID, UserName: f.user.Name})
					if err != nil {
						t.Fatal(err)
					}

					request.AddCookie(&http.Cookie{Name: web.CookieAuthName, Value: token})
				} else {
					request.Header.Set("Authorization", "Bearer "+key.Result)
				}

				ctx, err := web.NewContext(f.engine, request, httptest.NewRecorder(), nil)
				if err != nil {
					t.Fatal(err)
				}
				ctx.SetTenant(f.tenant)

				called := false
				err = middlewares.User()(func(c *web.Context) error {
					called = true
					if !c.IsAuthenticated() || !c.User().IsMuted() {
						t.Fatal("recovered lookup lost the authenticated user's mute")
					}

					return nil
				})(ctx)

				return called, err
			}

			transaction, err := dbx.BeginTx(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer transaction.Rollback()

			if _, err := transaction.Execute("ALTER TABLE user_mutes RENAME TO unavailable_user_mutes"); err != nil {
				t.Fatal(err)
			}

			ctx := context.WithValue(context.Background(), app.TransactionCtxKey, transaction)
			called, err := authenticate(ctx)
			if err == nil || called {
				t.Fatal("failed mute lookup reached the authenticated handler")
			}

			if strings.Contains(err.Error(), key.Result) {
				t.Fatal("authentication failure disclosed the API key")
			}

			if err := transaction.Rollback(); err != nil {
				t.Fatal(err)
			}

			if called, err := authenticate(context.Background()); err != nil || !called {
				t.Fatalf("user lookup did not recover after database repair: %v", err)
			}
		})
	}
}
