package postgres_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
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
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/pagedoc"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"github.com/lib/pq"
	"github.com/reearth/ygo/crdt"
)

func openPageEditing(t testing.TB, f postWorkflow) *entity.PageEditSession {
	t.Helper()
	created := &cmd.CreatePage{
		Title: "Published Page", Content: "Published body", Status: entity.PageStatusPublished,
		Visibility: entity.PageVisibilityPublic, AllowComments: true,
	}
	if err := bus.Dispatch(f.ctx, created); err != nil {
		t.Fatal(err)
	}
	opened := &cmd.OpenPageEdit{PageID: created.Result.ID}
	if err := bus.Dispatch(f.ctx, opened); err != nil {
		t.Fatal(err)
	}
	return opened.Result
}

func pageEditUpdate(t testing.TB, session *entity.PageEditSession, change func(*crdt.Transaction)) []byte {
	t.Helper()
	document := crdt.New()
	if err := crdt.ApplyUpdateV1(document, session.State, nil); err != nil {
		t.Fatal(err)
	}
	vector := document.StateVector()
	document.Transact(change)
	return crdt.EncodeStateAsUpdateV1(document, vector)
}

func syncPageEditingHTTP(t testing.TB, f postWorkflow, session *entity.PageEditSession, change func(*crdt.Transaction)) {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"update":      pageEditUpdate(t, session, change),
		"stateVector": session.StateVector,
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := f.requestWithParams(handlers.SyncPageEdit(), http.MethodPost,
		"/api/pages/draft", string(body), web.StringMap{"id": fmt.Sprint(session.PageID)})
	if err != nil || response.Code != http.StatusOK {
		t.Fatalf("synchronize Page: status=%d body=%s error=%v", response.Code, response.Body, err)
	}
}

func TestPageEditCreationRetriesAndDeletion(t *testing.T) {
	f := newPostWorkflow(t)
	const callers = 6
	results := make(chan *entity.PageEditSession, callers)
	failures := make(chan error, callers)
	var done sync.WaitGroup
	for range callers {
		done.Add(1)
		go func() {
			defer done.Done()
			open := &cmd.OpenPageEdit{SubmissionID: "new-shared-page"}
			if err := bus.Dispatch(f.ctx, open); err != nil {
				failures <- err
				return
			}
			results <- open.Result
		}()
	}
	done.Wait()
	close(results)
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}

	pageID := 0
	for result := range results {
		if pageID != 0 && result.PageID != pageID {
			t.Fatal("concurrent retries created different Pages")
		}
		pageID = result.PageID
		materialized, err := pagedoc.Read(result.State)
		if err != nil || materialized.Title != "" || materialized.Content != "" {
			t.Fatalf("new Page document=%+v error=%v", materialized, err)
		}
	}
	if pageID == 0 || workflowCount(t, "SELECT count(*) FROM pages WHERE tenant_id=1") != 1 {
		t.Fatal("creation did not converge on one Page")
	}
	if err := bus.Dispatch(f.ctx, &cmd.DeletePage{PageID: pageID}); err != nil {
		t.Fatal(err)
	}
	if err := bus.Dispatch(f.ctx, &cmd.OpenPageEdit{SubmissionID: "new-shared-page"}); err == nil {
		t.Fatal("late creation retry resurrected a deleted Page")
	}
	if count := workflowCount(t, "SELECT count(*) FROM pages WHERE tenant_id=1"); count != 0 {
		t.Fatalf("deleted Page count=%d", count)
	}
}

