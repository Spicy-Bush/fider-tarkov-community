package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func reportFixture(t testing.TB, count int) (postWorkflow, []int, []*entity.User) {
	t.Helper()
	f := newPostWorkflow(t)
	post := &cmd.AddNewPost{Title: "Report submissions", Description: "Independent report targets"}
	otherPost := &cmd.AddNewPost{Title: "Other report discussion", Description: "Independent report targets"}
	reason := &cmd.CreateReportReason{Title: "Review reason", Description: "Report submission checks"}
	first := &query.GetUserByID{UserID: 2}
	second := &query.GetUserByID{UserID: 3}
	if err := bus.Dispatch(f.ctx, post, otherPost, reason, first, second); err != nil {
		t.Fatal(err)
	}

	rows, err := dbx.Connection().Query(`
        INSERT INTO comments (tenant_id, post_id, user_id, content, created_at)
        SELECT $1, CASE WHEN n % 2 = 1 THEN $2::integer ELSE $5::integer END, $3, 'Reported comment ' || n, NOW()
        FROM generate_series(1, $4) n RETURNING id
    `, f.tenant.ID, post.Result.ID, f.user.ID, count, otherPost.Result.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	ids := make([]int, 0, count)
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	return f, ids, []*entity.User{first.Result, second.Result}
}

type reportResponse struct {
	status int
	body   string
	err    error
}

func requestReport(f postWorkflow, reporter *entity.User, target enum.ReportType, id int, body string) reportResponse {
	handler := handlers.ReportComment()
	params := web.StringMap{"id": fmt.Sprint(id)}
	path := "/api/comments/" + params["id"] + "/report"
	if target == enum.ReportTypePost {
		handler = handlers.ReportPost()
		params = web.StringMap{"number": fmt.Sprint(id)}
		path = "/api/posts/" + params["number"] + "/report"
	}

	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response := httptest.NewRecorder()
	ctx, err := web.NewContext(f.engine, request, response, params)
	if err != nil {
		return reportResponse{err: err}
	}

	ctx.SetTenant(f.tenant)
	ctx.SetUser(reporter)

	err = handler(ctx)
	if err == nil {
		err = ctx.Commit()
	}

	return reportResponse{status: response.Code, body: response.Body.String(), err: err}
}

func (response reportResponse) expect(t testing.TB, status int) int {
	t.Helper()
	if response.err != nil || response.status != status {
		t.Fatalf("report: want status %d, got %+v", status, response)
	}

	if status != http.StatusOK {
		return 0
	}

	var receipt struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal([]byte(response.body), &receipt); err != nil || receipt.ID == 0 {
		t.Fatalf("invalid report acknowledgement: %+v", response)
	}

	return receipt.ID
}

func TestReportSubmissionConcurrency(t *testing.T) {
	cases := []struct {
		name              string
		sameTarget        bool
		firstPost         bool
		differentReporter bool
		limit             int
		accepted          int
		persisted         int
		rejection         string
	}{
		{name: "duplicate target", sameTarget: true, limit: 1, accepted: 2, persisted: 1},
		{name: "shared daily quota", limit: 1, accepted: 1, persisted: 1, rejection: "report limit"},
		{name: "independent targets", limit: 2, accepted: 2, persisted: 2},
		{name: "post and comment share quota", firstPost: true, limit: 1, accepted: 1, persisted: 1, rejection: "report limit"},
		{name: "unrelated reporters", differentReporter: true, limit: 1, accepted: 2, persisted: 2},
	}

	for _, candidate := range cases {
		t.Run(candidate.name, func(t *testing.T) {
			f, ids, users := reportFixture(t, 2)
			f.tenant.GeneralSettings.ReportLimitsPerDay = candidate.limit
			if candidate.sameTarget {
				ids[1] = ids[0]
			}
			if !candidate.differentReporter {
				users[1] = users[0]
			}

			owner := &query.GetDiscussion{CommentID: ids[0]}
			if err := bus.Dispatch(f.ctx, owner); err != nil {
				t.Fatal(err)
			}

			blocker, err := dbx.Connection().Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer blocker.Rollback()
			if _, err := blocker.Exec("LOCK TABLE reports IN SHARE MODE"); err != nil {
				t.Fatal(err)
			}

			completed := make(chan reportResponse, 2)
			for i := 0; i < 2; i++ {
				go func(index int) {
					target, id := enum.ReportTypeComment, ids[index]
					if candidate.firstPost && index == 0 {
						target, id = enum.ReportTypePost, owner.Result.Owner.Number
					}

					completed <- requestReport(f, users[index], target, id, `{"reason":"Review reason"}`)
				}(i)
			}

			deadline := time.Now().Add(5 * time.Second)
			for workflowCount(t, `
                SELECT COUNT(*) FROM pg_stat_activity
                WHERE datname = current_database() AND wait_event_type = 'Lock'
            `) < 2 {
				select {
				case result := <-completed:
					t.Fatalf("request returned before the insertion barrier: %+v", result)
				default:
				}
				if time.Now().After(deadline) {
					t.Fatal("both requests did not reach the controlled insertion boundary")
				}
				time.Sleep(10 * time.Millisecond)
			}

			if err := blocker.Commit(); err != nil {
				t.Fatal(err)
			}
			accepted := 0
			reportIDs := make(map[int]bool)
			for i := 0; i < 2; i++ {
				select {
				case result := <-completed:
					if result.status == http.StatusOK {
						accepted++
						reportIDs[result.expect(t, http.StatusOK)] = true
					} else {
						result.expect(t, http.StatusBadRequest)
						if !strings.Contains(strings.ToLower(result.body), candidate.rejection) {
							t.Fatalf("unexpected rejection: %+v", result)
						}
					}
				case <-time.After(5 * time.Second):
					t.Fatal("report did not finish after the database recovered")
				}
			}

			if accepted != candidate.accepted {
				t.Fatalf("accepted %d reports, want %d", accepted, candidate.accepted)
			}
			if len(reportIDs) != candidate.persisted {
				t.Fatalf("acknowledged %d distinct reports, want %d", len(reportIDs), candidate.persisted)
			}
			if count := workflowCount(t, "SELECT COUNT(*) FROM reports"); count != candidate.persisted {
				t.Fatalf("persisted %d reports, want %d", count, candidate.persisted)
			}
		})
	}
}

func BenchmarkReportSubmissionHandlers(b *testing.B) {
	for _, differentReporters := range []bool{false, true} {
		b.Run(fmt.Sprintf("different_reporters=%v", differentReporters), func(b *testing.B) {
			f, ids, users := reportFixture(b, 2*b.N)
			f.tenant.GeneralSettings.ReportLimitsPerDay = 2*b.N + 1
			if !differentReporters {
				users[1] = users[0]
			}
			b.ReportAllocs()
			b.ResetTimer()

			for iteration := 0; iteration < b.N; iteration++ {
				var group sync.WaitGroup
				var results [2]reportResponse
				for index := 0; index < 2; index++ {
					group.Add(1)
					go func(index int) {
						defer group.Done()
						results[index] = requestReport(f, users[index], enum.ReportTypeComment, ids[2*iteration+index], `{"reason":"Review reason"}`)
					}(index)
				}
				group.Wait()
				for _, result := range results {
					result.expect(b, http.StatusOK)
				}
			}

			b.StopTimer()
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(2*b.N), "ns/report")
			if count := workflowCount(b, "SELECT COUNT(*) FROM reports"); count != 2*b.N {
				b.Fatalf("persisted %d reports, want %d", count, 2*b.N)
			}
		})
	}
}

