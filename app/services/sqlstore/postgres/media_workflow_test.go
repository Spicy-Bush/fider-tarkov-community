package postgres_test

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/middlewares"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/blob"
)

func uploadMediaFixture(t testing.TB, ctx context.Context, id, name, prefix string) *dto.FileInfo {
	t.Helper()
	var content bytes.Buffer
	if err := png.Encode(&content, image.NewNRGBA(image.Rect(0, 0, 4, 7))); err != nil {
		t.Fatal(err)
	}

	kind := enum.FileUploadPrivate
	if prefix == "attachments/" {
		kind = enum.FileUploadPublic
	}
	upload := &cmd.UploadImageFile{
		SubmissionID: id, Name: name, Type: kind, Content: content.Bytes(),
	}
	if err := bus.Dispatch(ctx, upload); err != nil {
		t.Fatal(err)
	}
	return upload.Result
}

func TestFileBrowserUploadIdentityAndStableRename(t *testing.T) {
	f := newPostWorkflow(t)
	f.user.Role = enum.RoleAdministrator
	first := uploadMediaFixture(t, f.ctx, "one-upload", "Original name", "files/")
	if first.Width != 4 || first.Height != 7 || first.ContentType != "image/png" {
		t.Fatalf("incorrect metadata: %+v", first)
	}

	rename := &cmd.RenameImageFile{BlobKey: first.BlobKey, Name: "A readable new name"}
	if err := bus.Dispatch(f.ctx, rename); err != nil {
		t.Fatal(err)
	}
	replayed := uploadMediaFixture(t, f.ctx, "one-upload", "Original name", "files/")
	if replayed.BlobKey != first.BlobKey || replayed.Name != rename.Name || replayed.URL != first.URL {
		t.Fatalf("upload replay undid rename or changed identity: %+v", replayed)
	}

	var content bytes.Buffer
	if err := png.Encode(&content, image.NewNRGBA(image.Rect(0, 0, 4, 7))); err != nil {
		t.Fatal(err)
	}
	conflict := &cmd.UploadImageFile{SubmissionID: "one-upload", Name: "Different intent", Type: enum.FileUploadPrivate, Content: content.Bytes()}
	if err := bus.Dispatch(f.ctx, conflict); err == nil {
		t.Fatal("different upload reused a receipt")
	}

	var wait sync.WaitGroup
	errors := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			upload := &cmd.UploadImageFile{SubmissionID: "concurrent", Name: "Concurrent upload", Type: enum.FileUploadPrivate, Content: content.Bytes()}
			errors <- bus.Dispatch(f.ctx, upload)
		}()
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	if count := workflowCount(t, "SELECT COUNT(*) FROM command_receipts WHERE kind='media-upload' AND tenant_id=$1", f.tenant.ID); count != 2 {
		t.Fatalf("receipts=%d, expected two unique uploads", count)
	}
}

