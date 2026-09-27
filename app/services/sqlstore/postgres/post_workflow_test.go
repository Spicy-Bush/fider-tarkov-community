package postgres_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/assets"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/apiv1"
	"github.com/Spicy-Bush/fider-tarkov-community/app/jobs"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/backup"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	blobsql "github.com/Spicy-Bush/fider-tarkov-community/app/services/blob/sql"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/email"
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

func (f postWorkflow) queuePostNotification(t testing.TB) {
	t.Helper()
	post := &cmd.AddNewPost{Title: "Queued proposal", Description: "A notification test"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}
	if err := bus.Dispatch(f.ctx, &cmd.SchedulePostNotification{Post: post.Result, BaseURL: "http://localhost:3000"}); err != nil {
		t.Fatal(err)
	}
}

func (f postWorkflow) request(handler web.HandlerFunc, method string, number int, body string) (*httptest.ResponseRecorder, error) {
	return f.requestWithParams(handler, method, "http://localhost:3000/api/v1/posts", body, web.StringMap{"number": fmt.Sprint(number)})
}

func (f postWorkflow) requestWithParams(handler web.HandlerFunc, method, path, body string, params web.StringMap) (*httptest.ResponseRecorder, error) {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RequestURI = req.URL.RequestURI()
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	c, err := web.NewContext(f.engine, req, recorder, params)
	if err != nil {
		return recorder, err
	}

	c.SetTenant(f.tenant)
	c.SetUser(f.user)
	err = handler(c)
	return recorder, err
}

func workflowCount(t testing.TB, sql string, args ...any) int {
	t.Helper()
	var count int
	if err := dbx.Connection().QueryRow(sql, args...).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestPostWorkflowVoteRevisions(t *testing.T) {
	f := newPostWorkflow(t)
	post := &cmd.AddNewPost{Title: "Vote revisions", Description: "A post"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}
	if err := bus.Dispatch(f.ctx, &cmd.AddVote{Post: post.Result, User: f.user, VoteType: enum.VoteTypeUp}); err != nil {
		t.Fatal(err)
	}
	write := func(direction int, revision int) map[string]any {
		t.Helper()
		handler, method := apiv1.RemoveVote(), http.MethodDelete
		if direction == 1 {
			handler, method = apiv1.AddVote(), http.MethodPost
		} else if direction == -1 {
			handler, method = apiv1.AddDownVote(), http.MethodPost
		}
		recorder, err := f.request(handler, method, 1, fmt.Sprintf(`{"revision":%d}`, revision))
		if err != nil || recorder.Code != http.StatusOK {
			t.Fatalf("vote: %v %d %s", err, recorder.Code, recorder.Body)
		}
		var state map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &state); err != nil {
			t.Fatal(err)
		}
		return state
	}
	removed := write(0, 1)
	if removed["applied"] != true || removed["revision"] != float64(2) || removed["upvotes"] != float64(0) {
		t.Fatalf("remove did not advance state: %v", removed)
	}
	stale := write(-1, 1)
	if stale["applied"] != false || stale["direction"] != float64(0) {
		t.Fatalf("stale vote overwritten: %v", stale)
	}
	noop := write(0, 2)
	if noop["revision"] != float64(3) {
		t.Fatalf("accepted no-op did not advance revision: %v", noop)
	}
	current := write(-1, 3)
	if current["downvotes"] != float64(1) || current["revision"] != float64(4) {
		t.Fatalf("downvote counts/revision incorrect: %v", current)
	}
	response, err := f.request(apiv1.GetPost(), http.MethodGet, 1, "")
	var hydrated entity.Post
	if err != nil || json.Unmarshal(response.Body.Bytes(), &hydrated) != nil || hydrated.VoteRevision != 4 || hydrated.VoteType != -1 {
		t.Fatalf("post hydration disagrees with committed vote: %v %s", err, response.Body)
	}
	if _, err := dbx.Connection().Exec("DELETE FROM post_votes; DELETE FROM post_subscribers; DELETE FROM posts WHERE number = 1"); err != nil {
		t.Fatalf("physical deletion failed with revision trigger: %v", err)
	}
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

func BenchmarkPostWorkflowVoteAPI(b *testing.B) {
	f := newPostWorkflow(b)
	var sequence atomic.Int64
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		title := fmt.Sprintf("Benchmark API vote %d", sequence.Add(1))
		post := &cmd.AddNewPost{Title: title, Description: "Benchmark"}
		if err := bus.Dispatch(f.ctx, post); err != nil {
			b.Error(err)
			return
		}
		recorder, err := f.request(apiv1.GetPost(), http.MethodGet, post.Result.Number, "")
		var initial entity.Post
		if err != nil || json.Unmarshal(recorder.Body.Bytes(), &initial) != nil {
			b.Errorf("post hydration: %v %s", err, recorder.Body)
			return
		}
		revision, direction := initial.VoteRevision, 1
		for pb.Next() {
			handler := apiv1.AddVote()
			if direction == -1 {
				handler = apiv1.AddDownVote()
			}
			body := fmt.Sprintf(`{"revision":%d}`, revision)
			recorder, err := f.request(handler, http.MethodPost, post.Result.Number, body)
			var after struct {
				Direction int  `json:"direction"`
				Applied   bool `json:"applied"`
				Revision  int64 `json:"revision"`
			}
			decodeErr := json.Unmarshal(recorder.Body.Bytes(), &after)
			if err != nil || decodeErr != nil || recorder.Code != http.StatusOK || !after.Applied || after.Direction != direction {
				b.Errorf("vote failed: %v %v %d %s", err, decodeErr, recorder.Code, recorder.Body)
				return
			}
			direction = -direction
			revision = after.Revision
		}
	})
}