func TestPageEditConcurrentMergePublicationAndRecovery(t *testing.T) {
	f := newPostWorkflow(t)
	session := openPageEditing(t, f)
	if _, err := mediaFixtureSQL("UPDATE page_drafts SET draft_data=$1 WHERE shared", `{"content":"Obsolete shared snapshot"}`); err != nil {
		t.Fatal(err)
	}
	updates := [][]byte{
		pageEditUpdate(t, session, func(transaction *crdt.Transaction) {
			transaction.GetText("content").Insert(transaction, 0, "Coauthored ", nil)
		}),
		pageEditUpdate(t, session, func(transaction *crdt.Transaction) {
			transaction.GetText("title").Insert(transaction, 0, "Revised ", nil)
		}),
	}
	failures := make(chan error, len(updates))
	var done sync.WaitGroup
	for _, update := range updates {
		done.Add(1)
		go func(update []byte) {
			defer done.Done()
			failures <- bus.Dispatch(f.ctx, &cmd.SyncPageEdit{
				PageID: session.PageID, Update: update, StateVector: session.StateVector,
			})
		}(update)
	}
	done.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}

	current := &query.GetPageEdit{PageID: session.PageID}
	published := &query.GetPageByID{ID: session.PageID}
	if err := bus.Dispatch(f.ctx, current, published); err != nil {
		t.Fatal(err)
	}
	if published.Result.Content != "Published body" || published.Result.Title != "Published Page" {
		t.Fatal("editing changed the published Page before publication")
	}
	merged, err := pagedoc.Read(current.Result.State)
	if err != nil || merged.Content != "Coauthored Published body" || merged.Title != "Revised Published Page" {
		t.Fatalf("concurrent edits were lost: page=%+v error=%v", merged, err)
	}
	if workflowCount(t, "SELECT count(*) FROM page_drafts WHERE shared AND draft_data <> '{}'::jsonb") != 0 {
		t.Fatal("shared document retained an obsolete JSON copy")
	}

	beforeRetry := current.Result
	for _, update := range updates {
		if err := bus.Dispatch(f.ctx, &cmd.SyncPageEdit{PageID: session.PageID, Update: update}); err != nil {
			t.Fatal(err)
		}
	}
	if err := bus.Dispatch(f.ctx, current); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(beforeRetry.State, current.Result.State) || !beforeRetry.UpdatedAt.Equal(current.Result.UpdatedAt) {
		t.Fatal("lost-ACK retry changed the accepted document")
	}
	if err := bus.Dispatch(f.ctx, &cmd.SyncPageEdit{PageID: session.PageID, Update: []byte{255, 255}}); err == nil {
		t.Fatal("malformed update was accepted")
	}

	publish := &cmd.PublishPageEdit{PageID: session.PageID, Status: entity.PageStatusPublished, SubmissionID: "publish-original"}
	if err := bus.Dispatch(f.ctx, publish, current); err != nil {
		t.Fatal(err)
	}
	if publish.Result.Content != merged.Content || publish.Result.Title != merged.Title {
		t.Fatal("publication did not use the authoritative shared document")
	}
	if !bytes.Equal(beforeRetry.State, current.Result.State) {
		t.Fatal("publication removed another editor's working document")
	}
}

func TestPageEditFreshAuthorityAndTenantIsolation(t *testing.T) {
	f := newPostWorkflow(t)
	session := openPageEditing(t, f)
	update := pageEditUpdate(t, session, func(transaction *crdt.Transaction) {
		transaction.GetText("content").Insert(transaction, 0, "Authorized ", nil)
	})

	for _, role := range entity.PermissionRoles {
		t.Run(role.String(), func(t *testing.T) {
			if _, err := mediaFixtureSQL("UPDATE users SET role=$1 WHERE id=1", role); err != nil {
				t.Fatal(err)
			}
			wantAllowed := role == enum.RoleAdministrator || role == enum.RoleCollaborator
			access := &query.GetRealtimeAccess{Viewers: []query.RealtimeViewer{
				{TenantID: f.tenant.ID, UserID: f.user.ID, PageID: session.PageID},
				{TenantID: f.tenant.ID, UserID: f.user.ID, PageID: session.PageID + 10000},
				{TenantID: 2, UserID: 4, PageID: session.PageID},
			}}
			if err := bus.Dispatch(f.ctx, access); err != nil {
				t.Fatal(err)
			}
			if access.Result[0].Pages != wantAllowed || access.Result[1].Pages || access.Result[2].Pages {
				t.Fatalf("realtime Page policy=%+v; own Page allowed=%t", access.Result, wantAllowed)
			}
			for _, operation := range []any{
				&cmd.OpenPageEdit{PageID: session.PageID},
				&query.GetPageEdit{PageID: session.PageID},
				&cmd.SyncPageEdit{PageID: session.PageID, Update: update},
				&cmd.PublishPageEdit{PageID: session.PageID, Status: entity.PageStatusPublished, SubmissionID: "publish-original"},
				&cmd.UploadPageEditBanner{PageID: session.PageID, SubmissionID: "banner-" + role.String(), Image: pngAttachment(t, 2)},
			} {
				err := bus.Dispatch(f.ctx, operation)
				if (err == nil) != wantAllowed {
					t.Fatalf("%T allowed=%t want=%t error=%v", operation, err == nil, wantAllowed, err)
				}
			}
		})
	}
	if _, err := mediaFixtureSQL("UPDATE users SET role=$1 WHERE id=1", enum.RoleAdministrator); err != nil {
		t.Fatal(err)
	}

	otherUser := *f.user
	otherUser.ID = 4
	otherTenantContext := context.WithValue(f.ctx, app.UserCtxKey, &otherUser)
	for _, ctx := range []context.Context{context.WithValue(f.ctx, app.UserCtxKey, (*entity.User)(nil)), otherTenantContext} {
		if err := bus.Dispatch(ctx, &query.GetPageEdit{PageID: session.PageID}); err == nil {
			t.Fatal("another tenant's user or anonymous viewer read the document")
		}
	}
	if _, err := mediaFixtureSQL("UPDATE users SET status=$1 WHERE id=1", enum.UserBlocked); err != nil {
		t.Fatal(err)
	}
	if err := bus.Dispatch(f.ctx, &cmd.SyncPageEdit{PageID: session.PageID, Update: update}); err == nil {
		t.Fatal("stale authenticated context bypassed stored blocked status")
	}
}

