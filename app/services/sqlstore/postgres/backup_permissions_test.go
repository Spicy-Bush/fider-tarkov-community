package postgres_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/middlewares"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/backup"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/blob"
)

func TestBackupDiscoversTenantTablesAndReportsSkippedBlobs(t *testing.T) {
	f := newPostWorkflow(t)
	if _, err := dbx.Connection().Exec(`
		CREATE TABLE backup_discovery_fixture (tenant_id integer, value text);
		INSERT INTO backup_discovery_fixture VALUES (1, 'included'), (2, 'private');
		INSERT INTO blobs (tenant_id,key,content_type,size,file,created_at,modified_at)
		VALUES (1,'invalid stored name','text/plain',7,'fixture',NOW(),NOW());
	`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := dbx.Connection().Exec("DROP TABLE backup_discovery_fixture"); err != nil {
			t.Error(err)
		}
	})

	data, err := backup.Create(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(data.Bytes()), int64(data.Len()))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := archive.Open("backup_discovery_fixture.json")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var values []struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(rows).Decode(&values); err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0].Value != "included" {
		t.Fatalf("discovered table lost tenant isolation: %+v", values)
	}

	warning, err := archive.Open("warnings.json")
	if err != nil {
		t.Fatal(err)
	}
	defer warning.Close()
	var skipped struct {
		Count int `json:"skippedBlobs"`
	}
	if err := json.NewDecoder(warning).Decode(&skipped); err != nil || skipped.Count != 1 {
		t.Fatalf("skipped files were not reported: count=%d error=%v", skipped.Count, err)
	}
	for _, file := range archive.File {
		if slices.Contains([]string{"blobs.json", "media_thumbnails.json", "media_inventory.json"}, file.Name) {
			t.Errorf("backup included rebuildable storage metadata: %s", file.Name)
		}
	}
}

func TestBackupPreservesSharedPageDocument(t *testing.T) {
	f := newPostWorkflow(t)
	open := &cmd.OpenPageEdit{SubmissionID: "page-backup"}
	if err := bus.Dispatch(f.ctx, open); err != nil {
		t.Fatal(err)
	}

	data, err := backup.Create(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(data.Bytes()), int64(data.Len()))
	if err != nil {
		t.Fatal(err)
	}
	file, err := archive.Open("page_drafts.json")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	var drafts []struct {
		PageID int    `json:"page_id"`
		State  []byte `json:"collaborative_state"`
	}
	if err := json.NewDecoder(file).Decode(&drafts); err != nil {
		t.Fatal(err)
	}
	if len(drafts) != 1 || drafts[0].PageID != open.Result.PageID {
		t.Fatalf("exported the wrong shared Page drafts: %+v", drafts)
	}
	if !bytes.Equal(drafts[0].State, open.Result.State) {
		t.Fatal("backup changed the collaborative document bytes")
	}
}

func TestBackupIncludesTenantExportPresets(t *testing.T) {
	f := newPostWorkflow(t)
	_, err := dbx.Connection().Exec(`
		INSERT INTO feedback_export_presets (id, tenant_id, name, recipe, created_at, updated_at)
		VALUES ('11111111111111111111111111111111', 1, 'Weekly', '{}', NOW(), NOW()),
		       ('22222222222222222222222222222222', 2, 'Private', '{}', NOW(), NOW())
	`)
	if err != nil {
		t.Fatal(err)
	}

	data, err := backup.Create(f.ctx)
	if err != nil {
		t.Fatal(err)
	}

	archive, err := zip.NewReader(bytes.NewReader(data.Bytes()), int64(data.Len()))
	if err != nil {
		t.Fatal(err)
	}

	file, err := archive.Open("feedback_export_presets.json")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	var presets []struct {
		Name     string `json:"name"`
		TenantID int    `json:"tenant_id"`
	}
	if err := json.NewDecoder(file).Decode(&presets); err != nil {
		t.Fatal(err)
	}

	if len(presets) != 1 || presets[0].TenantID != 1 || presets[0].Name != "Weekly" {
		t.Fatalf("exported presets = %+v", presets)
	}
}