func TestPostWorkflowConcurrentVotesAndStaleStatus(t *testing.T) {
	f := newPostWorkflow(t)
	post := &cmd.AddNewPost{Title: "Concurrent votes", Description: "A post"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}

	var voters sync.WaitGroup
	var applied atomic.Int64
	start := make(chan struct{})
	for i := 0; i < 16; i++ {
		voters.Add(1)
		go func() {
			defer voters.Done()
			<-start

			recorder, err := f.request(apiv1.AddDownVote(), http.MethodPost, post.Result.Number, `{"revision":0}`)
			var state cmd.PostVoteState
			if err != nil || recorder.Code != http.StatusOK || json.Unmarshal(recorder.Body.Bytes(), &state) != nil {
				t.Errorf("concurrent vote failed: %v %d %s", err, recorder.Code, recorder.Body)
				return
			}

			if state.Applied {
				applied.Add(1)
			}

			if state.Revision != 1 || state.Direction != -1 || state.Downvotes != 1 || state.Upvotes != 0 {
				t.Errorf("concurrent request returned inconsistent state: %+v", state)
			}
		}()
	}

	close(start)
	voters.Wait()
	if applied.Load() != 1 || workflowCount(t, "SELECT vote_type FROM post_votes") != -1 ||
		workflowCount(t, "SELECT revision FROM post_vote_revisions") != 1 {
		t.Fatal("concurrent requests applied the same revision more than once")
	}

	if err := bus.Dispatch(f.ctx, &cmd.SetPostResponse{Post: post.Result, Status: enum.PostDeleted}); err != nil {
		t.Fatal(err)
	}
	// Stale callers must not bypass the persisted deletion.
	post.Result.Status = enum.PostOpen
	if err := bus.Dispatch(f.ctx, &cmd.AddVote{Post: post.Result, User: f.user, VoteType: enum.VoteTypeUp}); err != nil {
		t.Fatal(err)
	}
	if workflowCount(t, "SELECT vote_type FROM post_votes") != -1 {
		t.Fatal("stale caller changed a deleted post's vote")
	}
}

