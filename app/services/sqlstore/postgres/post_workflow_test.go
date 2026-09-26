package postgres_test

import (
	"context"
	"math/rand"
	"net/url"
	"os"
	"testing"
	"time"

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

func workflowCount(t testing.TB, sql string) int {
	t.Helper()
	var count int
	if err := dbx.Connection().QueryRow(sql).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestPostWorkflowVoteModelAndActivity(t *testing.T) {
	f := newPostWorkflow(t)
	post := &cmd.AddNewPost{Title: "Vote model", Description: "Vote model"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}
	model := map[int]int{}
	random := rand.New(rand.NewSource(927))
	for step := 0; step < 300; step++ {
		userID := random.Intn(3) + 1
		direction := random.Intn(3) - 1
		user := &entity.User{ID: userID}
		var change bus.Msg = &cmd.AddVote{Post: post.Result, User: user, VoteType: enum.VoteType(direction)}
		if direction == 0 {
			change = &cmd.RemoveVote{Post: post.Result, User: user}
		}
		if err := bus.Dispatch(f.ctx, change); err != nil {
			t.Fatalf("seed 927 step %d: %v", step, err)
		}
		model[userID] = direction
		up, down := 0, 0
		for _, vote := range model {
			if vote == 1 {
				up++
			}
			if vote == -1 {
				down++
			}
		}
		var actualUp, actualDown int
		if err := dbx.Connection().QueryRow("SELECT upvotes, downvotes FROM posts WHERE id = $1", post.Result.ID).Scan(&actualUp, &actualDown); err != nil {
			t.Fatal(err)
		}
		if up != actualUp || down != actualDown {
			t.Fatalf("seed 927 step %d: expected %d/%d, got %d/%d", step, up, down, actualUp, actualDown)
		}
	}
	vote := &cmd.AddVote{Post: post.Result, User: f.user, VoteType: enum.VoteTypeUp}
	if err := bus.Dispatch(f.ctx, vote); err != nil {
		t.Fatal(err)
	}
	var before, after time.Time
	if err := dbx.Connection().QueryRow("SELECT created_at FROM post_votes WHERE post_id = $1 AND user_id = 1", post.Result.ID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err := bus.Dispatch(f.ctx, vote); err != nil {
		t.Fatal(err)
	}
	if err := dbx.Connection().QueryRow("SELECT created_at FROM post_votes WHERE post_id = $1 AND user_id = 1", post.Result.ID).Scan(&after); err != nil || !after.Equal(before) {
		t.Fatalf("replay refreshed vote age: %v %v %v", before, after, err)
	}
	if _, err := dbx.Connection().Exec("UPDATE posts SET last_activity_at = '2000-01-01'"); err != nil {
		t.Fatal(err)
	}
	vote.VoteType = enum.VoteTypeDown
	if err := bus.Dispatch(f.ctx, vote); err != nil {
		t.Fatal(err)
	}
	if workflowCount(t, "SELECT COUNT(*) FROM posts WHERE last_activity_at > '2000-01-02'") != 1 {
		t.Fatal("changed vote did not update ranking activity")
	}
}

func BenchmarkPostWorkflowVote(b *testing.B) {
	f := newPostWorkflow(b)
	post := &cmd.AddNewPost{Title: "Benchmark vote", Description: "Benchmark"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		direction := enum.VoteTypeUp
		for pb.Next() {
			if err := bus.Dispatch(f.ctx, &cmd.AddVote{Post: post.Result, User: f.user, VoteType: direction}); err != nil {
				b.Error(err)
			}
			direction = -direction
		}
	})
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
