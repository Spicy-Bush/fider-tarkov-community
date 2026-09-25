package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/assets"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/mock"
	blobsql "github.com/Spicy-Bush/fider-tarkov-community/app/services/blob/sql"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/moderation"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/sqlstore/postgres"
)

func moderationDatabase(t testing.TB) context.Context {
	t.Helper()
	bus.Reset()
	bus.Init(postgres.Service{})
	old := env.Config.OpenAI
	env.Config.OpenAI.APIKey = "test-key"
	env.Config.OpenAI.ModerationEnabled = true
	env.Config.OpenAI.Concurrency = 2
	t.Cleanup(func() { env.Config.OpenAI = old })

	if _, err := dbx.Connection().Exec(`DELETE FROM moderation_checks; UPDATE moderation_provider SET available_at=NOW(); UPDATE moderation_provider_slots SET lease_until=NOW()`); err != nil {
		t.Fatal(err)
	}

	ctx := context.WithValue(context.Background(), app.TenantCtxKey, &entity.Tenant{ID: 1})
	return ctx
}

func dispatchModeration(t *testing.T, ctx context.Context, message any) {
	t.Helper()

	if err := bus.Dispatch(ctx, message); err != nil {
		t.Fatal(err)
	}
}

func createModerationPost(t *testing.T, ctx context.Context) int {
	t.Helper()
	var id int
	err := dbx.Connection().QueryRow(`INSERT INTO posts(title,description,slug,number,tenant_id,user_id,created_at,status)
      VALUES('Title to check','Description to check','moderation-test-' || (SELECT COALESCE(MAX(number),0)+1 FROM posts WHERE tenant_id=1),(SELECT COALESCE(MAX(number),0)+1 FROM posts WHERE tenant_id=1),1,1,NOW(),0) RETURNING id`).Scan(&id)

	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		dbx.Connection().Exec(`DELETE FROM reports WHERE reported_type='post' AND reported_id=$1;`, id)
		dbx.Connection().Exec(`DELETE FROM posts WHERE id=$1`, id)
	})

	dispatchModeration(t, ctx, &cmd.ScheduleModeration{ContentType: "post", ContentID: id})
	return id
}

func takeModeration(t *testing.T) cmd.ModerationCheck {
	t.Helper()
	claim := &cmd.ClaimModeration{}
	dispatchModeration(t, context.Background(), claim)

	if claim.Result == nil {
		t.Fatal("expected a due check")
	}

	return *claim.Result
}

func TestModerationCheckTransactionAndRecovery(t *testing.T) {
	ctx := moderationDatabase(t)
	id := createModerationPost(t, ctx)
	first := takeModeration(t)

	if first.Text != "Title to check\n\nDescription to check" {
		t.Fatalf("title/description missing: %q", first.Text)
	}

	retry := &cmd.FinishModeration{Check: first, Outcome: cmd.ModerationRetry, RetryAfterSeconds: 60, Error: "temporary provider failure"}
	dispatchModeration(t, ctx, retry)
	early := &cmd.ClaimModeration{}
	dispatchModeration(t, ctx, early)

	if early.Result != nil {
		t.Fatal("retried before due time")
	}

	if _, err := dbx.Connection().Exec(`UPDATE moderation_checks SET next_attempt_at=NOW() WHERE content_id=$1`, id); err != nil {
		t.Fatal(err)
	}

	second := takeModeration(t)

	if second.Attempts != 2 {
		t.Fatalf("attempts=%d", second.Attempts)
	}

	done := &cmd.FinishModeration{Check: second, Outcome: cmd.ModerationReviewed, Findings: []cmd.ModerationFinding{{Category: "sexual", Score: 0.95}}, Result: `{"results":[]}`}
	dispatchModeration(t, ctx, done)
	duplicate := &cmd.FinishModeration{Check: second, Outcome: cmd.ModerationReviewed, Findings: []cmd.ModerationFinding{{Category: "sexual", Score: 0.95}}, Result: `{"results":[]}`}
	dispatchModeration(t, ctx, duplicate)
	var reports int

	if err := dbx.Connection().QueryRow(`SELECT COUNT(*) FROM reports WHERE reported_id=$1 AND reported_type='post' AND reporter_id IS NULL`, id).Scan(&reports); err != nil {
		t.Fatal(err)
	}

	if !done.Applied || duplicate.Applied || reports != 1 {
		t.Fatalf("completion/replay: applied=%v duplicate=%v reports=%d", done.Applied, duplicate.Applied, reports)
	}

	tx, err := dbx.BeginTx(ctx)

	if err != nil {
		t.Fatal(err)
	}

	txctx := context.WithValue(ctx, app.TransactionCtxKey, tx)
	dispatchModeration(t, txctx, &cmd.ScheduleModeration{ContentType: "post", ContentID: id})

	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}

	claim := &cmd.ClaimModeration{}
	dispatchModeration(t, ctx, claim)

	if claim.Result != nil {
		t.Fatal("rolled-back scheduling escaped transaction")
	}
}

func TestModerationStaleClaimsAndManualApproval(t *testing.T) {
	ctx := moderationDatabase(t)
	id := createModerationPost(t, ctx)
	first := takeModeration(t)

	if _, err := dbx.Connection().Exec(`UPDATE moderation_checks SET next_attempt_at=NOW()-INTERVAL '1 second'`); err != nil {
		t.Fatal(err)
	}

	reclaimed := takeModeration(t)
	stale := &cmd.FinishModeration{Check: first, Outcome: cmd.ModerationReviewed, Findings: []cmd.ModerationFinding{{Category: "sexual", Score: 0.95}}}
	dispatchModeration(t, ctx, stale)

	if stale.Applied {
		t.Fatal("expired claimant overwrote reclaimed check")
	}

	dispatchModeration(t, ctx, &cmd.ScheduleModeration{ContentType: "post", ContentID: id})
	stale = &cmd.FinishModeration{Check: reclaimed, Outcome: cmd.ModerationReviewed, Findings: []cmd.ModerationFinding{{Category: "sexual", Score: 0.95}}}
	dispatchModeration(t, ctx, stale)

	if stale.Applied {
		t.Fatal("old revision overwrote new edit")
	}

	current := takeModeration(t)
	dispatchModeration(t, ctx, &cmd.SetModerationPending{ContentType: "post", ContentID: id, Pending: false})
	stale = &cmd.FinishModeration{Check: current, Outcome: cmd.ModerationReviewed, Findings: []cmd.ModerationFinding{{Category: "sexual", Score: 0.95}}}
	dispatchModeration(t, ctx, stale)

	if stale.Applied {
		t.Fatal("automatic result overrode manual approval")
	}
}