func TestPostWorkflowArchiveVoteRevival(t *testing.T) {
	f := newPostWorkflow(t)
	post := &cmd.AddNewPost{Title: "Revive archived post", Description: "A post"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}
	if err := bus.Dispatch(f.ctx, &cmd.ArchivePost{Post: post.Result}); err != nil {
		t.Fatal(err)
	}
	_, err := dbx.Connection().Exec(`INSERT INTO users
		(name, email, created_at, tenant_id, role, status, avatar_type, avatar_bkey)
		SELECT 'Voter ' || n, 'voter' || n || '@example.com', NOW(), $1, 0, 1, 1, ''
		FROM generate_series(1, 10) n`, f.tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = dbx.Connection().Exec(`INSERT INTO post_votes (post_id, tenant_id, user_id, vote_type, created_at)
		SELECT $1, $2, id, 1, NOW() FROM users WHERE email LIKE 'voter%@example.com'`, post.Result.ID, f.tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	recorder, err := f.request(apiv1.AddVote(), http.MethodPost, post.Result.Number, `{"revision":0}`)
	if err != nil || recorder.Code != http.StatusOK {
		t.Fatalf("reviving vote failed: %v %d %s", err, recorder.Code, recorder.Body)
	}
	if workflowCount(t, "SELECT status FROM posts") != int(enum.PostOpen) ||
		workflowCount(t, "SELECT upvotes FROM posts") != 11 {
		t.Fatal("eleven new upvotes did not revive the archived post")
	}
}

func TestPostWorkflowVoteVisibilityAndPermissions(t *testing.T) {
	f := newPostWorkflow(t)
	post := &cmd.AddNewPost{Title: "Vote visibility", Description: "A post"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}
	visitor := f
	visitor.user = &entity.User{ID: 2, Role: enum.RoleVisitor}
	if _, err := dbx.Connection().Exec("UPDATE posts SET moderation_pending = TRUE"); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		handler := apiv1.GetPost()
		if method == http.MethodPost {
			handler = apiv1.AddVote()
		}
		recorder, err := visitor.request(handler, method, post.Result.Number, `{"revision":0}`)
		if err != nil || recorder.Code != http.StatusNotFound {
			t.Fatalf("hidden post visible through %s vote: %v %d", method, err, recorder.Code)
		}
	}
	if _, err := dbx.Connection().Exec(`UPDATE posts SET moderation_pending = FALSE, locked_settings = '{"locked":true}'`); err != nil {
		t.Fatal(err)
	}
	recorder, err := visitor.request(apiv1.AddVote(), http.MethodPost, post.Result.Number, `{"revision":0}`)
	if err != nil || recorder.Code != http.StatusForbidden {
		t.Fatalf("visitor voted on locked post: %v %d", err, recorder.Code)
	}
	if _, err := dbx.Connection().Exec("UPDATE posts SET locked_settings = NULL, status = 2"); err != nil {
		t.Fatal(err)
	}
	recorder, err = f.request(apiv1.AddVote(), http.MethodPost, post.Result.Number, `{"revision":0}`)
	if err != nil || recorder.Code != http.StatusForbidden || workflowCount(t, "SELECT COUNT(*) FROM post_votes") != 0 {
		t.Fatalf("closed post accepted vote: %v %d", err, recorder.Code)
	}
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

func TestPostWorkflowVoteRequiresRevision(t *testing.T) {
	f := newPostWorkflow(t)
	post := &cmd.AddNewPost{Title: "Required vote revision", Description: "A post"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}

	for _, role := range []enum.Role{enum.RoleVisitor, enum.RoleHelper, enum.RoleModerator, enum.RoleCollaborator, enum.RoleAdministrator} {
		t.Run(role.String(), func(t *testing.T) {
			actor := f
			actor.user = &entity.User{ID: 2, Role: role}
			actor.user.Muted = true

			for _, endpoint := range []struct {
				name    string
				method  string
				handler web.HandlerFunc
			}{
				{"up", http.MethodPost, apiv1.AddVote()},
				{"down", http.MethodPost, apiv1.AddDownVote()},
				{"remove", http.MethodDelete, apiv1.RemoveVote()},
			} {
				for _, body := range []string{"", "{}", "null", `{"revision":null}`, `{"revision":-1}`, `{"revision":"0"}`, `{"revision":0.5}`, `{"revision":`} {
					t.Run(endpoint.name+"/"+body, func(t *testing.T) {
						recorder, err := actor.request(endpoint.handler, endpoint.method, post.Result.Number, body)
						if err != nil || recorder.Code != http.StatusBadRequest {
							t.Fatalf("invalid revision accepted: %v %d %s", err, recorder.Code, recorder.Body)
						}
					})
				}

				recorder, err := actor.request(endpoint.handler, endpoint.method, post.Result.Number, `{"revision":0}`)
				if err != nil || recorder.Code != http.StatusForbidden {
					t.Fatalf("muted user accepted by %s: %v %d %s", endpoint.name, err, recorder.Code, recorder.Body)
				}
			}
		})
	}

	if workflowCount(t, "SELECT COUNT(*) FROM post_votes") != 0 ||
		workflowCount(t, "SELECT COUNT(*) FROM post_vote_revisions") != 0 ||
		workflowCount(t, "SELECT upvotes + downvotes FROM posts WHERE id = $1", post.Result.ID) != 0 {
		t.Fatal("rejected requests changed stored vote state")
	}
}

func TestPostWorkflowConcurrentNotificationDelivery(t *testing.T) {
	f := newPostWorkflow(t)
	f.queuePostNotification(t)
	prepare := func(context.Context, *entity.Post) ([]cmd.PostNotificationRecipient, error) {
		return []cmd.PostNotificationRecipient{{Channel: "email", ID: 2}, {Channel: "email", ID: 3}}, nil
	}
	if err := bus.Dispatch(f.ctx, &cmd.ProcessPostNotification{Prepare: prepare}); err != nil {
		t.Fatal(err)
	}
	entered := make(chan int, 2)
	release := make(chan struct{})
	finished := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			finished <- bus.Dispatch(f.ctx, &cmd.ProcessPostNotification{
				Prepare: prepare,
				Send: func(ctx context.Context, post *entity.Post, recipients []cmd.PostNotificationRecipient) error {
					entered <- recipients[0].ID
					<-release
					return nil
				},
			})
		}()
	}
	ids := make(map[int]bool)
	for i := 0; i < 2; i++ {
		select {
		case id := <-entered:
			ids[id] = true
		case <-time.After(5 * time.Second):
			close(release)
			t.Fatal("one recipient blocked another recipient's delivery")
		}
	}
	close(release)
	for i := 0; i < 2; i++ {
		if err := <-finished; err != nil {
			t.Fatal(err)
		}
	}
	if len(ids) != 2 || workflowCount(t, "SELECT COUNT(*) FROM post_notification_deliveries") != 0 {
		t.Fatal("concurrent delivery repeated a recipient or retained the completed envelope")
	}
}