func TestFileBrowserFiltersPaginationAndScopes(t *testing.T) {
	f := newPostWorkflow(t)
	unused := uploadMediaFixture(t, f.ctx, "unused", "Alpha 100%_literal", "attachments/")
	draft := uploadMediaFixture(t, f.ctx, "draft", "Beta", "attachments/")
	active := uploadMediaFixture(t, f.ctx, "active", "Gamma", "attachments/")
	private := uploadMediaFixture(t, f.ctx, "private", "Delta", "files/")

	for _, test := range []struct {
		key    string
		status entity.PageStatus
		slug   string
	}{
		{draft.BlobKey, entity.PageStatusDraft, "draft-file"},
		{active.BlobKey, entity.PageStatusPublished, "published-file"},
	} {
		if err := bus.Dispatch(f.ctx, &cmd.CreatePage{
			Title: test.slug, Slug: test.slug, Content: "![image](/static/images/" + test.key + ")", Status: test.status,
			Visibility: entity.PageVisibilityPublic,
		}); err != nil {
			t.Fatal(err)
		}
	}

	for _, test := range []struct {
		name          string
		search        string
		kind          string
		usage         string
		includeDrafts bool
		keys          []string
	}{
		{"literal search", "%_", "all", "all", false, []string{unused.BlobKey}},
		{"file type", "", "files", "all", false, []string{private.BlobKey}},
		{"unused", "", "attachments", "unused", false, []string{unused.BlobKey}},
		{"draft cleanup", "", "attachments", "unused", true, []string{unused.BlobKey, draft.BlobKey}},
		{"used", "", "all", "used", false, []string{draft.BlobKey, active.BlobKey}},
	} {
		t.Run(test.name, func(t *testing.T) {
			q := query.NewListImageFiles()
			q.Search = test.search
			q.Type = test.kind
			q.Usage = test.usage
			q.IncludeDrafts = test.includeDrafts
			if err := bus.Dispatch(f.ctx, q); err != nil {
				t.Fatal(err)
			}

			keys := []string{}
			for _, file := range q.Result {
				keys = append(keys, file.BlobKey)
			}

			slices.Sort(keys)
			slices.Sort(test.keys)
			if !slices.Equal(keys, test.keys) || q.Total != len(test.keys) {
				t.Fatalf("keys=%v total=%d expected=%v", keys, q.Total, test.keys)
			}
		})
	}

	q := query.NewListImageFiles()
	q.Page = 999
	q.PageSize = 2
	q.SortBy = "size"
	q.SortDir = "asc"
	if err := bus.Dispatch(f.ctx, q); err != nil || q.Page != 2 || len(q.Result) != 2 {
		t.Fatalf("page clamp: %+v %v", q, err)
	}

	second := q.Result
	q.Page = 1
	if err := bus.Dispatch(f.ctx, q); err != nil {
		t.Fatal(err)
	}

	for _, file := range q.Result {
		if slices.ContainsFunc(second, func(other *dto.FileInfo) bool { return other.BlobKey == file.BlobKey }) {
			t.Fatal("tied sorting repeated a file across pages")
		}
	}

	prune := &cmd.PruneFiles{Type: "attachments", Before: time.Now(), IncludeDrafts: true}
	if err := bus.Dispatch(f.ctx, prune); err != nil {
		t.Fatal(err)
	}

	if len(prune.Result.Deleted) != 2 || slices.Contains(prune.Result.Deleted, active.BlobKey) {
		t.Fatalf("cleanup scopes: %+v", prune.Result)
	}

	var body string
	if err := dbx.Connection().QueryRow("SELECT content FROM pages WHERE slug='draft-file'").Scan(&body); err != nil || body == "" {
		t.Fatalf("cleanup erased retained draft text: %v %q", err, body)
	}
}