func TestModerationProfileSaveOutageAndLatestEdit(t *testing.T) {
	ctx := moderationDatabase(t)
	var original string

	if err := dbx.Connection().QueryRow(`SELECT name FROM users WHERE id=1`).Scan(&original); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { dbx.Connection().Exec(`UPDATE users SET name=$1 WHERE id=1`, original) })
	start := time.Now()
	change := &cmd.SaveProfileName{UserID: 1, Name: "First proposed name", Review: true}
	dispatchModeration(t, ctx, change)
	t.Logf("durable profile save without provider: %s", time.Since(start))
	first := takeModeration(t)
	dispatchModeration(t, ctx, &cmd.FinishModeration{Check: first, Outcome: cmd.ModerationRetry, RetryAfterSeconds: 3600, Error: "quota exhausted"})
	var public string

	if err := dbx.Connection().QueryRow(`SELECT name FROM users WHERE id=1`).Scan(&public); err != nil {
		t.Fatal(err)
	}

	if public != "First proposed name" || !change.Pending {
		t.Fatal("provider failure did not publish the saved proposal")
	}

	status := &cmd.GetProfileModeration{UserID: 1}
	dispatchModeration(t, ctx, status)

	if len(status.Result) != 1 || status.Result[0].Text != "First proposed name" {
		t.Fatal("saved proposal lost during outage")
	}

	dispatchModeration(t, ctx, &cmd.SaveProfileName{UserID: 1, Name: "Latest proposed name", Review: true})
	second := takeModeration(t)
	stale := &cmd.FinishModeration{Check: first, Outcome: cmd.ModerationReviewed}
	dispatchModeration(t, ctx, stale)

	if stale.Applied {
		t.Fatal("first edit replaced newer edit")
	}

	done := &cmd.FinishModeration{Check: second, Outcome: cmd.ModerationReviewed}
	dispatchModeration(t, ctx, done)

	if err := dbx.Connection().QueryRow(`SELECT name FROM users WHERE id=1`).Scan(&public); err != nil {
		t.Fatal(err)
	}

	if public != "Latest proposed name" {
		t.Fatalf("recovery failed: %s", public)
	}

	dispatchModeration(t, ctx, &cmd.SaveProfileName{UserID: 1, Name: "Rejected proposed name", Review: true})
	third := takeModeration(t)
	dispatchModeration(t, ctx, &cmd.FinishModeration{Check: third, Outcome: cmd.ModerationReviewed, Findings: []cmd.ModerationFinding{{Category: "sexual", Score: 0.95}}})
	dispatchModeration(t, ctx, status)

	if status.Result[0].State != "rejected" {
		t.Fatal("rejected proposal has no visible outcome")
	}

	if err := dbx.Connection().QueryRow(`SELECT name FROM users WHERE id=1`).Scan(&public); err != nil {
		t.Fatal(err)
	}

	if public != "Latest proposed name" {
		t.Fatal("rejected proposal damaged public profile")
	}
}

func TestModerationProviderConcurrencyAndCooldown(t *testing.T) {
	ctx := moderationDatabase(t)

	for i := 0; i < 8; i++ {
		createModerationPost(t, ctx)
	}

	claims := make(chan *cmd.ModerationCheck, 8)
	failures := make(chan error, 8)
	var wg sync.WaitGroup

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := &cmd.ClaimModeration{}

			if err := bus.Dispatch(ctx, c); err != nil {
				failures <- err
			}

			claims <- c.Result
		}()
	}

	wg.Wait()
	close(claims)
	close(failures)

	for err := range failures {
		t.Fatal(err)
	}

	acquired := []*cmd.ModerationCheck{}

	for claim := range claims {
		if claim != nil {
			acquired = append(acquired, claim)
		}
	}

	if len(acquired) != 2 {
		t.Fatalf("limit=2 acquired=%d", len(acquired))
	}

	// Editing a running job cannot create another provider slot.
	dispatchModeration(t, ctx, &cmd.ScheduleModeration{ContentType: "post", ContentID: acquired[0].ContentID})
	blocked := &cmd.ClaimModeration{}
	dispatchModeration(t, ctx, blocked)

	if blocked.Result != nil {
		t.Fatal("edit bypassed provider capacity")
	}

	for _, claim := range acquired {
		dispatchModeration(t, ctx, &cmd.FinishModeration{Check: *claim, Outcome: cmd.ModerationRetry, CooldownSeconds: 60})
	}

	dispatchModeration(t, ctx, blocked)

	if blocked.Result != nil {
		t.Fatal("shared cooldown not honored")
	}

	if _, err := dbx.Connection().Exec(`UPDATE moderation_provider SET available_at=NOW()`); err != nil {
		t.Fatal(err)
	}

	dispatchModeration(t, ctx, blocked)

	if blocked.Result == nil {
		t.Fatal("queue did not resume after cooldown")
	}
}

func TestModerationFailedCompletionRollsBack(t *testing.T) {
	ctx := moderationDatabase(t)
	id := createModerationPost(t, ctx)
	check := takeModeration(t)
	bad := &cmd.FinishModeration{Check: check, Outcome: cmd.ModerationReviewed, Findings: []cmd.ModerationFinding{{Category: "sexual", Score: 0.95}}, Result: "not json"}

	if err := bus.Dispatch(ctx, bad); err == nil {
		t.Fatal("expected invalid JSON to abort completion")
	}

	var state string

	if err := dbx.Connection().QueryRow(`SELECT state FROM moderation_checks WHERE content_id=$1`, id).Scan(&state); err != nil {
		t.Fatal(err)
	}

	if state != "running" {
		t.Fatal("failed transaction lost its claim")
	}

	result, _ := json.Marshal(map[string]any{"results": []string{}})
	dispatchModeration(t, ctx, &cmd.FinishModeration{Check: check, Outcome: cmd.ModerationReviewed, Findings: []cmd.ModerationFinding{{Category: "sexual", Score: 0.95}}, Result: string(result)})
	t.Log(fmt.Sprintf("completion recovered for post %d", id))
}