func TestPostWorkflowNotificationAfterAuthorDeletion(t *testing.T) {
	f := newPostWorkflow(t)
	f.queuePostNotification(t)
	if err := bus.Dispatch(f.ctx, &cmd.DeleteCurrentUser{}); err != nil {
		t.Fatal(err)
	}
	prepared := false
	err := bus.Dispatch(f.ctx, &cmd.ProcessPostNotification{
		Prepare: func(ctx context.Context, post *entity.Post) ([]cmd.PostNotificationRecipient, error) {
			author := ctx.Value(app.UserCtxKey).(*entity.User)
			if author.Status != enum.UserDeleted || author.Name != "" || author.Email != "" {
				t.Fatal("delivery restored the deleted author's identity")
			}
			prepared = true
			return nil, nil
		},
	})
	if err != nil || !prepared || workflowCount(t, "SELECT COUNT(*) FROM post_notification_deliveries") != 0 {
		t.Fatalf("deleted author stranded delivery: %v", err)
	}
}

func BenchmarkPostNotificationQueue(b *testing.B) {
	for _, batchSize := range []int{1, 1000} {
		b.Run(fmt.Sprintf("batch-%d", batchSize), func(b *testing.B) {
			benchmarkPostNotificationQueue(b, batchSize)
		})
	}
}