func TestReportSubmissionValidationAndRecovery(t *testing.T) {
	f, ids, users := reportFixture(t, 2)
	request := func(user *entity.User, body string, want int) {
		t.Helper()
		requestReport(f, user, enum.ReportTypeComment, ids[0], body).expect(t, want)
	}

	request(nil, `{"reason":"Review reason"}`, http.StatusForbidden)
	blocked := *users[0]
	blocked.Status = enum.UserBlocked
	request(&blocked, `{"reason":"Review reason"}`, http.StatusForbidden)
	request(f.user, `{"reason":"Review reason"}`, http.StatusForbidden)
	request(users[0], `{"reason":"Missing reason"}`, http.StatusBadRequest)
	oversized, err := json.Marshal(map[string]string{"reason": "Review reason", "details": strings.Repeat("x", 2001)})
	if err != nil {
		t.Fatal(err)
	}
	request(users[0], string(oversized), http.StatusBadRequest)

	f.tenant.GeneralSettings.ReportingGloballyDisabled = true
	request(users[0], `{"reason":"Review reason"}`, http.StatusBadRequest)
	f.tenant.GeneralSettings.ReportingGloballyDisabled = false
	if count := workflowCount(t, "SELECT COUNT(*) FROM reports"); count != 0 {
		t.Fatalf("rejected requests persisted %d reports", count)
	}

	request(users[0], `{"reason":"Review reason","reporterId":1,"reportedId":0,"reportedType":"post"}`, http.StatusOK)
	if count := workflowCount(t, "SELECT COUNT(*) FROM reports WHERE reporter_id = $1 AND reported_type = 'comment' AND reported_id = $2", users[0].ID, ids[0]); count != 1 {
		t.Fatal("request body replaced the authenticated actor or route target")
	}
	request(users[0], `{"reason":"Review reason","details":"Different report"}`, http.StatusBadRequest)
}

