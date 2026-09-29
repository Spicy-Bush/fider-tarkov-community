package postgres_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/api"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
)

func TestPostImageStorageDoesNotHoldWriteLocks(t *testing.T) {
	for _, operation := range []string{"create", "edit"} {
		t.Run(operation, func(t *testing.T) {
			f := newPostWorkflow(t)
			previous := env.Config.BlobStorage
			t.Cleanup(func() { env.Config.BlobStorage = previous })
			env.Config.BlobStorage.Type = "fs"
			env.Config.BlobStorage.FS.Path = t.TempDir()

			handler, method, number := api.CreatePost(), http.MethodPost, 0
			lockQuery := "SELECT id FROM tenants WHERE id = $1 FOR UPDATE NOWAIT"
			lockID := f.tenant.ID

			if operation == "edit" {
				post := &cmd.AddNewPost{Title: "An existing post for editing", Description: "An existing post"}
				if err := bus.Dispatch(f.ctx, post); err != nil {
					t.Fatal(err)
				}

				handler, method, number = api.UpdatePost(), http.MethodPut, post.Result.Number
				lockQuery = "SELECT id FROM posts WHERE id = $1 FOR UPDATE NOWAIT"
				lockID = post.Result.ID
			}

			storing := make(chan struct{})
			resume := make(chan struct{})
			bus.AddHandler(func(ctx context.Context, c *cmd.StoreBlob) error {
				close(storing)
				<-resume
				return nil
			})

			type response struct {
				status int
				err    error
			}
			completed := make(chan response, 1)
			body := submissionBody(t, "image-write-lock", true)
			go func() {
				recorder, err := f.request(handler, method, number, body)
				completed <- response{status: recorder.Code, err: err}
			}()

			select {
			case <-storing:
			case <-time.After(5 * time.Second):
				close(resume)
				t.Fatal("image request did not reach external blob storage")
			}

			inUse := dbx.Connection().Stats().InUse
			_, lockErr := dbx.Connection().Exec(lockQuery, lockID)
			close(resume)

			select {
			case result := <-completed:
				if result.err != nil || result.status != http.StatusOK {
					t.Fatalf("post write: status=%d error=%v", result.status, result.err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("post write did not finish after blob storage resumed")
			}

			if lockErr != nil {
				t.Errorf("image storage held the %s write lock: %v", operation, lockErr)
			}
			if inUse != 0 {
				t.Errorf("external image storage retained %d SQL connections", inUse)
			}

			for _, query := range []string{
				"SELECT COUNT(*) FROM posts",
				"SELECT COUNT(*) FROM attachments",
				"SELECT COUNT(*) FROM media_assets WHERE key LIKE 'attachments/%'",
			} {
				if count := workflowCount(t, query); count != 1 {
					t.Errorf("%s: want one committed effect, got %d", query, count)
				}
			}
		})
	}
}