func TestModerationWorkerRecoversSavedAvatar(t *testing.T) {
	ctx := moderationDatabase(t)
	var originalKey string
	var originalType int

	if err := dbx.Connection().QueryRow(`SELECT avatar_bkey,avatar_type FROM users WHERE id=1`).Scan(&originalKey, &originalType); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		dbx.Connection().Exec(`UPDATE users SET avatar_bkey=$1,avatar_type=$2 WHERE id=1`, originalKey, originalType)
	})

	available := false
	bus.AddHandler(func(ctx context.Context, q *query.GetBlobByKey) error {
		if !available {
			return fmt.Errorf("storage temporarily unavailable")
		}

		q.Result = &dto.Blob{Content: []byte("image bytes"), ContentType: "image/png"}
		return nil
	})

	status := 429
	requests := 0
	bus.AddHandler(func(ctx context.Context, c *cmd.HTTPRequest) error {
		requests++
		c.ResponseStatusCode = status
		c.ResponseHeader = http.Header{"Retry-After": []string{"120"}, "X-Request-Id": []string{"req_test"}}
		c.ResponseBody = []byte(`{"error":{"code":"rate_limit_exceeded","message":"private submitted content"}}`)

		if status == 200 {
			c.ResponseBody = []byte(`{"results":[{"category_scores":{"sexual":0,"sexual/minors":0,"self-harm":0,"self-harm/intent":0,"self-harm/instructions":0}}]}`)
		}

		return nil
	})

	dispatchModeration(t, ctx, &cmd.SaveProfileAvatar{UserID: 1, AvatarType: enum.AvatarTypeCustom, BlobKey: "avatars/saved-test", Review: true})
	run := func() {
		t.Helper()

		if worked, err := moderation.ProcessNext(context.Background()); err != nil || !worked {
			t.Fatalf("worker: worked=%v err=%v", worked, err)
		}
	}

	due := func() {
		t.Helper()

		if _, err := dbx.Connection().Exec(`UPDATE moderation_checks SET next_attempt_at=NOW(); UPDATE moderation_provider SET available_at=NOW()`); err != nil {
			t.Fatal(err)
		}
	}

	run()

	if requests != 0 {
		t.Fatal("missing image was submitted as empty success")
	}

	available = true
	due()
	run()
	var state, diagnostic, key string

	if err := dbx.Connection().QueryRow(`SELECT state,last_error FROM moderation_checks WHERE content_type='avatar'`).Scan(&state, &diagnostic); err != nil {
		t.Fatal(err)
	}

	if state != "pending" || !strings.Contains(diagnostic, "rate_limit_exceeded") || strings.Contains(diagnostic, "private") {
		t.Fatalf("lost failure diagnostics: %s %s", state, diagnostic)
	}

	if err := dbx.Connection().QueryRow(`SELECT avatar_bkey FROM users WHERE id=1`).Scan(&key); err != nil {
		t.Fatal(err)
	}

	if key != "avatars/saved-test" {
		t.Fatal("provider failure did not publish the saved avatar")
	}

	status = 200
	due()
	run()

	if err := dbx.Connection().QueryRow(`SELECT avatar_bkey FROM users WHERE id=1`).Scan(&key); err != nil {
		t.Fatal(err)
	}

	if key != "avatars/saved-test" || requests != 2 {
		t.Fatalf("recovery: avatar=%s requests=%d", key, requests)
	}
}

func TestModerationWorkerClassifiesFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		state  string
	}{
		{"credentials", 401, `{"error":{"code":"invalid_api_key"}}`, "pending"},
		{"server", 503, `{}`, "pending"},
		{"empty", 200, `{"results":[]}`, "pending"},
		{"missing scores", 200, `{"results":[{}]}`, "pending"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := moderationDatabase(t)
			createModerationPost(t, ctx)
			bus.AddHandler(func(ctx context.Context, c *cmd.HTTPRequest) error {
				c.ResponseStatusCode = tc.status
				c.ResponseBody = []byte(tc.body)
				return nil
			})

			if _, err := moderation.ProcessNext(context.Background()); err != nil {
				t.Fatal(err)
			}

			var state string

			if err := dbx.Connection().QueryRow(`SELECT state FROM moderation_checks`).Scan(&state); err != nil {
				t.Fatal(err)
			}

			if state != tc.state {
				t.Fatalf("state=%s want %s", state, tc.state)
			}
		})
	}
}

func TestModerationProfileHTTPWorkflow(t *testing.T) {
	ctx := moderationDatabase(t)
	assets.FS = os.DirFS(env.Path("."))
	var original string

	if err := dbx.Connection().QueryRow(`SELECT name FROM users WHERE id=1`).Scan(&original); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { dbx.Connection().Exec(`UPDATE users SET name=$1 WHERE id=1`, original) })
	server := mock.NewServer().OnTenant(mock.DemoTenant).AsUser(mock.JonSnow)
	code, response := server.ExecutePost(handlers.UpdateUserName(), `{"name":"Saved through HTTP"}`)

	if code != 200 {
		t.Fatalf("save: %d %s", code, response.Body.String())
	}

	var saved struct {
		Name    string `json:"name"`
		Pending bool   `json:"pending"`
	}

	if err := json.Unmarshal(response.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}

	if !saved.Pending || saved.Name != original {
		t.Fatalf("save response: %+v", saved)
	}

	code, response = mock.NewServer().OnTenant(mock.DemoTenant).AsUser(mock.JonSnow).Execute(handlers.ProfileModerationStatus())

	if code != 200 || !strings.Contains(response.Body.String(), "Saved through HTTP") {
		t.Fatalf("reload: %d %s", code, response.Body.String())
	}

	check := takeModeration(t)
	dispatchModeration(t, ctx, &cmd.FinishModeration{Check: check, Outcome: cmd.ModerationReviewed})
	code, response = mock.NewServer().OnTenant(mock.DemoTenant).AsUser(mock.JonSnow).Execute(handlers.ProfileModerationStatus())

	if code != 200 || !strings.Contains(response.Body.String(), `"name":"Saved through HTTP"`) {
		t.Fatalf("completion: %d %s", code, response.Body.String())
	}
}

func TestModerationPendingAvatarFileLifecycle(t *testing.T) {
	ctx := moderationDatabase(t)
	dispatchModeration(t, ctx, &cmd.SaveProfileAvatar{UserID: 1, AvatarType: enum.AvatarTypeCustom, BlobKey: "avatars/before", Review: true})
	first := takeModeration(t)
	usage := &query.IsImageFileInUse{BlobKey: "avatars/before"}
	dispatchModeration(t, ctx, usage)

	if !usage.Result {
		t.Fatal("pending avatar considered unused")
	}

	dispatchModeration(t, ctx, &cmd.UpdateImageFileReferences{OldBlobKey: "avatars/before", NewBlobKey: "avatars/after"})
	stale := &cmd.FinishModeration{Check: first, Outcome: cmd.ModerationReviewed}
	dispatchModeration(t, ctx, stale)

	if stale.Applied {
		t.Fatal("renamed image applied stale key")
	}

	second := takeModeration(t)

	if len(second.BlobKeys) != 1 || second.BlobKeys[0] != "avatars/after" {
		t.Fatalf("renamed proposal lost: %+v", second)
	}

	dispatchModeration(t, ctx, &cmd.DeleteImageFileReferences{BlobKey: "avatars/after"})
	stale = &cmd.FinishModeration{Check: second, Outcome: cmd.ModerationReviewed}
	dispatchModeration(t, ctx, stale)

	if stale.Applied {
		t.Fatal("deleted avatar became public")
	}
}