func benchmarkPostNotificationQueue(b *testing.B, batchSize int) {
	f := newPostWorkflow(b)
	var delivered int
	process := func() *cmd.ProcessPostNotification {
		return &cmd.ProcessPostNotification{
			EmailBatchSize: batchSize,
			Prepare: func(context.Context, *entity.Post) ([]cmd.PostNotificationRecipient, error) {
				recipients := make([]cmd.PostNotificationRecipient, 100)
				for i := range recipients {
					recipients[i] = cmd.PostNotificationRecipient{Channel: "email", ID: i + 1}
				}
				return recipients, nil
			},
			Send: func(ctx context.Context, post *entity.Post, recipients []cmd.PostNotificationRecipient) error {
				delivered += len(recipients)
				return nil
			},
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		post := &cmd.AddNewPost{Title: fmt.Sprintf("Delivery benchmark %d", i), Description: "A representative notification"}
		if err := bus.Dispatch(f.ctx, post); err != nil {
			b.Fatal(err)
		}
		if err := bus.Dispatch(f.ctx, &cmd.SchedulePostNotification{Post: post.Result, BaseURL: "http://localhost:3000"}); err != nil {
			b.Fatal(err)
		}
		for {
			operation := process()
			if err := bus.Dispatch(f.ctx, operation); err != nil {
				b.Fatal(err)
			}
			if !operation.Found {
				break
			}
		}
	}
	b.StopTimer()
	if delivered != b.N*100 || workflowCount(b, "SELECT COUNT(*) FROM post_notification_deliveries") != 0 {
		b.Fatal("benchmark did not complete and reclaim every delivery")
	}
}

func TestPostWorkflowNotificationBatchFallback(t *testing.T) {
	f := newPostWorkflow(t)
	f.queuePostNotification(t)
	batches, healthy, recovered := 0, 0, 0
	fail := true
	process := func() error {
		return bus.Dispatch(f.ctx, &cmd.ProcessPostNotification{
			EmailBatchSize: 1000,
			Prepare: func(context.Context, *entity.Post) ([]cmd.PostNotificationRecipient, error) {
				return []cmd.PostNotificationRecipient{{Channel: "email", ID: 2}, {Channel: "email", ID: 3}}, nil
			},
			Send: func(ctx context.Context, post *entity.Post, recipients []cmd.PostNotificationRecipient) error {
				if len(recipients) > 1 {
					batches++
					return &email.RecipientRejected{Cause: fmt.Errorf("batch contains an invalid recipient")}
				}
				if recipients[0].ID == 2 {
					healthy++
					return nil
				}
				if fail {
					return fmt.Errorf("recipient temporarily unavailable")
				}
				recovered++
				return nil
			},
		})
	}
	if err := process(); err != nil {
		t.Fatal(err)
	}
	if err := process(); err == nil || batches != 1 {
		t.Fatal("expected one rejected batch")
	}
	if _, err := dbx.Connection().Exec("UPDATE post_notification_recipients SET available_at = NOW()"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		_ = process()
	}
	if batches != 1 || healthy != 1 || recovered != 0 || workflowCount(t, "SELECT COUNT(*) FROM post_notification_recipients") != 1 {
		t.Fatal("failed batch did not isolate recipient failures")
	}
	fail = false
	if _, err := dbx.Connection().Exec("UPDATE post_notification_recipients SET available_at = NOW()"); err != nil {
		t.Fatal(err)
	}
	if err := process(); err != nil || healthy != 1 || recovered != 1 || workflowCount(t, "SELECT COUNT(*) FROM post_notification_deliveries") != 0 {
		t.Fatalf("failed recipient did not recover independently: %v", err)
	}
}

func BenchmarkPostNotificationIdle(b *testing.B) {
	f := newPostWorkflow(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		operation := &cmd.ProcessPostNotification{}
		if err := bus.Dispatch(f.ctx, operation); err != nil || operation.Found {
			b.Fatalf("idle queue: found=%v error=%v", operation.Found, err)
		}
	}
}

func TestPostWorkflowTransientFailurePreservesEmailBatch(t *testing.T) {
	f := newPostWorkflow(t)
	f.queuePostNotification(t)
	var sizes []int
	operation := func() *cmd.ProcessPostNotification {
		return &cmd.ProcessPostNotification{
			EmailBatchSize: 1000,
			Prepare: func(context.Context, *entity.Post) ([]cmd.PostNotificationRecipient, error) {
				recipients := make([]cmd.PostNotificationRecipient, 1000)
				for i := range recipients {
					recipients[i] = cmd.PostNotificationRecipient{Channel: "email", ID: i + 1}
				}
				return recipients, nil
			},
			Send: func(_ context.Context, _ *entity.Post, recipients []cmd.PostNotificationRecipient) error {
				sizes = append(sizes, len(recipients))
				if len(sizes) == 1 {
					return fmt.Errorf("provider temporarily unavailable")
				}
				return nil
			},
		}
	}
	if err := bus.Dispatch(f.ctx, operation()); err != nil {
		t.Fatal(err)
	}
	if err := bus.Dispatch(f.ctx, operation()); err == nil {
		t.Fatal("expected the provider failure")
	}
	if _, err := dbx.Connection().Exec("UPDATE post_notification_recipients SET available_at = NOW()"); err != nil {
		t.Fatal(err)
	}
	if err := bus.Dispatch(f.ctx, operation()); err != nil {
		t.Fatal(err)
	}
	if len(sizes) != 2 || sizes[0] != 1000 || sizes[1] != 1000 {
		t.Fatalf("temporary provider failure fragmented a healthy batch: %v", sizes)
	}
	if workflowCount(t, "SELECT COUNT(*) FROM post_notification_deliveries") != 0 {
		t.Fatal("successful batch retained pending work")
	}
}

func TestPostWorkflowNotificationRecovery(t *testing.T) {
	f := newPostWorkflow(t)

	emailType := env.Config.Email.Type
	env.Config.Email.Type = "smtp"
	t.Cleanup(func() {
		env.Config.Email.Type = emailType
	})

	f.queuePostNotification(t)

	emailRecipients := []*entity.User{
		{ID: 2, Email: "accepted@example.com"},
		{ID: 3, Email: "recover@example.com"},
	}

	bus.AddHandler(func(ctx context.Context, q *query.GetActiveSubscribers) error {
		if q.Channel == enum.NotificationChannelWeb {
			q.Result = []*entity.User{{ID: 2}}
		} else if q.Channel == enum.NotificationChannelEmail {
			for _, user := range emailRecipients {
				if len(q.UserIDs) == 0 || slices.Contains(q.UserIDs, user.ID) {
					q.Result = append(q.Result, user)
				}
			}
		}

		return nil
	})

	bus.AddHandler(func(ctx context.Context, q *query.ListActiveWebhooksByType) error {
		q.Result = []*entity.Webhook{{ID: 1}, {ID: 2}}
		return nil
	})

	accepted, recovered, hooks := 0, 0, 0
	fail := true

	bus.AddHandler(func(ctx context.Context, c *cmd.SendMail) error {
		if len(c.To) != 1 {
			t.Fatalf("delivery bundles independent recipients: %d", len(c.To))
		}

		if c.To[0].Address == "accepted@example.com" {
			accepted++
		} else if fail {
			return fmt.Errorf("injected delivery failure")
		} else {
			recovered++
		}

		return nil
	})

	bus.AddHandler(func(context.Context, *cmd.DeliverWebhook) error {
		hooks++
		return nil
	})

	t.Cleanup(func() {
		bus.Init(postgres.Service{}, blobsql.Service{})
	})

	dbx.Connection().SetMaxOpenConns(1)
	t.Cleanup(func() {
		dbx.Connection().SetMaxOpenConns(env.Config.Database.MaxOpenConns)
	})

	job := &jobs.PostNotificationDeliveryJob{}
	job.Run()

	if accepted != 1 || recovered != 0 || hooks != 2 {
		t.Fatalf("partial delivery blocked healthy work: accepted=%d recovered=%d hooks=%d", accepted, recovered, hooks)
	}

	if workflowCount(t, "SELECT COUNT(*) FROM notifications") != 1 {
		t.Fatal("web notification was not delivered exactly once")
	}

	for attempt := 0; attempt < 3; attempt++ {
		if _, err := dbx.Connection().Exec("UPDATE post_notification_recipients SET available_at = NOW()"); err != nil {
			t.Fatal(err)
		}

		job.Run()
	}

	if accepted != 1 || hooks != 2 {
		t.Fatalf("retry repeated completed work: accepted=%d hooks=%d", accepted, hooks)
	}

	if workflowCount(t, "SELECT COUNT(*) FROM post_notification_recipients") != 1 {
		t.Fatal("retry discarded the failed recipient")
	}

	fail = false
	if _, err := dbx.Connection().Exec("UPDATE post_notification_recipients SET available_at = NOW()"); err != nil {
		t.Fatal(err)
	}

	job.Run()

	if accepted != 1 || recovered != 1 || hooks != 2 {
		t.Fatalf("recovery duplicated or lost delivery: %d %d %d", accepted, recovered, hooks)
	}

	pendingDeliveries := workflowCount(t, "SELECT COUNT(*) FROM post_notification_deliveries")
	pendingRecipients := workflowCount(t, "SELECT COUNT(*) FROM post_notification_recipients")
	if pendingDeliveries != 0 || pendingRecipients != 0 {
		t.Fatal("completed delivery retained queue data")
	}
}

func submissionBody(t testing.TB, id string, images bool) string {
	t.Helper()

	input := map[string]any{
		"title":        "A complete suggestion " + id,
		"description":  strings.Repeat("A useful description for the community. ", 6),
		"submissionId": id,
	}

	if images {
		var imageData bytes.Buffer
		if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 20, 20))); err != nil {
			t.Fatal(err)
		}

		input["attachments"] = []*dto.ImageUpload{
			{
				Upload: &dto.ImageUploadData{
					FileName:    "example.png",
					ContentType: "image/png",
					Content:     imageData.Bytes(),
				},
			},
		}
	}

	body, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}

	return string(body)
}