func TestFileBrowserPageOrder(t *testing.T) {
	f := newPostWorkflow(t)
	files := make([]*dto.FileInfo, 23)
	for index := range files {
		file := &dto.FileInfo{
			BlobKey:   fmt.Sprintf("files/order-%02d", index),
			Name:      []string{"Alpha", "beta", "Gamma"}[index%3],
			Size:      int64(index % 5),
			CreatedAt: time.Unix(int64(index%4), 0).UTC(),
		}
		_, err := dbx.Connection().Exec(`
			INSERT INTO media_assets (tenant_id,key,name,size,created_at,content_type)
			VALUES ($1,$2,$3,$4,$5,'image/png')
		`, f.tenant.ID, file.BlobKey, file.Name, file.Size, file.CreatedAt)
		if err != nil {
			t.Fatal(err)
		}
		files[index] = file
	}

	for _, field := range []string{"name", "size", "createdAt"} {
		for _, direction := range []string{"asc", "desc"} {
			t.Run(field+"/"+direction, func(t *testing.T) {
				expected := slices.Clone(files)
				slices.SortFunc(expected, func(left, right *dto.FileInfo) int {
					order := 0
					switch field {
					case "name":
						order = strings.Compare(strings.ToLower(left.Name), strings.ToLower(right.Name))
					case "size":
						order = cmp.Compare(left.Size, right.Size)
					case "createdAt":
						order = left.CreatedAt.Compare(right.CreatedAt)
					}
					if order == 0 {
						order = strings.Compare(left.BlobKey, right.BlobKey)
					}
					if direction == "desc" {
						order = -order
					}
					return order
				})

				var actual []*dto.FileInfo
				for page := 1; page <= 4; page++ {
					request := query.NewListImageFiles()
					request.Page = page
					request.PageSize = 7
					request.SortBy = field
					request.SortDir = direction
					if err := bus.Dispatch(f.ctx, request); err != nil {
						t.Fatal(err)
					}
					if request.Total != len(files) || request.TotalPages != 4 {
						t.Fatalf("page totals: %+v", request)
					}
					actual = append(actual, request.Result...)
				}

				if !slices.EqualFunc(actual, expected, func(left, right *dto.FileInfo) bool {
					return left.BlobKey == right.BlobKey
				}) {
					t.Fatal("page order omitted, repeated, or misplaced a file")
				}
			})
		}
	}
}

func TestFileBrowserDeletionIsolationAndRecovery(t *testing.T) {
	f := newPostWorkflow(t)
	file := uploadMediaFixture(t, f.ctx, "deleted-reference", "A deleted post image", "attachments/")
	post := &cmd.AddNewPost{Title: "Deleted content", Description: "![image](" + file.URL + ")"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}

	remove := &cmd.DeleteFiles{BlobKeys: []string{file.BlobKey}}
	if err := bus.Dispatch(f.ctx, remove); err != nil || len(remove.Result.Skipped) != 1 {
		t.Fatalf("active reference ignored: %+v %v", remove.Result, err)
	}

	if _, err := mediaFixtureSQL("UPDATE posts SET status=6 WHERE id=$1", post.Result.ID); err != nil {
		t.Fatal(err)
	}
	remove.IncludeDeleted = true

	for attempt := 0; attempt < 2; attempt++ {
		if err := bus.Dispatch(f.ctx, remove); err != nil || !slices.Equal(remove.Result.Deleted, []string{file.BlobKey}) {
			t.Fatalf("explicit deletion/replay %d: %+v %v", attempt, remove.Result, err)
		}
	}

	if count := workflowCount(t, "SELECT COUNT(*) FROM blobs WHERE tenant_id=$1 AND key=$2", f.tenant.ID, file.BlobKey); count != 0 {
		t.Fatal("physical image survived completed deletion")
	}

	foreign := context.WithValue(f.ctx, app.TenantCtxKey, &entity.Tenant{ID: 2})
	if err := bus.Dispatch(foreign, remove); err != nil || len(remove.Result.Errors) != 1 {
		t.Fatalf("foreign tenant accepted another tenant's receipt: %+v %v", remove.Result, err)
	}
}