// Includes durable submission, claiming, provider admission and completion.
// The provider response is fixed so this measures local work, not network latency.
func BenchmarkModerationWorkflow(b *testing.B) {
	bus.Reset()
	bus.Init(postgres.Service{})
	old := env.Config.OpenAI
	env.Config.OpenAI.APIKey = "test-key"
	env.Config.OpenAI.ModerationEnabled = true
	env.Config.OpenAI.Concurrency = 2
	b.Cleanup(func() { env.Config.OpenAI = old })
	_, err := dbx.Connection().Exec(`DELETE FROM moderation_checks; UPDATE moderation_provider SET available_at=NOW(); UPDATE moderation_provider_slots SET lease_until=NOW()`)

	if err != nil {
		b.Fatal(err)
	}

	var id int
	err = dbx.Connection().QueryRow(`INSERT INTO posts(title,description,slug,number,tenant_id,user_id,created_at,status)
 VALUES('Benchmark title','Benchmark description','moderation-bench',(SELECT COALESCE(MAX(number),0)+1 FROM posts WHERE tenant_id=1),1,1,NOW(),0) RETURNING id`).Scan(&id)

	if err != nil {
		b.Fatal(err)
	}

	b.Cleanup(func() {
		dbx.Connection().Exec(`DELETE FROM moderation_checks WHERE content_type='post' AND content_id=$1`, id)
		dbx.Connection().Exec(`DELETE FROM posts WHERE id=$1`, id)
	})

	ctx := context.WithValue(context.Background(), app.TenantCtxKey, &entity.Tenant{ID: 1})
	requests := 0
	bus.AddHandler(func(ctx context.Context, c *cmd.HTTPRequest) error {
		requests++
		c.ResponseStatusCode = 200
		c.ResponseBody = []byte(`{"results":[{"category_scores":{"sexual":0,"sexual/minors":0,"self-harm":0,"self-harm/intent":0,"self-harm/instructions":0}}]}`)
		return nil
	})

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err := bus.Dispatch(ctx, &cmd.ScheduleModeration{ContentType: "post", ContentID: id}); err != nil {
			b.Fatal(err)
		}

		if worked, err := moderation.ProcessNext(ctx); err != nil || !worked {
			b.Fatalf("worked=%v err=%v", worked, err)
		}
	}

	b.StopTimer()
	var state string

	if err := dbx.Connection().QueryRow(`SELECT state FROM moderation_checks WHERE content_type='post' AND content_id=$1`, id).Scan(&state); err != nil {
		b.Fatal(err)
	}

	if state != "complete" || requests != b.N {
		b.Fatalf("state=%s requests=%d operations=%d", state, requests, b.N)
	}
}

func TestModerationAvatarPublication(t *testing.T) {
	ctx := moderationDatabase(t)
	bus.Init(blobsql.Service{})
	assets.FS = os.DirFS(env.Path("."))
	var oldKey string
	var oldType int

	if err := dbx.Connection().QueryRow(`SELECT avatar_bkey,avatar_type FROM users WHERE id=1`).Scan(&oldKey, &oldType); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		dbx.Connection().Exec(`UPDATE users SET avatar_bkey=$1,avatar_type=$2 WHERE id=1`, oldKey, oldType)
		dbx.Connection().Exec(`DELETE FROM blobs WHERE tenant_id=1 AND key IN ('avatars/publication-test','avatars/rejected-test')`)
	})

	for _, key := range []string{"avatars/publication-test", "avatars/rejected-test"} {
		dispatchModeration(t, ctx, &cmd.StoreBlob{Key: key, Content: []byte("image fixture"), ContentType: "image/png"})
	}

	dispatchModeration(t, ctx, &cmd.SaveProfileAvatar{UserID: 1, AvatarType: enum.AvatarTypeCustom, BlobKey: "avatars/publication-test", Review: true})
	public := func(key string) int {
		code, _ := mock.NewServer().OnTenant(mock.DemoTenant).AddParam("bkey", key).Execute(handlers.ViewUploadedImage())
		return code
	}

	if code := public("avatars/publication-test"); code != 404 {
		t.Fatalf("pending public image: %d", code)
	}

	code, response := mock.NewServer().OnTenant(mock.DemoTenant).AsUser(mock.JonSnow).WithURL("http://demo/_api/user/moderation/avatar?revision=1").Execute(handlers.PreviewProfileAvatar())

	if code != 200 || response.Body.String() != "image fixture" || response.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("owner preview: %d %s", code, response.Body.String())
	}

	code, _ = mock.NewServer().OnTenant(mock.DemoTenant).AsUser(mock.AryaStark).WithURL("http://demo/_api/user/moderation/avatar?revision=1").Execute(handlers.PreviewProfileAvatar())

	if code != 404 {
		t.Fatalf("another user preview: %d", code)
	}

	check := takeModeration(t)
	dispatchModeration(t, ctx, &cmd.FinishModeration{Check: check, Outcome: cmd.ModerationReviewed})

	if code := public("avatars/publication-test"); code != 200 {
		t.Fatalf("approved public image: %d", code)
	}

	dispatchModeration(t, ctx, &cmd.SaveProfileAvatar{UserID: 1, AvatarType: enum.AvatarTypeCustom, BlobKey: "avatars/rejected-test", Review: true})
	check = takeModeration(t)
	dispatchModeration(t, ctx, &cmd.FinishModeration{Check: check, Outcome: cmd.ModerationReviewed, Findings: []cmd.ModerationFinding{{Category: "sexual", Score: 0.95}}})

	if code := public("avatars/rejected-test"); code != 404 {
		t.Fatalf("rejected public image: %d", code)
	}

	if code := public("avatars/publication-test"); code != 200 {
		t.Fatalf("previous approved avatar lost: %d", code)
	}
}

func TestModerationFailuresBeyondFirstPage(t *testing.T) {
	ctx := moderationDatabase(t)
	_, err := dbx.Connection().Exec(`INSERT INTO moderation_checks(tenant_id,content_type,content_id,state,text_content,updated_at)
 SELECT 1,'post',n,CASE WHEN n>100 THEN 'failed' ELSE 'pending' END,'fixture',NOW()+n*INTERVAL '1 second' FROM generate_series(1,201) n`)

	if err != nil {
		t.Fatal(err)
	}

	list := &cmd.ListModerationFailures{}
	dispatchModeration(t, ctx, list)

	if list.Total != 201 || list.Failed != 101 || len(list.Result) != 100 {
		t.Fatalf("counts: %+v", list)
	}

	for _, check := range list.Result {
		if check.State != "failed" {
			t.Fatal("failures were hidden behind pending checks")
		}
	}

	retry := &cmd.RetryModerationFailures{}
	dispatchModeration(t, ctx, retry)

	if retry.Count != 101 {
		t.Fatalf("retried %d failures", retry.Count)
	}
}

func TestModerationPruningProtectsSavedWork(t *testing.T) {
	ctx := moderationDatabase(t)
	bus.Init(blobsql.Service{})
	keys := []string{"avatars/prune-kept", "avatars/prune-unused"}
	t.Cleanup(func() {
		dbx.Connection().Exec(`DELETE FROM blobs WHERE tenant_id=1 AND key IN ('avatars/prune-kept','avatars/prune-unused')`)
	})

	for _, key := range keys {
		dispatchModeration(t, ctx, &cmd.StoreBlob{Key: key, Content: []byte("fixture"), ContentType: "image/png"})
	}

	dispatchModeration(t, ctx, &cmd.SaveProfileAvatar{UserID: 1, AvatarType: enum.AvatarTypeCustom, BlobKey: keys[0], Review: true})
	prunable := &query.GetPrunableFiles{}
	dispatchModeration(t, ctx, prunable)
	found := false

	for _, key := range prunable.Result {
		if key == keys[0] {
			t.Fatal("saved avatar selected for pruning")
		}

		if key == keys[1] {
			found = true
		}
	}

	if !found {
		t.Fatal("unused avatar retained")
	}
}