func TestPostWorkflowConcurrentReplayAndDeletedReceipt(t *testing.T) {
	f := newPostWorkflow(t)
	body := submissionBody(t, "same-operation", true)

	var writers sync.WaitGroup
	responses := make(chan string, 16)

	for i := 0; i < 16; i++ {
		writers.Add(1)
		go func() {
			defer writers.Done()

			recorder, err := f.request(apiv1.CreatePost(), http.MethodPost, 0, body)
			if err != nil || recorder.Code != http.StatusOK {
				t.Errorf("create: status=%d error=%v body=%s", recorder.Code, err, recorder.Body)
			}

			responses <- recorder.Body.String()
		}()
	}

	writers.Wait()
	close(responses)

	var original string
	for response := range responses {
		if original == "" {
			original = response
		} else if response != original {
			t.Errorf("replay changed receipt: %s != %s", response, original)
		}
	}

	effectCounts := []string{
		"SELECT COUNT(*) FROM posts",
		"SELECT COUNT(*) FROM post_votes",
		"SELECT COUNT(*) FROM post_subscribers",
		"SELECT COUNT(*) FROM attachments",
		"SELECT COUNT(*) FROM post_notification_deliveries",
	}

	for _, query := range effectCounts {
		if count := workflowCount(t, query); count != 1 {
			t.Errorf("%s: want exactly one effect, got %d", query, count)
		}
	}

	_, err := dbx.Connection().Exec(
		"UPDATE posts SET status = $1, title = $2",
		enum.PostDeleted, "edited and deleted",
	)
	if err != nil {
		t.Fatal(err)
	}

	f.user.Muted = true
	replay, err := f.request(apiv1.CreatePost(), http.MethodPost, 0, body)
	if err != nil {
		t.Fatal(err)
	}

	if replay.Code != http.StatusOK || replay.Body.String() != original {
		t.Fatalf("deleted receipt was not recovered: status=%d body=%s", replay.Code, replay.Body)
	}

	deleted, err := f.request(apiv1.GetPost(), http.MethodGet, 1, "")
	if err != nil || deleted.Code != http.StatusNotFound {
		t.Fatalf("deleted post visible: %v %d %s", err, deleted.Code, deleted.Body)
	}

	changedBody := strings.Replace(body, "A complete suggestion", "A different suggestion", 1)
	conflict, err := f.request(apiv1.CreatePost(), http.MethodPost, 0, changedBody)
	if err != nil || conflict.Code != http.StatusConflict {
		t.Fatalf("changed payload reused identity: %v %d %s", err, conflict.Code, conflict.Body)
	}

	newBody := submissionBody(t, "muted-new-operation", false)
	denied, err := f.request(apiv1.CreatePost(), http.MethodPost, 0, newBody)
	if err != nil || denied.Code != http.StatusBadRequest {
		t.Fatalf("muted user created a new post: %v %d %s", err, denied.Code, denied.Body)
	}

	for _, query := range effectCounts {
		if count := workflowCount(t, query); count != 1 {
			t.Errorf("%s: receipt recovery or rejected submission changed effects to %d", query, count)
		}
	}
}