func TestAdministratorBackupIncludesProtectedBlobs(t *testing.T) {
	f := newPostWorkflow(t)
	keys := []string{"files/private.txt", "avatars/unpublished.png", "public.png"}
	for _, key := range keys {
		if err := bus.Dispatch(f.ctx, &cmd.StoreBlob{Key: key, Content: []byte(key), ContentType: "text/plain"}); err != nil {
			t.Fatal(err)
		}
	}

	other := context.WithValue(f.ctx, app.TenantCtxKey, &entity.Tenant{ID: 2})
	if err := bus.Dispatch(other, &cmd.StoreBlob{Key: "files/other.txt", Content: []byte("other tenant"), ContentType: "text/plain"}); err != nil {
		t.Fatal(err)
	}

	data, err := backup.Create(f.ctx)
	if err != nil {
		t.Fatal(err)
	}

	archive, err := zip.NewReader(bytes.NewReader(data.Bytes()), int64(data.Len()))
	if err != nil {
		t.Fatal(err)
	}

	for _, key := range keys {
		file, err := archive.Open("blobs/" + key)
		if err != nil {
			t.Fatal(err)
		}

		content, err := io.ReadAll(file)
		file.Close()
		if err != nil || string(content) != key {
			t.Fatalf("exported %s = %q: %v", key, content, err)
		}
	}

	if _, err := archive.Open("blobs/files/other.txt"); err == nil {
		t.Fatal("backup included another tenant's file")
	}

	f.tenant.RolePermissions = nil
	f.user.Role = enum.RoleHelper
	err = bus.Dispatch(f.ctx, &query.GetBlobByKey{Key: keys[0], ForBackup: true})
	if errors.Cause(err) != blob.ErrNotFound {
		t.Fatalf("revoked backup read was allowed: %v", err)
	}
}

func TestBackupCredentialsCannotBeDelegated(t *testing.T) {
	f := newPostWorkflow(t)
	const credential = "backup-permission-test-key"
	if _, err := mediaFixtureSQL("UPDATE users SET api_key = $1 WHERE id = 1", credential); err != nil {
		t.Fatal(err)
	}

	for _, role := range entity.PermissionRoles {
		t.Run(role.String(), func(t *testing.T) {
			f.user.Role = role
			f.tenant.RolePermissions = entity.RolePermissions{
				role: {entity.ExportBackup: true, entity.ExportFeedback: true},
			}

			response, err := f.requestWithParams(
				middlewares.RequirePermission(entity.ExportBackup)(handlers.ExportBackupZip()),
				http.MethodGet, "/admin/export/backup.zip", "", nil,
			)
			if err != nil {
				t.Fatal(err)
			}

			if role == enum.RoleAdministrator {
				if response.Code != http.StatusOK {
					t.Fatalf("administrator backup status = %d", response.Code)
				}

				archive, err := zip.NewReader(bytes.NewReader(response.Body.Bytes()), int64(response.Body.Len()))
				if err != nil {
					t.Fatal(err)
				}

				file, err := archive.Open("users.json")
				if err != nil {
					t.Fatal(err)
				}
				defer file.Close()

				data, err := io.ReadAll(file)
				if err != nil || !bytes.Contains(data, []byte(credential)) {
					t.Fatalf("administrator backup lost account credentials: %v", err)
				}
			} else {
				if response.Code != http.StatusForbidden {
					t.Fatalf("delegated backup status = %d", response.Code)
				}

				if data, err := backup.Create(f.ctx); err == nil || data != nil {
					t.Fatalf("direct backup operation bypassed permissions: %v", err)
				}

				err := bus.Dispatch(f.ctx, &query.GetBlobByKey{Key: "files/private", ForBackup: true})
				if errors.Cause(err) != blob.ErrNotFound {
					t.Fatalf("backup blob read bypassed permissions: %v", err)
				}
			}

			response, err = f.requestWithParams(
				middlewares.RequirePermission(entity.ExportFeedback)(handlers.ExportPostsToCSV()),
				http.MethodGet, "/admin/export/posts.csv", "", nil,
			)
			if err != nil || response.Code != http.StatusOK {
				t.Fatalf("feedback export status = %d: %v", response.Code, err)
			}
		})
	}
}