func TestPageEditBannerOwnershipAndRemovalRecovery(t *testing.T) {
	f := newPostWorkflow(t)
	session := openPageEditing(t, f)
	upload := &cmd.UploadPageEditBanner{PageID: session.PageID, SubmissionID: "shared-banner", Image: pngAttachment(t, 2)}
	if err := bus.Dispatch(f.ctx, upload); err != nil {
		t.Fatal(err)
	}
	key := upload.Result
	if err := bus.Dispatch(f.ctx, upload); err != nil || upload.Result != key {
		t.Fatalf("banner retry changed identity: key=%s error=%v", upload.Result, err)
	}
	if _, err := mediaFixtureSQL("DELETE FROM command_receipts WHERE kind='media-upload'"); err != nil {
		t.Fatal(err)
	}

	collaborator := *f.user
	collaborator.ID = 2
	collaborator.Role = enum.RoleCollaborator
	if _, err := mediaFixtureSQL("UPDATE users SET role=$1 WHERE id=2", enum.RoleCollaborator); err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(f.ctx, app.UserCtxKey, &collaborator)
	update := pageEditUpdate(t, session, func(transaction *crdt.Transaction) {
		transaction.GetMap("settings").Set(transaction, "bannerImageBKey", key)
	})
	if err := bus.Dispatch(ctx, &cmd.SyncPageEdit{PageID: session.PageID, Update: update}); err != nil {
		t.Fatal(err)
	}
	read := &query.CanReadAttachment{Key: key}
	if err := bus.Dispatch(ctx, read); err != nil || !read.Result {
		t.Fatalf("coeditor cannot view the shared banner: allowed=%t error=%v", read.Result, err)
	}
	anonymous := context.WithValue(f.ctx, app.UserCtxKey, (*entity.User)(nil))
	if err := bus.Dispatch(anonymous, read); err != nil || read.Result {
		t.Fatalf("unpublished banner leaked: allowed=%t error=%v", read.Result, err)
	}

	if _, err := mediaFixtureSQL("SELECT unlink_media_reference_fields($1,$2,false,false,true)", f.tenant.ID, key); err != nil {
		t.Fatal(err)
	}
	var sharedBanner string
	err := dbx.Connection().QueryRow("SELECT banner_image_bkey FROM page_drafts WHERE page_id=$1 AND shared", session.PageID).Scan(&sharedBanner)
	if err != nil || sharedBanner != key {
		t.Fatalf("generic unlink changed the shared document's projection: key=%q error=%v", sharedBanner, err)
	}

	remove := &cmd.DeleteFiles{BlobKeys: []string{key}, IncludeDrafts: true}
	if err := bus.Dispatch(f.ctx, remove); err != nil || len(remove.Result.Deleted) != 1 {
		t.Fatalf("banner removal=%+v error=%v", remove.Result, err)
	}
	current := &query.GetPageEdit{PageID: session.PageID}
	if err := bus.Dispatch(ctx, current); err != nil {
		t.Fatal(err)
	}
	materialized, err := pagedoc.Read(current.Result.State)
	if err != nil || materialized.BannerImage.BlobKey != "" || materialized.Content != "Published body" {
		t.Fatalf("banner removal damaged working document: %+v error=%v", materialized, err)
	}

	textUpdate := pageEditUpdate(t, session, func(transaction *crdt.Transaction) {
		transaction.GetText("content").Insert(transaction, 0, "After deletion ", nil)
	})
	if err := bus.Dispatch(ctx, &cmd.SyncPageEdit{PageID: session.PageID, Update: textUpdate}); err != nil {
		t.Fatalf("an older editor could not recover after banner removal: %v", err)
	}
	if err := bus.Dispatch(ctx, &cmd.PublishPageEdit{PageID: session.PageID, Status: entity.PageStatusPublished, SubmissionID: "publish-original"}); err != nil {
		t.Fatal(err)
	}
	if err := bus.Dispatch(f.ctx, upload); err == nil {
		t.Fatal("deleted upload receipt recreated the banner")
	}
}