func BenchmarkModerationProfileHTTP(b *testing.B) {
	moderationDatabase(b)
	assets.FS = os.DirFS(env.Path("."))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		server := mock.NewServer().OnTenant(mock.DemoTenant).AsUser(mock.JonSnow)
		b.StartTimer()
		code, response := server.ExecutePost(handlers.UpdateUserName(), `{"name":"Saved benchmark proposal"}`)

		if code != 200 || !strings.Contains(response.Body.String(), `"pending":true`) {
			b.Fatalf("save: %d %s", code, response.Body.String())
		}
	}
}

func BenchmarkModerationIdleBacklog(b *testing.B) {
	moderationDatabase(b)

	if _, err := dbx.Connection().Exec(`INSERT INTO moderation_checks(tenant_id,content_type,content_id,state,text_content,next_attempt_at)
 SELECT 1,'post',n,'pending','fixture',NOW()+INTERVAL '1 hour' FROM generate_series(1,100000) n`); err != nil {
		b.Fatal(err)
	}

	b.Cleanup(func() { dbx.Connection().Exec(`DELETE FROM moderation_checks`) })
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		c := &cmd.ClaimModeration{}

		if err := bus.Dispatch(context.Background(), c); err != nil || c.Result != nil {
			b.Fatalf("claim=%+v err=%v", c.Result, err)
		}
	}
}

func BenchmarkModerationTenantLookup(b *testing.B) {
	moderationDatabase(b)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		q := &query.GetFirstTenant{}

		if err := bus.Dispatch(context.Background(), q); err != nil || q.Result == nil {
			b.Fatalf("tenant: %v", err)
		}
	}
}

func TestModerationProfileModel(t *testing.T) {
	ctx := moderationDatabase(t)
	var original string

	if err := dbx.Connection().QueryRow(`SELECT name FROM users WHERE id=1`).Scan(&original); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { dbx.Connection().Exec(`UPDATE users SET name=$1 WHERE id=1`, original) })
	random := rand.New(rand.NewSource(20260926))
	expectedPublic := original
	generation := 0
	pending := false
	type attempt struct {
		check      cmd.ModerationCheck
		generation int
	}

	attempts := []attempt{}

	for step := 0; step < 200; step++ {
		switch random.Intn(3) {
		case 0:
			generation++
			pending = true
			dispatchModeration(t, ctx, &cmd.SaveProfileName{UserID: 1, Name: fmt.Sprintf("Proposal %d", generation), Review: true})
		case 1:
			claim := &cmd.ClaimModeration{}
			dispatchModeration(t, ctx, claim)

			if claim.Result != nil {
				attempts = append(attempts, attempt{*claim.Result, generation})
			}
		case 2:

			if len(attempts) > 0 {
				index := random.Intn(len(attempts))
				a := attempts[index]
				attempts = append(attempts[:index], attempts[index+1:]...)
				var findings []cmd.ModerationFinding

				if random.Intn(3) == 0 {
					findings = []cmd.ModerationFinding{{Category: "sexual", Score: 0.95}}
				}

				dispatchModeration(t, ctx, &cmd.FinishModeration{Check: a.check, Outcome: cmd.ModerationReviewed, Findings: findings})

				if a.generation == generation && pending {
					pending = false

					if len(findings) == 0 {
						expectedPublic = fmt.Sprintf("Proposal %d", generation)
					}
				}
			}
		}

		var actual string

		if err := dbx.Connection().QueryRow(`SELECT name FROM users WHERE id=1`).Scan(&actual); err != nil {
			t.Fatal(err)
		}

		if actual != expectedPublic {
			t.Fatalf("seed=20260926 step=%d public=%q expected=%q", step, actual, expectedPublic)
		}
	}

	// Restore healthy conditions and complete the same saved intent.

	for _, a := range attempts {
		dispatchModeration(t, ctx, &cmd.FinishModeration{Check: a.check, Outcome: cmd.ModerationRetry, RetryAfterSeconds: 0})
	}

	claim := &cmd.ClaimModeration{}
	dispatchModeration(t, ctx, claim)

	if pending {
		if claim.Result == nil {
			t.Fatal("saved proposal cannot make progress")
		}

		dispatchModeration(t, ctx, &cmd.FinishModeration{Check: *claim.Result, Outcome: cmd.ModerationReviewed})
		var actual string

		if err := dbx.Connection().QueryRow(`SELECT name FROM users WHERE id=1`).Scan(&actual); err != nil {
			t.Fatal(err)
		}

		if actual != fmt.Sprintf("Proposal %d", generation) {
			t.Fatal("recovery lost latest saved proposal")
		}
	}
}

func TestModerationAvatarTypeSwitchCannotPublishUpload(t *testing.T) {
	moderationDatabase(t)
	bus.Init(blobsql.Service{})
	assets.FS = os.DirFS(env.Path("."))
	var oldKey string
	var oldType int

	if err := dbx.Connection().QueryRow(`SELECT avatar_bkey,avatar_type FROM users WHERE id=1`).Scan(&oldKey, &oldType); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		dbx.Connection().Exec(`UPDATE users SET avatar_bkey=$1,avatar_type=$2 WHERE id=1`, oldKey, oldType)
	})

	for _, kind := range []string{"letter", "gravatar"} {
		t.Run(kind, func(t *testing.T) {
			server := mock.NewServer().OnTenant(mock.DemoTenant).AsUser(mock.JonSnow)
			code, response := server.ExecutePost(handlers.UpdateUserAvatar(), fmt.Sprintf(`{"avatarType":%q,"avatar":{"upload":{"fileName":"ignored.png","content":"iVBORw0KGgoAAAANSUhEUgAAAEAAAABACAIAAAAlC+aJAAAAdklEQVR4nO3PQQkAMAzAwEqv9Ino4xgEIuAys/t3XtCAFjSgBQ1oQQNa0IAWNKAFDWhBA1rQgBY0oAUNaEEDWtCAFjSgBQ1oQQNa0IAWNKAFDWhBA1rQgBY0oAUNaEEDWtCAFjSgBQ1oQQNa0IAWNKAFDWjBrQfPEgDxw82rKQAAAABJRU5ErkJggg==","contentType":"image/png"}}}`, kind))

			if code != 200 {
				t.Fatalf("save non-custom: %d %s", code, response.Body.String())
			}

			var key string

			if err := dbx.Connection().QueryRow(`SELECT avatar_bkey FROM users WHERE id=1`).Scan(&key); err != nil {
				t.Fatal(err)
			}

			code, response = mock.NewServer().OnTenant(mock.DemoTenant).AsUser(mock.JonSnow).ExecutePost(handlers.UpdateUserAvatar(), `{"avatarType":"custom"}`)

			if code != 400 {
				t.Fatalf("custom without an image must fail validation: %d %s", code, response.Body.String())
			}

			if key != "" {
				published := &query.IsAvatarPublished{Key: key}
				dispatchModeration(t, context.WithValue(context.Background(), app.TenantCtxKey, mock.DemoTenant), published)
				t.Fatalf("non-custom submission stored unchecked key %q; published after switch=%v", key, published.Result)
			}
		})
	}
}

