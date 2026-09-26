package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
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
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
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
	req := httptest.NewRequest(method, "http://localhost:3000/api/v1/posts", strings.NewReader(body))
	req.RequestURI = req.URL.RequestURI()
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	c := web.NewContext(f.engine, req, recorder, web.StringMap{"number": fmt.Sprint(number)})
	c.SetTenant(f.tenant)
	c.SetUser(f.user)
	err := handler(c)
	return recorder, err
}

func workflowCount(t testing.TB, sql string) int {
	t.Helper()
	var count int
	if err := dbx.Connection().QueryRow(sql).Scan(&count); err != nil {
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

func TestPostWorkflowLegacyToggleAndStaleStatus(t *testing.T) {
	f := newPostWorkflow(t)
	post := &cmd.AddNewPost{Title: "Legacy vote concurrency", Description: "A post"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}
	var voters sync.WaitGroup
	for i := 0; i < 16; i++ {
		voters.Add(1)
		go func() {
			defer voters.Done()
			recorder, err := f.request(apiv1.ToggleVote(), http.MethodPost, post.Result.Number, "")
			if err != nil || recorder.Code != http.StatusOK {
				t.Errorf("legacy toggle failed: %v %d", err, recorder.Code)
			}
		}()
	}
	voters.Wait()
	if workflowCount(t, "SELECT vote_type FROM post_votes") != -1 ||
		workflowCount(t, "SELECT revision FROM post_vote_revisions") != 16 {
		t.Fatal("concurrent toggles lost an accepted change")
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

func TestPostWorkflowLegacyVoteContracts(t *testing.T) {
	f := newPostWorkflow(t)
	post := &cmd.AddNewPost{Title: "Legacy voting", Description: "A post"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		handler   web.HandlerFunc
		method    string
		response  string
		upvotes   int
		downvotes int
	}{
		{apiv1.AddVote(), http.MethodPost, "{}", 1, 0},
		{apiv1.AddVote(), http.MethodPost, "{}", 1, 0},
		{apiv1.AddDownVote(), http.MethodPost, "{}", 0, 1},
		{apiv1.ToggleVote(), http.MethodPost, `{"voted":true}`, 1, 0},
		{apiv1.ToggleVote(), http.MethodPost, `{"voted":false}`, 0, 1},
		{apiv1.RemoveVote(), http.MethodDelete, "{}", 0, 0},
	} {
		recorder, err := f.request(step.handler, step.method, post.Result.Number, "")
		if err != nil || recorder.Code != http.StatusOK || strings.TrimSpace(recorder.Body.String()) != step.response {
			t.Fatalf("legacy response: %v %d %s", err, recorder.Code, recorder.Body)
		}
		if workflowCount(t, "SELECT upvotes FROM posts") != step.upvotes ||
			workflowCount(t, "SELECT downvotes FROM posts") != step.downvotes {
			t.Fatalf("legacy counts: expected %d up, %d down", step.upvotes, step.downvotes)
		}
	}
	recorder, err := f.request(apiv1.AddVote(), http.MethodPost, post.Result.Number, `{"revision":0}`)
	if err != nil || recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"applied":false`) {
		t.Fatalf("stale revision after legacy changes: %v %d %s", err, recorder.Code, recorder.Body)
	}
	visitor := f
	visitor.user = &entity.User{ID: 2, Role: enum.RoleVisitor}
	for _, step := range []struct {
		settings string
		status   int
	}{
		{`locked_settings = '{"locked":true}'`, http.StatusBadRequest},
		{`locked_settings = NULL, status = 2`, http.StatusOK},
		{`status = 6`, http.StatusNotFound},
	} {
		if _, err := dbx.Connection().Exec("UPDATE posts SET " + step.settings); err != nil {
			t.Fatal(err)
		}
		for _, handler := range []web.HandlerFunc{apiv1.AddVote(), apiv1.AddDownVote(), apiv1.RemoveVote()} {
			recorder, err := visitor.request(handler, http.MethodPost, post.Result.Number, "")
			if err != nil || recorder.Code != step.status || workflowCount(t, "SELECT COUNT(*) FROM post_votes") != 0 {
				t.Fatalf("legacy permission %s: %v %d %s", step.settings, err, recorder.Code, recorder.Body)
			}
		}
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