func TestPostWorkflowRollbackAndRecovery(t *testing.T) {
	f := newPostWorkflow(t)
	body := submissionBody(t, "recover-after-failure", true)

	bus.AddHandler(func(context.Context, *cmd.ScheduleModeration) error {
		return fmt.Errorf("injected scheduling failure")
	})

	failed, err := f.request(apiv1.CreatePost(), http.MethodPost, 0, body)
	if err == nil || failed.Code == http.StatusOK {
		t.Fatalf("late failure reported success: %v %d", err, failed.Code)
	}

	effectCounts := []string{
		"SELECT COUNT(*) FROM posts",
		"SELECT COUNT(*) FROM post_votes",
		"SELECT COUNT(*) FROM post_subscribers",
		"SELECT COUNT(*) FROM attachments",
		"SELECT COUNT(*) FROM post_notification_deliveries",
	}

	for _, query := range effectCounts {
		if count := workflowCount(t, query); count != 0 {
			t.Errorf("%s: failed operation retained %d rows", query, count)
		}
	}

	bus.Init(postgres.Service{}, blobsql.Service{})

	recovered, err := f.request(apiv1.CreatePost(), http.MethodPost, 0, body)
	if err != nil || recovered.Code != http.StatusOK {
		t.Fatalf("recovery failed: %v %d %s", err, recovered.Code, recovered.Body)
	}

	if workflowCount(t, "SELECT number FROM posts") != 1 {
		t.Fatal("rolled-back number was consumed")
	}
}

func TestPostWorkflowIndependentConcurrentCreates(t *testing.T) {
	f := newPostWorkflow(t)
	var writers sync.WaitGroup

	for i := 0; i < 24; i++ {
		body := submissionBody(t, fmt.Sprintf("independent-%d", i), false)
		writers.Add(1)
		go func() {
			defer writers.Done()

			recorder, err := f.request(apiv1.CreatePost(), http.MethodPost, 0, body)
			if err != nil || recorder.Code != http.StatusOK {
				t.Errorf("create failed: %v %d %s", err, recorder.Code, recorder.Body)
			}
		}()
	}

	writers.Wait()

	var count, distinct, minimum, maximum int
	err := dbx.Connection().QueryRow(`
		SELECT COUNT(*), COUNT(DISTINCT number), MIN(number), MAX(number) FROM posts
	`).Scan(&count, &distinct, &minimum, &maximum)
	if err != nil {
		t.Fatal(err)
	}

	if count != 24 || distinct != 24 || minimum != 1 || maximum != 24 {
		t.Fatalf("numbering: count=%d distinct=%d range=%d..%d", count, distinct, minimum, maximum)
	}
}

func BenchmarkPostWorkflowCreate(b *testing.B) {
	f := newPostWorkflow(b)
	var sequence atomic.Int64

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			id := fmt.Sprintf("benchmark-%d", sequence.Add(1))
			body := submissionBody(b, id, false)

			recorder, err := f.request(apiv1.CreatePost(), http.MethodPost, 0, body)
			if err != nil || recorder.Code != http.StatusOK {
				b.Errorf("create failed: %v %d %s", err, recorder.Code, recorder.Body)
			}
		}
	})
}