func TestModerationInactiveAvatarKeyRequiresReview(t *testing.T) {
	ctx := moderationDatabase(t)
	var oldKey string
	var oldType int

	if err := dbx.Connection().QueryRow(`SELECT avatar_bkey,avatar_type FROM users WHERE id=1`).Scan(&oldKey, &oldType); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		dbx.Connection().Exec(`UPDATE users SET avatar_bkey=$1,avatar_type=$2 WHERE id=1`, oldKey, oldType)
	})

	if _, err := dbx.Connection().Exec(`UPDATE users SET avatar_type=1,avatar_bkey='avatars/inactive' WHERE id=1`); err != nil {
		t.Fatal(err)
	}

	published := &query.IsAvatarPublished{Key: "avatars/inactive"}
	dispatchModeration(t, ctx, published)

	if published.Result {
		t.Fatal("inactive avatar key is public")
	}

	change := &cmd.SaveProfileAvatar{UserID: 1, AvatarType: enum.AvatarTypeCustom, BlobKey: "avatars/inactive", Review: true}
	dispatchModeration(t, ctx, change)

	if !change.Pending {
		t.Fatal("inactive avatar skipped review")
	}

	dispatchModeration(t, ctx, published)

	if published.Result {
		t.Fatal("pending avatar is public")
	}

	check := takeModeration(t)
	dispatchModeration(t, ctx, &cmd.FinishModeration{Check: check, Outcome: cmd.ModerationReviewed})
	dispatchModeration(t, ctx, published)

	if !published.Result {
		t.Fatal("approved avatar did not become public")
	}

	dispatchModeration(t, ctx, change)

	if change.Pending {
		t.Fatal("unchanged approved avatar unnecessarily queued again")
	}
}

func TestModerationAccountDeletion(t *testing.T) {
	for _, state := range []string{"pending", "running", "failed", "rejected", "complete", "canceled"} {
		t.Run(state, func(t *testing.T) {
			ctx := moderationDatabase(t)
			bus.Init(blobsql.Service{})
			var id int

			if err := dbx.Connection().QueryRow(`INSERT INTO users(name,email,created_at,tenant_id,role,status,avatar_type,avatar_bkey)
                VALUES ('Public name','',NOW(),1,1,1,1,'') RETURNING id`).Scan(&id); err != nil {
				t.Fatal(err)
			}

			key := fmt.Sprintf("avatars/deletion-%d", id)
			t.Cleanup(func() {
				dbx.Connection().Exec(`DELETE FROM moderation_checks WHERE tenant_id=1 AND content_id=$1 AND content_type IN ('name','avatar')`, id)
				dbx.Connection().Exec(`DELETE FROM blobs WHERE tenant_id=1 AND key=$1`, key)
				dbx.Connection().Exec(`DELETE FROM users WHERE id=$1`, id)
			})

			ctx = context.WithValue(ctx, app.UserCtxKey, &entity.User{ID: id})
			dispatchModeration(t, ctx, &cmd.StoreBlob{Key: key, Content: []byte("fixture"), ContentType: "image/png"})
			dispatchModeration(t, ctx, &cmd.SaveProfileName{UserID: id, Name: "Private submission", Review: true})
			nameClaim := takeModeration(t)
			dispatchModeration(t, ctx, &cmd.SaveProfileAvatar{UserID: id, AvatarType: enum.AvatarTypeCustom, BlobKey: key, Review: true})
			avatarClaim := takeModeration(t)

			if _, err := dbx.Connection().Exec(`UPDATE moderation_checks SET state=$1 WHERE content_id=$2`, state, id); err != nil {
				t.Fatal(err)
			}

			dispatchModeration(t, ctx, &cmd.DeleteCurrentUser{})

			for _, claim := range []cmd.ModerationCheck{nameClaim, avatarClaim} {
				finish := &cmd.FinishModeration{Check: claim, Outcome: cmd.ModerationReviewed}
				dispatchModeration(t, ctx, finish)

				if finish.Applied {
					t.Fatal("in-flight completion published after deletion")
				}
			}

			var count int

			if err := dbx.Connection().QueryRow(`SELECT count(*) FROM moderation_checks WHERE tenant_id=1 AND content_id=$1 AND content_type IN ('name','avatar')`, id).Scan(&count); err != nil {
				t.Fatal(err)
			}

			if count != 0 {
				t.Errorf("deleted account retained %d profile checks in state %s", count, state)
			}

			prunable := &query.GetPrunableFiles{}
			dispatchModeration(t, ctx, prunable)
			found := false

			for _, candidate := range prunable.Result {
				found = found || candidate == key
			}

			if !found {
				t.Error("deleted account's proposal still prevents avatar pruning")
			}

			for _, change := range []any{
				&cmd.SaveProfileName{UserID: id, Name: "Late submission", Review: true},
				&cmd.SaveProfileAvatar{UserID: id, AvatarType: enum.AvatarTypeCustom, BlobKey: key, Review: true},
			} {
				if err := bus.Dispatch(ctx, change); err == nil {
					t.Errorf("late %T succeeded after account deletion", change)
				}
			}
		})
	}
}

func TestModerationReportIncludesFindings(t *testing.T) {
	assets.FS = os.DirFS(env.Path("."))
	ctx := moderationDatabase(t)
	env.Config.OpenAI.SexualThreshold = 0.8
	env.Config.OpenAI.SelfHarmThreshold = 0.8
	id := createModerationPost(t, ctx)
	bus.AddHandler(func(ctx context.Context, c *cmd.HTTPRequest) error {
		c.ResponseStatusCode = 200
		c.ResponseBody = []byte(`{"results":[{"category_scores":{"sexual":0.95,"sexual/minors":0.01,"self-harm":0.91,"self-harm/intent":0.02,"self-harm/instructions":0.03}}]}`)
		return nil
	})

	if worked, err := moderation.ProcessNext(context.Background()); err != nil || !worked {
		t.Fatalf("worked=%v error=%v", worked, err)
	}

	var reportID int

	if err := dbx.Connection().QueryRow(`SELECT id FROM reports WHERE reported_type='post' AND reported_id=$1`, id).Scan(&reportID); err != nil {
		t.Fatal(err)
	}

	get := &query.GetReportByID{ReportID: reportID}
	dispatchModeration(t, ctx, get)
	code, response := mock.NewServer().OnTenant(mock.DemoTenant).AsUser(mock.JonSnow).AddParam("id", reportID).Execute(handlers.GetReport())
	var report struct {
		Reporter *entity.User `json:"reporter"`
		Details  string       `json:"details"`
	}

	if err := json.Unmarshal(response.Body.Bytes(), &report); err != nil {
		t.Fatalf("status=%d body=%s error=%v", code, response.Body.String(), err)
	}

	if code != 200 || report.Reporter != nil || report.Details != "Flagged categories: sexual (0.95), self-harm (0.91)" {
		t.Fatalf("staff report status=%d details=%q", code, report.Details)
	}

	userID := 1
	human := &cmd.CreateReport{ReportedType: enum.ReportTypePost, ReportedID: id, Reason: "Human report", Details: "User explanation", ReporterID: &userID}
	dispatchModeration(t, ctx, human)
	code, response = mock.NewServer().OnTenant(mock.DemoTenant).AsUser(mock.JonSnow).Execute(handlers.ListReports())
	var list struct {
		Reports []entity.Report `json:"reports"`
	}

	if err := json.Unmarshal(response.Body.Bytes(), &list); err != nil || code != 200 {
		t.Fatalf("report list status=%d error=%v", code, err)
	}

	foundAutomatic, foundHuman := false, false

	for _, item := range list.Reports {
		if item.ID == reportID {
			foundAutomatic = item.Reporter == nil && item.Details == report.Details
		}

		if item.ID == human.Result {
			foundHuman = item.Reporter != nil && item.Reporter.ID == userID && item.Details == "User explanation"
		}
	}

	if !foundAutomatic || !foundHuman {
		t.Fatalf("mixed report list lost a reporter or explanation: automatic=%v human=%v", foundAutomatic, foundHuman)
	}
}