func TestReportSubmissionRecoversLostAcknowledgement(t *testing.T) {
	f, ids, users := reportFixture(t, 2)
	f.tenant.GeneralSettings.ReportLimitsPerDay = 1
	targetID := ids[0]
	post := func(body string, want int) int {
		t.Helper()
		return requestReport(f, users[0], enum.ReportTypeComment, targetID, body).expect(t, want)
	}

	body := `{"reason":"Review reason","details":"Exact report details"}`
	post(body, http.StatusOK)
	if count := f.engine.Worker().Length(); count != 2 {
		t.Fatalf("new report queued %d tasks, want 2", count)
	}

	var committedID int
	if err := dbx.Connection().QueryRow("SELECT id FROM reports").Scan(&committedID); err != nil {
		t.Fatal(err)
	}
	if recoveredID := post(body, http.StatusOK); recoveredID != committedID {
		t.Fatalf("replay acknowledged report %d, want committed report %d", recoveredID, committedID)
	}
	post(`{"reason":"Review reason","details":"Different details"}`, http.StatusBadRequest)
	post(`{"reason":"Different reason","details":"Exact report details"}`, http.StatusBadRequest)

	f.tenant.GeneralSettings.ReportingGloballyDisabled = true
	if _, err := dbx.Connection().Exec("UPDATE report_reasons SET is_active = FALSE WHERE tenant_id = $1", f.tenant.ID); err != nil {
		t.Fatal(err)
	}
	if recoveredID := post(body, http.StatusOK); recoveredID != committedID {
		t.Fatal("creation policy change prevented acknowledgement of an existing report")
	}
	if count := f.engine.Worker().Length(); count != 2 {
		t.Fatalf("replays queued additional notification tasks: %d", count)
	}
	if count := workflowCount(t, "SELECT COUNT(*) FROM reports"); count != 1 {
		t.Fatalf("replays persisted %d reports, want 1", count)
	}

	f.tenant.GeneralSettings.ReportingGloballyDisabled = false
	if _, err := dbx.Connection().Exec("UPDATE report_reasons SET is_active = TRUE WHERE tenant_id = $1", f.tenant.ID); err != nil {
		t.Fatal(err)
	}
	targetID = ids[1]
	post(body, http.StatusBadRequest)
}

func TestReportSubmissionRollbackReleasesAllowance(t *testing.T) {
	f, ids, users := reportFixture(t, 1)
	f.tenant.GeneralSettings.ReportLimitsPerDay = 1
	ctx := context.WithValue(f.ctx, app.UserCtxKey, users[0])
	failure := errors.New("fail after report insertion")

	err := dbx.InTransaction(ctx, func(ctx context.Context, trx *dbx.Trx) error {
		report := &cmd.CreateReport{
			ReportedType: enum.ReportTypeComment,
			ReportedID:   ids[0],
			Reason:       "Review reason",
		}
		if err := bus.Dispatch(ctx, report); err != nil {
			return err
		}
		if report.Result == 0 {
			t.Fatal("report did not reach insertion")
		}

		return failure
	})
	if err != failure {
		t.Fatalf("rollback lost the failure cause: %v", err)
	}
	if count := workflowCount(t, "SELECT COUNT(*) FROM reports"); count != 0 {
		t.Fatalf("rollback retained %d reports", count)
	}

	requestReport(f, users[0], enum.ReportTypeComment, ids[0], `{"reason":"Review reason"}`).expect(t, http.StatusOK)
	if count := workflowCount(t, "SELECT COUNT(*) FROM reports"); count != 1 {
		t.Fatalf("healthy retry persisted %d reports, want 1", count)
	}
}

func TestReportSubmissionRecoversAmongExistingDuplicates(t *testing.T) {
	f, ids, users := reportFixture(t, 1)
	f.tenant.GeneralSettings.ReportLimitsPerDay = 1
	if _, err := dbx.Connection().Exec(`
        INSERT INTO reports (tenant_id, reporter_id, reported_type, reported_id, reason, details, status, created_at)
        VALUES ($1, $2, 'comment', $3, 'Review reason', 'Older details', 'pending', NOW()),
               ($1, $2, 'comment', $3, 'Review reason', NULL, 'pending', NOW())
    `, f.tenant.ID, users[0].ID, ids[0]); err != nil {
		t.Fatal(err)
	}

	var expectedID int
	if err := dbx.Connection().QueryRow("SELECT id FROM reports WHERE details IS NULL").Scan(&expectedID); err != nil {
		t.Fatal(err)
	}
	response := requestReport(f, users[0], enum.ReportTypeComment, ids[0], `{"reason":"Review reason"}`)
	if recoveredID := response.expect(t, http.StatusOK); recoveredID != expectedID {
		t.Fatalf("recovered report %d, want matching report %d", recoveredID, expectedID)
	}
	if count := workflowCount(t, "SELECT COUNT(*) FROM reports"); count != 2 {
		t.Fatalf("recovery changed existing report count: %d", count)
	}
}