func TestFileBrowserDeletionOwnsTransactions(t *testing.T) {
	f := newPostWorkflow(t)
	file := uploadMediaFixture(t, f.ctx, "transaction-ownership", "Still present", "files/")
	trx, err := dbx.BeginTx(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer trx.Rollback()

	ctx := context.WithValue(f.ctx, app.TransactionCtxKey, trx)
	deletion := &cmd.DeleteFiles{BlobKeys: []string{file.BlobKey}}
	if err := bus.Dispatch(ctx, deletion); err == nil {
		t.Fatal("deletion accepted an outer transaction")
	}
	prune := &cmd.PruneFiles{Type: "files", Before: time.Now()}
	if err := bus.Dispatch(ctx, prune); err == nil {
		t.Fatal("cleanup accepted an outer transaction")
	}
	if err := trx.Commit(); err != nil {
		t.Fatal(err)
	}

	if workflowCount(t, "SELECT COUNT(*) FROM media_assets WHERE tenant_id=$1 AND deletion_requested_at IS NULL", f.tenant.ID) != 1 {
		t.Fatal("rejected deletion changed the file")
	}
}

func TestFileBrowserHTTPInputsAndPermissions(t *testing.T) {
	f := newPostWorkflow(t)
	file := uploadMediaFixture(t, f.ctx, "permission-file", "A file", "files/")
	operations := []struct {
		name    string
		method  string
		path    string
		body    string
		status  int
		handler web.HandlerFunc
	}{
		{"list", http.MethodGet, "/api/admin/files", "", http.StatusOK, handlers.ListFiles()},
		{"usage", http.MethodGet, "/api/admin/files/usage?key=" + file.BlobKey, "", http.StatusOK, handlers.GetFileUsage()},
		{"download", http.MethodGet, "/api/admin/files/download?key=" + file.BlobKey, "", http.StatusOK, handlers.DownloadFile()},
		{"rename", http.MethodPut, "/api/admin/files/name", `{"blobKey":"` + file.BlobKey + `","name":"Renamed"}`, http.StatusOK, handlers.RenameFile()},
		{"delete", http.MethodPost, "/api/admin/files/delete", `{"blobKeys":[],"force":true}`, http.StatusBadRequest, handlers.BulkDeleteFiles()},
		{"prune", http.MethodPost, "/api/admin/files/prune", `{}`, http.StatusBadRequest, handlers.PruneUnusedFiles()},
	}

	for _, role := range []enum.Role{enum.RoleVisitor, enum.RoleHelper, enum.RoleModerator, enum.RoleCollaborator, enum.RoleAdministrator} {
		f.user.Role = role
		for _, operation := range operations {
			response, err := f.requestWithParams(middlewares.RequirePermission(entity.ManageFiles)(operation.handler), operation.method, operation.path, operation.body, nil)
			if err != nil {
				t.Fatal(err)
			}

			if role != enum.RoleAdministrator && response.Code != http.StatusForbidden {
				t.Fatalf("%s role=%s status=%d", operation.name, role, response.Code)
			}

			if role == enum.RoleAdministrator && response.Code != operation.status {
				t.Fatalf("%s status=%d expected=%d: %s", operation.name, response.Code, operation.status, response.Body)
			}
		}
	}

	activeUser := f.user
	blockedUser := *activeUser
	blockedUser.Status = enum.UserBlocked
	for _, test := range []struct {
		user   *entity.User
		status int
	}{
		{nil, http.StatusUnauthorized},
		{&blockedUser, http.StatusForbidden},
	} {
		f.user = test.user
		for _, operation := range operations {
			response, err := f.requestWithParams(middlewares.RequirePermission(entity.ManageFiles)(operation.handler), operation.method, operation.path, operation.body, nil)
			if err != nil || response.Code != test.status {
				t.Fatalf("%s status=%d expected=%d error=%v", operation.name, response.Code, test.status, err)
			}
		}
	}
	f.user = activeUser

	for _, parameters := range []string{"page=0", "page=-1", "page=x", "pageSize=0", "pageSize=-2", "pageSize=101", "type=nonsense", "sortBy=key"} {
		response, err := f.requestWithParams(handlers.ListFiles(), http.MethodGet, "/api/admin/files?"+parameters, "", nil)
		if err != nil || response.Code != http.StatusBadRequest {
			t.Fatalf("invalid list input %q: %v %d %s", parameters, err, response.Code, response.Body)
		}

		var failure map[string]any
		if err := json.Unmarshal(response.Body.Bytes(), &failure); err != nil {
			t.Fatal("validation response was not JSON")
		}
	}
}

func TestFileBrowserPruneReplaysAcceptedBatch(t *testing.T) {
	f := newPostWorkflow(t)
	_, err := dbx.Connection().Exec(`
		INSERT INTO media_assets (tenant_id,key,name,content_type,size)
		SELECT $1, 'files/batch-' || lpad(i::text, 3, '0'), 'Receipt batch', 'image/png', 1
		FROM generate_series(1,60) i
	`, f.tenant.ID)
	if err != nil {
		t.Fatal(err)
	}

	batch := &cmd.PruneFiles{Type: "files", Search: "Receipt batch", Before: time.Now()}
	if err := bus.Dispatch(f.ctx, batch); err != nil {
		t.Fatal(err)
	}
	accepted := slices.Clone(batch.Result.Deleted)
	cursor := batch.Result.NextCursor
	if len(accepted) != 50 || cursor == "" {
		t.Fatalf("first bounded batch: %+v", batch.Result)
	}

	if err := bus.Dispatch(f.ctx, batch); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(batch.Result.Deleted, accepted) || batch.Result.NextCursor != cursor {
		t.Fatalf("lost-response replay changed accepted batch: %+v", batch.Result)
	}

	lateImport := "files/batch-late"
	bus.AddHandler(func(ctx context.Context, scan *query.ScanBlobMetadata) error {
		files := []dto.BlobMetadata{{
			Key:         lateImport,
			ContentType: "image/png",
			Size:        1,
			ModifiedAt:  batch.Before.Add(-time.Hour),
		}}
		for number := 51; number <= 60; number++ {
			files = append(files, dto.BlobMetadata{
				Key:         fmt.Sprintf("files/batch-%03d", number),
				ContentType: "image/png",
				Size:        1,
				ModifiedAt:  batch.Before.Add(-time.Hour),
			})
		}

		return scan.Accept(files, "", true)
	})
	if err := bus.Dispatch(f.ctx, &cmd.ImportMediaInventory{}); err != nil {
		t.Fatal(err)
	}
	if _, err := dbx.Connection().Exec("UPDATE media_assets SET name='Receipt batch' WHERE tenant_id=$1 AND key=$2", f.tenant.ID, lateImport); err != nil {
		t.Fatal(err)
	}

	batch.Cursor = cursor
	if err := bus.Dispatch(f.ctx, batch); err != nil {
		t.Fatal(err)
	}
	if len(batch.Result.Deleted) != 10 || batch.Result.NextCursor != "" {
		t.Fatalf("remaining cleanup batch: %+v", batch.Result)
	}
	if count := workflowCount(t, "SELECT COUNT(*) FROM media_assets WHERE deleted_at IS NOT NULL"); count != 60 {
		t.Fatalf("completed cleanup deleted %d of 60 files", count)
	}
	if count := workflowCount(t, "SELECT COUNT(*) FROM media_assets WHERE tenant_id=$1 AND key=$2 AND deleted_at IS NULL", f.tenant.ID, lateImport); count != 1 {
		t.Fatal("cleanup deleted an image first discovered after review")
	}
}

func TestFileBrowserDelayedCleanupReplaysOnlyAcceptedKeys(t *testing.T) {
	f := newPostWorkflow(t)
	_, err := dbx.Connection().Exec(`
		INSERT INTO media_assets (tenant_id,key,name,content_type,size,created_at,cataloged_at)
		VALUES ($1,'files/accepted','Accepted image','image/png',1,NOW()-INTERVAL '10 days',NOW()-INTERVAL '10 days');
	`, f.tenant.ID)
	if err != nil {
		t.Fatal(err)
	}

	prune := &cmd.PruneFiles{Type: "files", Before: time.Now().Add(-8 * 24 * time.Hour)}
	if err := bus.Dispatch(f.ctx, prune); err != nil || !slices.Equal(prune.Result.Deleted, []string{"files/accepted"}) {
		t.Fatalf("delayed cleanup did not complete: result=%+v err=%v", prune.Result, err)
	}

	if err := bus.Dispatch(f.ctx, &cmd.RetryMediaDeletions{}); err != nil {
		t.Fatal(err)
	}
	if count := workflowCount(t, "SELECT COUNT(*) FROM command_receipts WHERE tenant_id=$1 AND kind='media-prune'", f.tenant.ID); count != 1 {
		t.Fatalf("cleanup lost its accepted receipt: %d", count)
	}

	_, err = dbx.Connection().Exec(`
		INSERT INTO media_assets (tenant_id,key,name,content_type,size,created_at,cataloged_at)
		VALUES ($1,'files/retained','Retained image','image/png',1,NOW()-INTERVAL '10 days',NOW()-INTERVAL '10 days')
	`, f.tenant.ID)
	if err != nil {
		t.Fatal(err)
	}

	if err := bus.Dispatch(f.ctx, prune); err != nil || !slices.Equal(prune.Result.Deleted, []string{"files/accepted"}) {
		t.Fatalf("replay selected different work: result=%+v err=%v", prune.Result, err)
	}
	if count := workflowCount(t, "SELECT COUNT(*) FROM media_assets WHERE tenant_id=$1 AND deletion_requested_at IS NULL", f.tenant.ID); count != 1 {
		t.Fatalf("replay changed a file outside its accepted batch: %d", count)
	}

	prune.Before = time.Now()
	if err := bus.Dispatch(f.ctx, prune); err != nil || len(prune.Result.Deleted) != 1 {
		t.Fatalf("a newly reviewed cleanup did not recover: result=%+v err=%v", prune.Result, err)
	}
}

func TestFileBrowserExternalDeletionResumesWithoutUserRetry(t *testing.T) {
	f := newPostWorkflow(t)
	first := uploadMediaFixture(t, f.ctx, "recover-delete", "Recover deletion", "files/")
	second := uploadMediaFixture(t, f.ctx, "healthy-delete", "Healthy deletion", "files/")

	previousStorage := env.Config.BlobStorage
	env.Config.BlobStorage.Type = "fs"
	env.Config.BlobStorage.FS.Path = t.TempDir()
	t.Cleanup(func() { env.Config.BlobStorage = previousStorage })
	if _, err := dbx.Connection().Exec("UPDATE media_assets SET storage_source=$1 WHERE tenant_id=$2", blob.StorageSource(), f.tenant.ID); err != nil {
		t.Fatal(err)
	}

	available := false
	removed := make(map[string]int)
	bus.AddHandler(func(ctx context.Context, request *cmd.DeleteBlob) error {
		if ctx.Value(app.TransactionCtxKey) != nil || dbx.Connection().Stats().InUse != 0 {
			t.Fatal("external storage waited with an open database connection")
		}
		if request.Key == first.BlobKey && !available {
			return errors.New("provider unavailable")
		}
		removed[request.Key]++
		return nil
	})

	deletion := &cmd.DeleteFiles{BlobKeys: []string{first.BlobKey, second.BlobKey}}
	if err := bus.Dispatch(f.ctx, deletion); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(deletion.Result.Pending, []string{first.BlobKey}) || !slices.Equal(deletion.Result.Deleted, []string{second.BlobKey}) {
		t.Fatalf("partial storage outcome lost: %+v", deletion.Result)
	}

	available = true
	if _, err := dbx.Connection().Exec("UPDATE media_assets SET next_deletion_attempt=NOW() WHERE key=$1", first.BlobKey); err != nil {
		t.Fatal(err)
	}
	if err := bus.Dispatch(context.Background(), &cmd.RetryMediaDeletions{}); err != nil {
		t.Fatal(err)
	}
	if removed[first.BlobKey] != 1 || removed[second.BlobKey] != 1 {
		t.Fatalf("pending work did not recover independently: %v", removed)
	}
	if count := workflowCount(t, "SELECT COUNT(*) FROM media_assets WHERE deleted_at IS NOT NULL"); count != 2 {
		t.Fatalf("recovered %d of two deletions", count)
	}
}