func TestModerationDeletionTransaction(t *testing.T) {
	ctx := moderationDatabase(t)
	var id int

	if err := dbx.Connection().QueryRow(`INSERT INTO users(name,email,created_at,tenant_id,role,status,avatar_type,avatar_bkey)
        VALUES ('Original','',NOW(),1,1,1,1,'') RETURNING id`).Scan(&id); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		dbx.Connection().Exec(`DELETE FROM moderation_checks WHERE tenant_id=1 AND content_id=$1 AND content_type IN ('name','avatar')`, id)
		dbx.Connection().Exec(`DELETE FROM users WHERE id=$1`, id)
	})

	ctx = context.WithValue(ctx, app.UserCtxKey, &entity.User{ID: id})
	dispatchModeration(t, ctx, &cmd.SaveProfileName{UserID: id, Name: "Saved proposal", Review: true})
	claim := takeModeration(t)
	trx, err := dbx.BeginTx(ctx)

	if err != nil {
		t.Fatal(err)
	}

	defer trx.MustRollback()
	dispatchModeration(t, context.WithValue(ctx, app.TransactionCtxKey, trx), &cmd.DeleteCurrentUser{})
	trx.MustRollback()
	finish := &cmd.FinishModeration{Check: claim, Outcome: cmd.ModerationReviewed}
	dispatchModeration(t, ctx, finish)

	if !finish.Applied {
		t.Fatal("rolled-back deletion lost the saved proposal")
	}

	dispatchModeration(t, ctx, &cmd.SaveProfileName{UserID: id, Name: "Next proposal", Review: true})
	claim = takeModeration(t)
	trx, err = dbx.BeginTx(ctx)

	if err != nil {
		t.Fatal(err)
	}

	defer trx.MustRollback()
	dispatchModeration(t, context.WithValue(ctx, app.TransactionCtxKey, trx), &cmd.DeleteCurrentUser{})
	concurrentCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	finish = &cmd.FinishModeration{Check: claim, Outcome: cmd.ModerationReviewed}
	finished := make(chan error, 1)
	saved := make(chan error, 1)
	go func() { finished <- bus.Dispatch(concurrentCtx, finish) }()
	go func() {
		saved <- bus.Dispatch(concurrentCtx, &cmd.SaveProfileName{UserID: id, Name: "Late proposal", Review: true})
	}()

	if err := trx.Commit(); err != nil {
		t.Fatal(err)
	}

	if err := <-finished; err != nil || finish.Applied {
		t.Fatalf("completion after committed deletion: applied=%v error=%v", finish.Applied, err)
	}

	if err := <-saved; err == nil {
		t.Fatal("concurrent save recreated a deleted profile")
	}

	var name string
	var count int

	if err := dbx.Connection().QueryRow(`SELECT name,(SELECT count(*) FROM moderation_checks WHERE tenant_id=1 AND content_id=$1 AND content_type IN ('name','avatar')) FROM users WHERE id=$1`, id).Scan(&name, &count); err != nil {
		t.Fatal(err)
	}

	if name != "" || count != 0 {
		t.Fatalf("deleted profile: name=%q checks=%d", name, count)
	}
}

func TestModerationFailOpenRecoveryAndReplacement(t *testing.T) {
	ctx := moderationDatabase(t)
	bus.AddHandler(func(ctx context.Context, c *cmd.UserListUpdateUser) error { return nil })
	var original string

	if err := dbx.Connection().QueryRow(`SELECT name FROM users WHERE id=1`).Scan(&original); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { dbx.Connection().Exec(`UPDATE users SET name=$1 WHERE id=1`, original) })
	status := 200
	body := `{"results":[{"category_scores":{"sexual":null,"sexual/minors":0,"self-harm":0,"self-harm/intent":0,"self-harm/instructions":0}}]}`
	calls := 0
	bus.AddHandler(func(ctx context.Context, c *cmd.HTTPRequest) error {
		calls++
		c.ResponseStatusCode = status
		c.ResponseBody = []byte(body)
		return nil
	})

	run := func() {
		t.Helper()

		if worked, err := moderation.ProcessNext(context.Background()); err != nil || !worked {
			t.Fatalf("worked=%v err=%v", worked, err)
		}
	}

	dispatchModeration(t, ctx, &cmd.SaveProfileName{UserID: 1, Name: "Visible during outage", Review: true})
	run()
	var actual, state string

	if err := dbx.Connection().QueryRow(`SELECT name,state FROM users JOIN moderation_checks ON content_id=users.id WHERE users.id=1 AND content_type='name'`).Scan(&actual, &state); err != nil {
		t.Fatal(err)
	}

	if actual != "Visible during outage" || state != "pending" {
		t.Fatalf("publication=%q check=%s", actual, state)
	}

	// Saving the already visible value must not silently mark it reviewed.
	dispatchModeration(t, ctx, &cmd.SaveProfileName{UserID: 1, Name: actual, Review: true})

	if err := dbx.Connection().QueryRow(`SELECT state FROM moderation_checks WHERE content_type='name'`).Scan(&state); err != nil {
		t.Fatal(err)
	}

	if state != "pending" {
		t.Fatalf("resave abandoned check: %s", state)
	}

	if _, err := dbx.Connection().Exec(`UPDATE moderation_provider SET available_at=NOW()+INTERVAL '1 hour'`); err != nil {
		t.Fatal(err)
	}

	dispatchModeration(t, ctx, &cmd.SaveProfileName{UserID: 1, Name: "Replacement during cooldown", Review: true})
	run()

	if calls != 1 {
		t.Fatalf("provider called during cooldown: %d", calls)
	}

	if err := dbx.Connection().QueryRow(`SELECT name FROM users WHERE id=1`).Scan(&actual); err != nil {
		t.Fatal(err)
	}

	if actual != "Replacement during cooldown" {
		t.Fatalf("cooldown blocked publication: %s", actual)
	}

	// A late rejection restores the value preceding the outage, not an unchecked intermediate edit.
	body = strings.Replace(body, "null", "1", 1)

	if _, err := dbx.Connection().Exec(`UPDATE moderation_checks SET next_attempt_at=NOW();UPDATE moderation_provider SET available_at=NOW()`); err != nil {
		t.Fatal(err)
	}

	run()

	if err := dbx.Connection().QueryRow(`SELECT name,state FROM users JOIN moderation_checks ON content_id=users.id WHERE users.id=1 AND content_type='name'`).Scan(&actual, &state); err != nil {
		t.Fatal(err)
	}

	if actual != original || state != "rejected" {
		t.Fatalf("rejection: public=%q state=%s", actual, state)
	}
}

