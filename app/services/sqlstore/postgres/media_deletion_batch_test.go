package postgres_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/sqlstore/postgres"
)

func TestMediaUnusedDeletionWaitsForPublishingReferences(t *testing.T) {
	f := newPostWorkflow(t)
	image := uploadMediaFixture(t, f.ctx, "publication-race", "Published image", "attachments/")
	ctx, cancel := context.WithTimeout(f.ctx, 5*time.Second)
	defer cancel()

	writer, err := mediaFixtureTransaction(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Rollback()

	var processID int
	if err := writer.Scalar(&processID, "SELECT pg_backend_pid()"); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Execute("UPDATE users SET avatar_bkey=$1 WHERE id=$2", image.BlobKey, f.user.ID); err != nil {
		t.Fatal(err)
	}
	if err := postgres.CompleteMediaReferencesForTest(writer); err != nil {
		t.Fatal(err)
	}

	deletion := &cmd.DeleteFiles{BlobKeys: []string{image.BlobKey}}
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
			t.Fatalf("deletion did not wait for the publishing transaction: %v", err)
		case <-ctx.Done():
			t.Fatal("deletion never reached the image lock")
		case <-time.After(time.Millisecond):
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-completed; err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(deletion.Result.Skipped, []string{image.BlobKey}) || len(deletion.Result.Deleted) != 0 {
		t.Fatalf("published reference was not protected: %+v", deletion.Result)
	}
	if workflowCount(t, "SELECT COUNT(*) FROM blobs WHERE tenant_id=$1 AND key=$2", f.tenant.ID, image.BlobKey) != 1 {
		t.Fatal("published image was removed")
	}
}

func BenchmarkMediaUnusedBatchDeletion(b *testing.B) {
	f := newPostWorkflow(b)
	keys := make([]string, 50)
	for index := range keys {
		keys[index] = fmt.Sprintf("files/delete-batch-%d", index)
	}
	b.ReportAllocs()
	b.ResetTimer()

	for iteration := 0; iteration < b.N; iteration++ {
		b.StopTimer()
		_, err := dbx.Connection().Exec(`
			INSERT INTO media_assets(tenant_id,key,name,content_type,size)
			SELECT $1,'files/delete-batch-' || i,'Batch image','image/png',1 FROM generate_series(0,49) i
			ON CONFLICT(tenant_id,key) DO UPDATE SET deleted_at=NULL, deletion_requested_at=NULL;
		`, f.tenant.ID)
		if err != nil {
			b.Fatal(err)
		}
		_, err = dbx.Connection().Exec(`
			INSERT INTO blobs(tenant_id,key,size,content_type,file,created_at,modified_at)
			SELECT $1,'files/delete-batch-' || i,1,'image/png','x'::bytea,NOW(),NOW() FROM generate_series(0,49) i
		`, f.tenant.ID)
		if err != nil {
			b.Fatal(err)
		}
		b.StartTimer()

		request := &cmd.DeleteFiles{BlobKeys: keys}
		if err := bus.Dispatch(f.ctx, request); err != nil {
			b.Fatal(err)
		}
		if len(request.Result.Deleted) != len(keys) || len(request.Result.Pending)+len(request.Result.Errors)+len(request.Result.Skipped) != 0 {
			b.Fatalf("deletion changed the workload: %+v", request.Result)
		}
	}

	b.StopTimer()
	var remaining int
	if err := dbx.Connection().QueryRow("SELECT COUNT(*) FROM blobs WHERE tenant_id=$1", f.tenant.ID).Scan(&remaining); err != nil {
		b.Fatal(err)
	}
	if remaining != 0 {
		b.Fatalf("deletion retained %d blobs", remaining)
	}
}