func TestPostWorkflowBackupSnapshot(t *testing.T) {
	f := newPostWorkflow(t)
	body := submissionBody(t, "backup", false)

	_, err := dbx.Connection().Exec(`
        INSERT INTO pages (tenant_id, created_by_id, updated_by_id, title, slug, content)
        VALUES (1, 1, 1, 'Exported Page', 'exported', 'Exported content'),
               (2, 4, 4, 'Other tenant', 'private', 'Other content');

        INSERT INTO page_drafts (page_id, tenant_id, user_id, content)
        VALUES (1, 1, 1, 'Unpublished changes'), (2, 2, 4, 'Other draft');

        INSERT INTO page_authors (page_id, user_id) VALUES (1, 1), (2, 4);
        INSERT INTO page_subscriptions (page_id, user_id) VALUES (1, 2), (2, 5);
        INSERT INTO page_reactions (page_id, user_id, emoji) VALUES (1, 2, '👍'), (2, 5, '👍');

        INSERT INTO page_topics (tenant_id, name, slug)
        VALUES (1, 'Exported topic', 'exported'), (2, 'Other topic', 'private');
        INSERT INTO page_topics_map (page_id, topic_id) VALUES (1, 1), (2, 2);

        INSERT INTO page_tags (tenant_id, name, slug)
        VALUES (1, 'Exported tag', 'exported'), (2, 'Other tag', 'private');
        INSERT INTO page_tags_map (page_id, tag_id) VALUES (1, 1), (2, 2);

        INSERT INTO comments (tenant_id, page_id, user_id, content, created_at)
        VALUES (1, 1, 2, 'Exported comment', NOW()),
               (2, 2, 5, 'Other comment', NOW());
        INSERT INTO reactions (comment_id, user_id, emoji, created_on)
        VALUES (1, 2, '👍', NOW()), (2, 5, '👍', NOW());
    `)
	if err != nil {
		t.Fatal(err)
	}

	recorder, err := f.request(apiv1.CreatePost(), http.MethodPost, 0, body)
	if err != nil || recorder.Code != http.StatusOK {
		t.Fatalf("setup: %v %s", err, recorder.Body)
	}

	prepare := &cmd.ProcessPostNotification{
		Prepare: func(context.Context, *entity.Post) ([]cmd.PostNotificationRecipient, error) {
			return []cmd.PostNotificationRecipient{
				{Channel: "email", ID: 2},
			}, nil
		},
	}

	if err := bus.Dispatch(f.ctx, prepare); err != nil {
		t.Fatal(err)
	}

	blocker, err := dbx.Connection().Begin()
	if err != nil {
		t.Fatal(err)
	}

	defer blocker.Rollback()

	if _, err := blocker.Exec("LOCK TABLE post_subscribers IN ACCESS EXCLUSIVE MODE"); err != nil {
		t.Fatal(err)
	}

	type result struct {
		archive *bytes.Buffer
		err     error
	}

	completed := make(chan result, 1)
	ctx, cancel := context.WithTimeout(f.ctx, 10*time.Second)
	defer cancel()

	go func() {
		archive, err := backup.Create(ctx)
		completed <- result{
			archive: archive,
			err:     err,
		}
	}()

	deadline := time.Now().Add(5 * time.Second)
	for workflowCount(t, `SELECT COUNT(*) FROM pg_stat_activity
		WHERE datname = current_database() AND wait_event_type = 'Lock'
		AND query LIKE 'SELECT * FROM %post_subscribers%'`) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("backup did not reach controlled export boundary")
		}

		time.Sleep(10 * time.Millisecond)
	}

	_, err = dbx.Connection().Exec(`WITH inserted AS (
		INSERT INTO posts (tenant_id, user_id, title, slug, description, created_at, status)
		VALUES ($1, $2, $3, $3, $4, NOW(), $5) RETURNING id
	) INSERT INTO post_votes (tenant_id, user_id, post_id, created_at, vote_type)
		SELECT $1, $2, id, NOW(), $6 FROM inserted`,
		f.tenant.ID, f.user.ID, "concurrent", "during backup", enum.PostOpen, enum.VoteTypeUp)
	if err != nil {
		t.Fatal(err)
	}

	if err := blocker.Commit(); err != nil {
		t.Fatal(err)
	}

	output := <-completed
	if output.err != nil {
		t.Fatal(output.err)
	}

	archive, err := zip.NewReader(bytes.NewReader(output.archive.Bytes()), int64(output.archive.Len()))
	if err != nil {
		t.Fatal(err)
	}

	expectedFiles := map[string]int{
		"pages.json":                        1,
		"page_drafts.json":                  1,
		"page_authors.json":                 1,
		"page_subscriptions.json":           1,
		"page_reactions.json":               1,
		"page_topics.json":                  1,
		"page_topics_map.json":              1,
		"page_tags.json":                    1,
		"page_tags_map.json":                1,
		"comments.json":                     1,
		"reactions.json":                    1,
		"posts.json":                        1,
		"post_votes.json":                   1,
		"post_vote_revisions.json":          1,
		"post_notification_deliveries.json": 1,
		"post_notification_recipients.json": 1,
	}

	for _, file := range archive.File {
		expectedCount, expected := expectedFiles[file.Name]
		if !expected {
			continue
		}

		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}

		content, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}

		var rows []map[string]any
		if err := json.Unmarshal(content, &rows); err != nil {
			t.Fatalf("invalid %s: %v", file.Name, err)
		}

		if len(rows) != expectedCount {
			t.Fatalf("inconsistent %s: want %d rows, got %d", file.Name, expectedCount, len(rows))
		}

		if pageID, present := rows[0]["page_id"]; present && pageID != nil && pageID != float64(1) {
			t.Fatalf("%s contains another tenant's Page: %v", file.Name, rows[0])
		}

		if commentID, present := rows[0]["comment_id"]; present && commentID != nil && commentID != float64(1) {
			t.Fatalf("%s contains another tenant's comment: %v", file.Name, rows[0])
		}

		if tenantID, present := rows[0]["tenant_id"]; present && tenantID != float64(f.tenant.ID) {
			t.Fatalf("%s contains another tenant's data: %v", file.Name, rows[0])
		}

		if file.Name == "posts.json" && rows[0]["submission_id"] != "backup" {
			t.Fatal("backup omitted submission identity")
		}

		delete(expectedFiles, file.Name)
	}

	if len(expectedFiles) != 0 {
		t.Fatalf("backup omitted files: %v", expectedFiles)
	}
}