func TestModerationCleanRecheckPreservesStaffHide(t *testing.T) {
	for _, kind := range []string{"post", "comment"} {
		t.Run(kind, func(t *testing.T) {
			ctx := moderationDatabase(t)
			postID := createModerationPost(t, ctx)
			contentID := postID
			table := "posts"

			if kind == "comment" {
				dispatchModeration(t, ctx, &cmd.FinishModeration{
					Check:   takeModeration(t),
					Outcome: cmd.ModerationReviewed,
				})

				err := dbx.Connection().QueryRow(`
					INSERT INTO comments (content, post_id, user_id, created_at, tenant_id)
					VALUES ('Comment to review', $1, 1, NOW(), 1)
					RETURNING id`, postID).Scan(&contentID)

				if err != nil {
					t.Fatal(err)
				}

				table = "comments"
				t.Cleanup(func() {
					dbx.Connection().Exec(`DELETE FROM reports WHERE reported_type='comment' AND reported_id=$1`, contentID)
					dbx.Connection().Exec(`DELETE FROM comments WHERE id=$1`, contentID)
				})

				dispatchModeration(t, ctx, &cmd.ScheduleModeration{
					ContentType: kind,
					ContentID:   contentID,
				})
			}

			for _, staffHidden := range []bool{false, true} {
				if staffHidden {
					dispatchModeration(t, ctx, &cmd.SetModerationPending{
						ContentType: kind,
						ContentID:   contentID,
						Pending:     true,
					})

					dispatchModeration(t, ctx, &cmd.ScheduleModeration{
						ContentType: kind,
						ContentID:   contentID,
					})
				}

				dispatchModeration(t, ctx, &cmd.FinishModeration{
					Check:    takeModeration(t),
					Outcome:  cmd.ModerationReviewed,
					Findings: []cmd.ModerationFinding{{Category: "sexual", Score: 1}},
				})

				dispatchModeration(t, ctx, &cmd.ScheduleModeration{
					ContentType: kind,
					ContentID:   contentID,
				})

				dispatchModeration(t, ctx, &cmd.FinishModeration{
					Check:   takeModeration(t),
					Outcome: cmd.ModerationReviewed,
				})

				var hidden bool
				err := dbx.Connection().QueryRow(`SELECT moderation_pending FROM `+table+` WHERE id=$1`, contentID).Scan(&hidden)

				if err != nil {
					t.Fatal(err)
				}

				if hidden != staffHidden {
					t.Fatalf("staff hidden=%v: clean recheck left hidden=%v", staffHidden, hidden)
				}
			}
		})
	}
}

func TestModerationFailOpenAvatarFallbackLifecycle(t *testing.T) {
	ctx := moderationDatabase(t)
	bus.Init(blobsql.Service{})
	var originalKey string
	var originalType int

	if err := dbx.Connection().QueryRow(`SELECT avatar_bkey,avatar_type FROM users WHERE id=1`).Scan(&originalKey, &originalType); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		dbx.Connection().Exec(`UPDATE users SET avatar_bkey=$1,avatar_type=$2 WHERE id=1`, originalKey, originalType)
		dbx.Connection().Exec(`DELETE FROM blobs WHERE tenant_id=1 AND key LIKE 'avatars/failopen-%'`)
	})

	for _, key := range []string{"avatars/failopen-old", "avatars/failopen-new", "avatars/failopen-renamed"} {
		dispatchModeration(t, ctx, &cmd.StoreBlob{Key: key, Content: []byte("image"), ContentType: "image/png"})
	}

	dispatchModeration(t, ctx, &cmd.SaveProfileAvatar{UserID: 1, AvatarType: enum.AvatarTypeCustom, BlobKey: "avatars/failopen-old"})
	dispatchModeration(t, ctx, &cmd.SaveProfileAvatar{UserID: 1, AvatarType: enum.AvatarTypeCustom, BlobKey: "avatars/failopen-new", Review: true})
	first := takeModeration(t)
	dispatchModeration(t, ctx, &cmd.FinishModeration{Check: first, Outcome: cmd.ModerationRetry})
	prune := &query.GetPrunableFiles{}
	dispatchModeration(t, ctx, prune)

	for _, key := range prune.Result {
		if key == "avatars/failopen-old" || key == "avatars/failopen-new" {
			t.Fatalf("pruned needed avatar %s", key)
		}
	}

	dispatchModeration(t, ctx, &cmd.UpdateImageFileReferences{OldBlobKey: "avatars/failopen-old", NewBlobKey: "avatars/failopen-renamed"})
	second := takeModeration(t)
	dispatchModeration(t, ctx, &cmd.FinishModeration{Check: second, Outcome: cmd.ModerationReviewed, Findings: []cmd.ModerationFinding{{Category: "sexual", Score: 1}}})
	var key string

	if err := dbx.Connection().QueryRow(`SELECT avatar_bkey FROM users WHERE id=1`).Scan(&key); err != nil {
		t.Fatal(err)
	}

	if key != "avatars/failopen-renamed" {
		t.Fatalf("fallback did not follow rename: %s", key)
	}

	dispatchModeration(t, ctx, &cmd.SaveProfileAvatar{
		UserID:     1,
		AvatarType: enum.AvatarTypeCustom,
		BlobKey:    "avatars/failopen-new",
		Review:     true,
	})

	dispatchModeration(t, ctx, &cmd.FinishModeration{
		Check:   takeModeration(t),
		Outcome: cmd.ModerationRetry,
	})

	dispatchModeration(t, ctx, &cmd.DeleteImageFileReferences{
		BlobKey: "avatars/failopen-renamed",
	})

	dispatchModeration(t, ctx, &cmd.FinishModeration{
		Check:    takeModeration(t),
		Outcome:  cmd.ModerationReviewed,
		Findings: []cmd.ModerationFinding{{Category: "sexual", Score: 1}},
	})

	var avatarType enum.AvatarType
	err := dbx.Connection().QueryRow(`
        SELECT avatar_bkey, avatar_type
        FROM users
        WHERE id = 1`).Scan(&key, &avatarType)

	if err != nil {
		t.Fatal(err)
	}

	if key != "" || avatarType != enum.AvatarTypeLetter {
		t.Fatalf("rejection restored a deleted avatar: key=%q type=%d", key, avatarType)
	}
}
