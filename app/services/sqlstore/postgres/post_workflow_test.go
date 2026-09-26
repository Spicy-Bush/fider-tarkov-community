package postgres_test

import (
	"context"
	"net/url"
	"os"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/assets"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	blobsql "github.com/Spicy-Bush/fider-tarkov-community/app/services/blob/sql"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/sqlstore/postgres"
)

type postWorkflow struct {
	ctx    context.Context
	engine *web.Engine
	tenant *entity.Tenant
	user   *entity.User
}

func newPostWorkflow(t testing.TB) postWorkflow {
	t.Helper()
	dbx.Seed()
	t.Cleanup(dbx.Seed)
	assets.FS = os.DirFS(env.Path("."))
	bus.Init(postgres.Service{}, blobsql.Service{})
	baseURL, _ := url.Parse("http://localhost:3000")
	ctx := context.WithValue(context.Background(), app.RequestCtxKey, web.Request{URL: baseURL})
	tenant := &query.GetTenantByDomain{Domain: "demo"}
	if err := bus.Dispatch(ctx, tenant); err != nil {
		t.Fatal(err)
	}
	ctx = context.WithValue(ctx, app.TenantCtxKey, tenant.Result)
	user := &query.GetUserByID{UserID: 1}
	if err := bus.Dispatch(ctx, user); err != nil {
		t.Fatal(err)
	}
	ctx = context.WithValue(ctx, app.UserCtxKey, user.Result)
	return postWorkflow{ctx: ctx, engine: web.New(), tenant: tenant.Result, user: user.Result}
}

func TestPostWorkflowDeletedOriginalAndImportedNumbers(t *testing.T) {
	f := newPostWorkflow(t)
	original := &cmd.AddNewPost{Title: "Original proposal", Description: "A post"}
	duplicate := &cmd.AddNewPost{Title: "Duplicate proposal", Description: "Another post"}
	if err := bus.Dispatch(f.ctx, original, duplicate); err != nil {
		t.Fatal(err)
	}
	if err := bus.Dispatch(f.ctx, &cmd.MarkPostAsDuplicate{Post: duplicate.Result, Original: original.Result, Text: "Already suggested"}); err != nil {
		t.Fatal(err)
	}
	if err := bus.Dispatch(f.ctx, &cmd.SetPostResponse{Post: original.Result, Status: enum.PostDeleted}); err != nil {
		t.Fatal(err)
	}
	post := &query.GetPostByNumber{Number: duplicate.Result.Number}
	if err := bus.Dispatch(f.ctx, post); err != nil || post.Result.Response == nil || post.Result.Response.Original != nil {
		t.Fatalf("duplicate exposed its deleted original: %v %+v", err, post.Result)
	}
	_, err := dbx.Connection().Exec(`INSERT INTO posts
		(number, title, slug, description, tenant_id, user_id, created_at, status)
		VALUES (100, 'Imported', 'imported', '', $1, $2, NOW(), 6)`, f.tenant.ID, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dbx.Connection().Exec("DELETE FROM posts WHERE number = 100"); err != nil {
		t.Fatal(err)
	}
	next := &cmd.AddNewPost{Title: "After removed import", Description: "A post"}
	if err := bus.Dispatch(f.ctx, next); err != nil || next.Result.Number != 101 {
		t.Fatalf("number allocator reused a deleted import: %v %+v", err, next.Result)
	}
}