func TestPageEditReadOnlyOpenAndRetainedDraft(t *testing.T) {
	f := newPostWorkflow(t)
	created := &cmd.CreatePage{Title: "Original", Content: "Published body", Status: entity.PageStatusPublished, Visibility: entity.PageVisibilityPublic}
	if err := bus.Dispatch(f.ctx, created); err != nil {
		t.Fatal(err)
	}
	if err := bus.Dispatch(f.ctx, &query.GetPageEdit{PageID: created.Result.ID}); err == nil {
		t.Fatal("reading initialized a shared editing session")
	}
	if count := workflowCount(t, "SELECT count(*) FROM page_drafts"); count != 0 {
		t.Fatal("GET created a persisted draft")
	}
	for _, userID := range []int{1, 2} {
		if _, err := mediaFixtureSQL(`
			INSERT INTO page_drafts(tenant_id,page_id,user_id,title,slug,content)
			VALUES(1,$1,$2,$3,'old-slug','Retained text')
		`, created.Result.ID, userID, fmt.Sprintf("Draft by %d", userID)); err != nil {
			t.Fatal(err)
		}
	}
	opened := &cmd.OpenPageEdit{PageID: created.Result.ID}
	if err := bus.Dispatch(f.ctx, opened); err != nil {
		t.Fatal(err)
	}
	if len(opened.Result.LegacyDrafts) != 1 || opened.Result.LegacyDrafts[0].UserID != f.user.ID {
		t.Fatal("opening mixed another user's retained private draft")
	}
	if err := bus.Dispatch(f.ctx, &cmd.PublishPageEdit{PageID: created.Result.ID, Status: entity.PageStatusPublished, SubmissionID: "publish-original"}); err != nil {
		t.Fatal(err)
	}
	if count := workflowCount(t, "SELECT count(*) FROM page_drafts WHERE shared"); count != 1 {
		t.Fatal("publication deleted the shared document")
	}
	if count := workflowCount(t, "SELECT count(*) FROM page_drafts WHERE NOT shared AND user_id=2"); count != 1 {
		t.Fatal("publication deleted another user's retained draft")
	}

	blank := &cmd.OpenPageEdit{SubmissionID: "blank-publication"}
	if err := bus.Dispatch(f.ctx, blank); err != nil {
		t.Fatal(err)
	}
	err := bus.Dispatch(f.ctx, &cmd.PublishPageEdit{PageID: blank.Result.PageID, Status: entity.PageStatusPublished, SubmissionID: "publish-empty"})
	if _, valid := err.(*validate.Result); !valid {
		t.Fatalf("empty publication bypassed Page validation: %v", err)
	}
}

func TestPageEditUnsentDeletedBannerPreservesText(t *testing.T) {
	f := newPostWorkflow(t)
	session := openPageEditing(t, f)
	upload := &cmd.UploadPageEditBanner{PageID: session.PageID, SubmissionID: "offline-banner", Image: pngAttachment(t, 2)}
	if err := bus.Dispatch(f.ctx, upload); err != nil {
		t.Fatal(err)
	}
	if _, err := mediaFixtureSQL("DELETE FROM command_receipts WHERE kind='media-upload'"); err != nil {
		t.Fatal(err)
	}

	document := crdt.New()
	if err := crdt.ApplyUpdateV1(document, session.State, nil); err != nil {
		t.Fatal(err)
	}
	beforeEdit := document.StateVector()
	document.Transact(func(transaction *crdt.Transaction) {
		transaction.GetText("content").Insert(transaction, 0, "Offline edit ", nil)
		transaction.GetMap("settings").Set(transaction, "bannerImageBKey", upload.Result)
	})
	pending := &cmd.SyncPageEdit{
		PageID:      session.PageID,
		Update:      crdt.EncodeStateAsUpdateV1(document, beforeEdit),
		StateVector: crdt.EncodeStateVectorV1(document),
	}
	remove := &cmd.DeleteFiles{BlobKeys: []string{upload.Result}}
	if err := bus.Dispatch(f.ctx, remove); err != nil || len(remove.Result.Deleted) != 1 {
		t.Fatalf("remove unsaved upload: result=%+v error=%v", remove.Result, err)
	}
	if err := bus.Dispatch(f.ctx, pending); err != nil {
		t.Fatalf("deleted own upload blocked pending text: %v", err)
	}
	checkPeer := func(update []byte) {
		t.Helper()
		peer := crdt.New()
		if err := crdt.ApplyUpdateV1(peer, session.State, nil); err != nil {
			t.Fatal(err)
		}
		if err := crdt.ApplyUpdateV1(peer, update, nil); err != nil {
			t.Fatal(err)
		}
		page, err := pagedoc.Read(crdt.EncodeStateAsUpdateV1(peer, nil))
		if err != nil || page.Content != "Offline edit Published body" || page.BannerImage.BlobKey != "" {
			t.Fatalf("accepted delta did not correct a coeditor immediately: page=%+v error=%v", page, err)
		}
	}
	checkPeer(pending.AcceptedUpdate)
	if err := crdt.ApplyUpdateV1(document, pending.Result.Update, nil); err != nil {
		t.Fatal(err)
	}
	corrected, err := pagedoc.Read(crdt.EncodeStateAsUpdateV1(document, nil))
	if err != nil || corrected.Content != "Offline edit Published body" || corrected.BannerImage.BlobKey != "" {
		t.Fatalf("server correction did not preserve pending text: page=%+v error=%v", corrected, err)
	}
	acceptedAt := pending.Result.UpdatedAt
	if err := bus.Dispatch(f.ctx, pending); err != nil || !pending.Result.UpdatedAt.Equal(acceptedAt) {
		t.Fatalf("reconciliation retry changed accepted work: %v", err)
	}
	checkPeer(pending.AcceptedUpdate)

	other := openPageEditing(t, f)
	foreign := &cmd.UploadPageEditBanner{PageID: other.PageID, SubmissionID: "other-page-banner", Image: pngAttachment(t, 2)}
	if err := bus.Dispatch(f.ctx, foreign); err != nil {
		t.Fatal(err)
	}
	remove = &cmd.DeleteFiles{BlobKeys: []string{foreign.Result}}
	if err := bus.Dispatch(f.ctx, remove); err != nil || len(remove.Result.Deleted) != 1 {
		t.Fatalf("remove other upload: result=%+v error=%v", remove.Result, err)
	}
	current := &query.GetPageEdit{PageID: session.PageID}
	if err := bus.Dispatch(f.ctx, current); err != nil {
		t.Fatal(err)
	}
	accepted := current.Result
	for _, key := range []string{foreign.Result, "pages/unowned-image.webp"} {
		update := pageEditUpdate(t, accepted, func(transaction *crdt.Transaction) {
			transaction.GetText("content").Insert(transaction, 0, "Rejected edit ", nil)
			transaction.GetMap("settings").Set(transaction, "bannerImageBKey", key)
		})
		if err := bus.Dispatch(f.ctx, &cmd.SyncPageEdit{PageID: session.PageID, Update: update}); err == nil {
			t.Fatalf("unowned banner claim was silently accepted: %s", key)
		}
		if err := bus.Dispatch(f.ctx, current); err != nil || !bytes.Equal(accepted.State, current.Result.State) {
			t.Fatalf("rejected claim changed the shared Page: %v", err)
		}
	}
}

func TestPageEditBannerDeletionWaitsForPendingText(t *testing.T) {
	f := newPostWorkflow(t)
	session := openPageEditing(t, f)
	upload := &cmd.UploadPageEditBanner{PageID: session.PageID, SubmissionID: "concurrent-banner", Image: pngAttachment(t, 2)}
	if err := bus.Dispatch(f.ctx, upload); err != nil {
		t.Fatal(err)
	}
	update := pageEditUpdate(t, session, func(transaction *crdt.Transaction) {
		transaction.GetMap("settings").Set(transaction, "bannerImageBKey", upload.Result)
	})
	current := &query.GetPageEdit{PageID: session.PageID}
	if err := bus.Dispatch(f.ctx, &cmd.SyncPageEdit{PageID: session.PageID, Update: update}, current); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(f.ctx, 5*time.Second)
	defer cancel()
	writer, err := mediaFixtureTransaction(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Rollback()
	if _, err := writer.Execute("SELECT id FROM pages WHERE tenant_id=$1 AND id=$2 FOR UPDATE", f.tenant.ID, session.PageID); err != nil {
		t.Fatal(err)
	}
	var processID int
	if err := writer.Scalar(&processID, "SELECT pg_backend_pid()"); err != nil {
		t.Fatal(err)
	}

	deletion := &cmd.DeleteFiles{BlobKeys: []string{upload.Result}, IncludeDrafts: true}
	completed := make(chan error, 1)
	go func() {
		completed <- bus.Dispatch(ctx, deletion)
	}()
	for {
		var waiting bool
		err := dbx.Connection().QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid))
			)
		`, processID).Scan(&waiting)
		if err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}

		select {
		case err := <-completed:
			t.Fatalf("banner deletion did not wait for the editing transaction: %v", err)
		case <-ctx.Done():
			t.Fatal("banner deletion never reached the Page lock")
		case <-time.After(time.Millisecond):
		}
	}

	text := pageEditUpdate(t, current.Result, func(transaction *crdt.Transaction) {
		transaction.GetText("content").Insert(transaction, 0, "Pending edit ", nil)
	})
	writeContext := context.WithValue(ctx, app.TransactionCtxKey, writer)
	if err := bus.Dispatch(writeContext, &cmd.SyncPageEdit{PageID: session.PageID, Update: text}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-completed; err != nil || len(deletion.Result.Deleted) != 1 {
		t.Fatalf("banner deletion=%+v error=%v", deletion.Result, err)
	}
	if err := bus.Dispatch(f.ctx, current); err != nil {
		t.Fatal(err)
	}
	page, err := pagedoc.Read(current.Result.State)
	if err != nil || page.Content != "Pending edit Published body" || page.BannerImage.BlobKey != "" {
		t.Fatalf("banner deletion lost a committed edit: page=%+v error=%v", page, err)
	}
}

func TestPageEditNativeDraftListUsesWorkingTitle(t *testing.T) {
	f := newPostWorkflow(t)
	open := &cmd.OpenPageEdit{SubmissionID: "draft-list-title"}
	if err := bus.Dispatch(f.ctx, open); err != nil {
		t.Fatal(err)
	}
	update := pageEditUpdate(t, open.Result, func(transaction *crdt.Transaction) {
		transaction.GetText("title").Insert(transaction, 0, "Unique editing title", nil)
		transaction.GetText("content").Insert(transaction, 0, "Ready for publication", nil)
	})
	if err := bus.Dispatch(f.ctx, &cmd.SyncPageEdit{PageID: open.Result.PageID, Update: update}); err != nil {
		t.Fatal(err)
	}
	list := &query.ListPages{Status: []entity.PageStatus{entity.PageStatusDraft}, Query: "Unique editing", Limit: 20}
	if err := bus.Dispatch(f.ctx, list); err != nil || list.TotalCount != 1 || list.Result[0].Title != "Unique editing title" {
		t.Fatalf("draft listing=%+v error=%v", list, err)
	}
	publish := &cmd.PublishPageEdit{PageID: open.Result.PageID, Status: entity.PageStatusPublished, SubmissionID: "publish-original"}
	if err := bus.Dispatch(f.ctx, publish); err != nil {
		t.Fatal(err)
	}
	current := &query.GetPageEdit{PageID: open.Result.PageID}
	if err := bus.Dispatch(f.ctx, current); err != nil {
		t.Fatal(err)
	}
	materialized, err := pagedoc.Read(current.Result.State)
	if err != nil || materialized.Status != entity.PageStatusPublished || publish.Result.Status != entity.PageStatusPublished {
		t.Fatalf("publication state diverged: page=%+v error=%v", materialized, err)
	}
}

func TestPageEditExternalBannerRetryAndRevocation(t *testing.T) {
	for _, interruption := range []string{"provider response lost", "authority revoked during upload"} {
		t.Run(interruption, func(t *testing.T) {
			f := newPostWorkflow(t)
			session := openPageEditing(t, f)
			previous := env.Config.BlobStorage
			t.Cleanup(func() { env.Config.BlobStorage = previous })
			env.Config.BlobStorage.Type = "fs"
			env.Config.BlobStorage.FS.Path = t.TempDir()

			stored := map[string][]byte{}
			writes := 0
			bus.AddHandler(func(ctx context.Context, command *cmd.StoreBlob) error {
				if dbx.Connection().Stats().InUse != 0 {
					t.Error("banner preparation retained a database connection")
				}
				writes++
				stored[command.Key] = bytes.Clone(command.Content)
				if writes != 1 {
					return nil
				}
				if interruption == "provider response lost" {
					return fmt.Errorf("provider acknowledgement lost")
				}
				_, err := mediaFixtureSQL("UPDATE users SET status=$1 WHERE id=1", enum.UserBlocked)
				return err
			})

			upload := &cmd.UploadPageEditBanner{PageID: session.PageID, SubmissionID: "external-banner", Image: pngAttachment(t, 2)}
			if err := bus.Dispatch(f.ctx, upload); err == nil {
				t.Fatal("interrupted upload was reported successful")
			}
			if count := workflowCount(t, "SELECT count(*) FROM command_receipts WHERE kind='media-upload'"); count != 0 {
				t.Fatal("failed upload retained a completed receipt")
			}
			if _, err := mediaFixtureSQL("UPDATE users SET status=$1 WHERE id=1", enum.UserActive); err != nil {
				t.Fatal(err)
			}
			if err := bus.Dispatch(f.ctx, upload); err != nil {
				t.Fatal(err)
			}
			if len(stored) != 1 || writes != 2 {
				t.Fatalf("recovery allocated new external objects: keys=%d writes=%d", len(stored), writes)
			}
			acceptedKey := upload.Result
			accepted := bytes.Clone(stored[acceptedKey])
			if err := bus.Dispatch(f.ctx, upload); err != nil || writes != 2 {
				t.Fatalf("completed retry repeated provider I/O: writes=%d error=%v", writes, err)
			}

			upload.Image = pngAttachment(t, 3)
			if err := bus.Dispatch(f.ctx, upload); err == nil || writes != 2 || !bytes.Equal(stored[acceptedKey], accepted) {
				t.Fatal("changed payload overwrote an accepted banner identity")
			}
			if _, err := mediaFixtureSQL("UPDATE users SET status=$1 WHERE id=1", enum.UserBlocked); err != nil {
				t.Fatal(err)
			}
			upload.Image = pngAttachment(t, 2)
			if err := bus.Dispatch(f.ctx, upload); err == nil || writes != 2 {
				t.Fatal("completed receipt bypassed revoked authority")
			}
		})
	}
}

func TestPageEditPublishRetryDoesNotPublishLaterWork(t *testing.T) {
	f := newPostWorkflow(t)
	session := openPageEditing(t, f)
	first := &cmd.PublishPageEdit{
		PageID: session.PageID, Status: entity.PageStatusPublished, SubmissionID: "publish-once",
	}
	if err := bus.Dispatch(f.ctx, first); err != nil || first.Replayed {
		t.Fatalf("first publish failed: replay=%t error=%v", first.Replayed, err)
	}
	original := first.Result
	update := pageEditUpdate(t, session, func(transaction *crdt.Transaction) {
		transaction.GetText("content").Insert(transaction, 0, "Later coeditor work: ", nil)
	})
	if err := bus.Dispatch(f.ctx, &cmd.SyncPageEdit{PageID: session.PageID, Update: update}); err != nil {
		t.Fatal(err)
	}

	if err := bus.Dispatch(f.ctx, first); err != nil || !first.Replayed || first.Result.Content != original.Content {
		t.Fatalf("lost response did not recover original publication: replay=%t error=%v", first.Replayed, err)
	}
	published := &query.GetPageByID{ID: session.PageID}
	if err := bus.Dispatch(f.ctx, published); err != nil || published.Result.Content != original.Content {
		t.Fatalf("retry silently published another editor's work: %+v error=%v", published.Result, err)
	}
	second := &cmd.PublishPageEdit{
		PageID: session.PageID, Status: entity.PageStatusPublished, SubmissionID: "publish-later-work",
	}
	if err := bus.Dispatch(f.ctx, second); err != nil || second.Replayed || second.Result.Content == original.Content {
		t.Fatalf("explicit later publication failed: replay=%t error=%v", second.Replayed, err)
	}
	if err := bus.Dispatch(f.ctx, first, published); err != nil || published.Result.Content != second.Result.Content {
		t.Fatalf("old replay rolled back latest publication: %+v error=%v", published.Result, err)
	}
	first.Status = entity.PageStatusUnpublished
	if err := bus.Dispatch(f.ctx, first); err == nil {
		t.Fatal("reused publication ID changed its requested status")
	}
}

func TestPageEditScheduledPublicationPreservesWorkingEdits(t *testing.T) {
	for _, changeSchedule := range []bool{false, true} {
		t.Run(fmt.Sprint(changeSchedule), func(t *testing.T) {
			f := newPostWorkflow(t)
			due := time.Now().Add(-time.Minute).Truncate(time.Microsecond)
			created := &cmd.CreatePage{
				Title: "Scheduled Page", Content: "Scheduled body", Status: entity.PageStatusScheduled,
				Visibility: entity.PageVisibilityPublic, ScheduledFor: &due,
			}
			if err := bus.Dispatch(f.ctx, created); err != nil {
				t.Fatal(err)
			}
			opened := &cmd.OpenPageEdit{PageID: created.Result.ID}
			if err := bus.Dispatch(f.ctx, opened); err != nil {
				t.Fatal(err)
			}
			update := pageEditUpdate(t, opened.Result, func(transaction *crdt.Transaction) {
				transaction.GetText("content").Insert(transaction, 0, "Unpublished additions: ", nil)
				if changeSchedule {
					transaction.GetMap("settings").Set(transaction, "scheduledFor", due.Add(time.Hour).Format(time.RFC3339Nano))
				}
			})
			if err := bus.Dispatch(f.ctx, &cmd.SyncPageEdit{PageID: created.Result.ID, Update: update}); err != nil {
				t.Fatal(err)
			}
			publish := &cmd.PublishScheduledPages{}
			current := &query.GetPageEdit{PageID: created.Result.ID}
			published := &query.GetPageByID{ID: created.Result.ID}
			if err := bus.Dispatch(f.ctx, publish, current, published); err != nil {
				t.Fatal(err)
			}
			if publish.Result != 1 || published.Result.Content != "Scheduled body" || published.Result.Status != entity.PageStatusPublished {
				t.Fatal("scheduled publication changed unsaved shared content")
			}
			working, err := pagedoc.Read(current.Result.State)
			wantStatus := entity.PageStatusPublished
			if changeSchedule {
				wantStatus = entity.PageStatusScheduled
			}
			if err != nil || working.Content != "Unpublished additions: Scheduled body" || working.Status != wantStatus {
				t.Fatalf("scheduler lost working edits: page=%+v error=%v", working, err)
			}
		})
	}
}

func TestPageEditTextPublicationPreservesAuthorOrder(t *testing.T) {
	f := newPostWorkflow(t)
	if _, err := mediaFixtureSQL("UPDATE users SET role=$1 WHERE id=2", enum.RoleCollaborator); err != nil {
		t.Fatal(err)
	}
	created := &cmd.CreatePage{
		Title: "Ordered credits", Content: "Published body", Authors: []int{2, 1},
		Status: entity.PageStatusPublished, Visibility: entity.PageVisibilityPublic,
	}
	if err := bus.Dispatch(f.ctx, created); err != nil {
		t.Fatal(err)
	}
	opened := &cmd.OpenPageEdit{PageID: created.Result.ID}
	if err := bus.Dispatch(f.ctx, opened); err != nil {
		t.Fatal(err)
	}
	update := pageEditUpdate(t, opened.Result, func(transaction *crdt.Transaction) {
		transaction.GetText("content").Insert(transaction, 0, "Revised ", nil)
	})
	publish := &cmd.PublishPageEdit{
		PageID: created.Result.ID, Status: entity.PageStatusPublished, SubmissionID: "ordered-credits",
	}
	if err := bus.Dispatch(f.ctx, &cmd.SyncPageEdit{PageID: created.Result.ID, Update: update}, publish); err != nil {
		t.Fatal(err)
	}
	authors := make([]int, len(publish.Result.Authors))
	for index, author := range publish.Result.Authors {
		authors[index] = author.ID
	}
	if !slices.Equal(authors, []int{2, 1}) || publish.Result.Content != "Revised Published body" {
		t.Fatalf("text publication reordered author credits: authors=%v content=%q", authors, publish.Result.Content)
	}
}

func TestPageEditRetainsDeletedInlineImageReferences(t *testing.T) {
	for _, phase := range []string{"opening", "publishing"} {
		t.Run(phase, func(t *testing.T) {
			f := newPostWorkflow(t)
			image := uploadMediaFixture(t, f.ctx, "deleted-inline", "Inline image", "files/")
			content := "Preserved text. ![image](/static/images/" + image.BlobKey + ")"
			created := &cmd.CreatePage{
				Title: "Deleted inline image", Content: content,
				Status: entity.PageStatusPublished, Visibility: entity.PageVisibilityPublic,
			}
			if phase == "publishing" {
				created.Content = ""
			}
			if err := bus.Dispatch(f.ctx, created); err != nil {
				t.Fatal(err)
			}

			opened := &cmd.OpenPageEdit{PageID: created.Result.ID}
			if phase == "publishing" {
				if err := bus.Dispatch(f.ctx, opened); err != nil {
					t.Fatal(err)
				}
				update := pageEditUpdate(t, opened.Result, func(transaction *crdt.Transaction) {
					transaction.GetText("content").Insert(transaction, 0, content, nil)
				})
				if err := bus.Dispatch(f.ctx, &cmd.SyncPageEdit{PageID: created.Result.ID, Update: update}); err != nil {
					t.Fatal(err)
				}
			}
			remove := &cmd.DeleteFiles{BlobKeys: []string{image.BlobKey}, Force: true, IncludeDrafts: true}
			if err := bus.Dispatch(f.ctx, remove); err != nil || len(remove.Result.Deleted) != 1 {
				t.Fatalf("delete inline image: result=%+v error=%v", remove.Result, err)
			}
			if phase == "opening" {
				if err := bus.Dispatch(f.ctx, opened); err != nil {
					t.Fatalf("existing deleted image prevented opening the Page: %v", err)
				}
			} else {
				publish := &cmd.PublishPageEdit{
					PageID: created.Result.ID, Status: entity.PageStatusPublished, SubmissionID: "deleted-inline-publication",
				}
				if err := bus.Dispatch(f.ctx, publish); err != nil || publish.Result.Content != content {
					t.Fatalf("deleted inline image prevented text publication: %v", err)
				}
			}
			current := &query.GetPageEdit{PageID: created.Result.ID}
			if err := bus.Dispatch(f.ctx, current); err != nil {
				t.Fatal(err)
			}
			page, err := pagedoc.Read(current.Result.State)
			if err != nil || page.Content != content {
				t.Fatalf("deleted image damaged saved text: page=%+v error=%v", page, err)
			}
		})
	}
}

func TestPageEditCannotTransferUnrelatedDeletedReferences(t *testing.T) {
	f := newPostWorkflow(t)
	image := uploadMediaFixture(t, f.ctx, "retained-inline", "Retained image", "files/")
	unused := uploadMediaFixture(t, f.ctx, "unreferenced-inline", "Unused image", "files/")
	content := "![image](/static/images/" + image.BlobKey + ")"
	created := &cmd.CreatePage{
		Title: "Image owner", Content: content,
		Status: entity.PageStatusPublished, Visibility: entity.PageVisibilityPublic,
	}
	if err := bus.Dispatch(f.ctx, created); err != nil {
		t.Fatal(err)
	}
	remove := &cmd.DeleteFiles{BlobKeys: []string{image.BlobKey, unused.BlobKey}, Force: true}
	if err := bus.Dispatch(f.ctx, remove); err != nil || len(remove.Result.Deleted) != 2 {
		t.Fatalf("delete images: result=%+v error=%v", remove.Result, err)
	}
	opened := &cmd.OpenPageEdit{PageID: created.Result.ID}
	if err := bus.Dispatch(f.ctx, opened); err != nil {
		t.Fatal(err)
	}
	other := openPageEditing(t, f)
	for _, scenario := range []struct {
		name    string
		session *entity.PageEditSession
		field   string
		key     string
	}{
		{name: "another Page", session: other, field: "content", key: image.BlobKey},
		{name: "another field", session: opened.Result, field: "excerpt", key: image.BlobKey},
		{name: "another key", session: opened.Result, field: "content", key: unused.BlobKey},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			update := pageEditUpdate(t, scenario.session, func(transaction *crdt.Transaction) {
				transaction.GetText(scenario.field).Insert(transaction, 0, "![rejected](/static/images/"+scenario.key+")", nil)
			})
			err := bus.Dispatch(f.ctx, &cmd.SyncPageEdit{PageID: scenario.session.PageID, Update: update})
			var constraint *pq.Error
			if !errors.As(err, &constraint) || constraint.Code != "23503" {
				t.Fatalf("unrelated deleted reference bypassed storage constraint: %v", err)
			}
			current := &query.GetPageEdit{PageID: scenario.session.PageID}
			if err := bus.Dispatch(f.ctx, current); err != nil || !bytes.Equal(current.Result.State, scenario.session.State) {
				t.Fatalf("rejected transfer changed the Page: %v", err)
			}
		})
	}
}
