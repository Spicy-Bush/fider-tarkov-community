package postgres_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
)

func TestPreparedSubmissionReloadsAuthority(t *testing.T) {
	for _, kind := range []string{"post", "comment", "comment edit"} {
		for _, change := range []string{"blocked", "muted"} {
			t.Run(kind+"/"+change, func(t *testing.T) {
				f := newPostWorkflow(t)
				submission := f.imageSubmission(t, kind, "authority-after-upload")
				encoded, err := json.Marshal(submission.payload)
				if err != nil {
					t.Fatal(err)
				}

				useExternalImageStorage(t)
				writes := 0
				keys := map[string]bool{}
				bus.AddHandler(func(ctx context.Context, upload *cmd.StoreBlob) error {
					if dbx.Connection().Stats().InUse != 0 {
						t.Error("provider I/O retained a database connection")
					}
					writes++
					keys[upload.Key] = true
					if writes > 1 {
						return nil
					}
					if change == "blocked" {
						_, err := mediaFixtureSQL("UPDATE users SET status=$1 WHERE id=$2", enum.UserBlocked, f.user.ID)
						return err
					}
					_, err := mediaFixtureSQL(`INSERT INTO user_mutes(user_id,tenant_id,reason,created_at,created_by)
						VALUES($1,$2,'Changed during upload',NOW(),$1)`, f.user.ID, f.tenant.ID)
					return err
				})

				response, err := f.requestWithParams(submission.handler, http.MethodPost, "/api/preparation", string(encoded), submission.params)
				want := http.StatusForbidden
				if kind == "post" && change == "muted" {
					want = http.StatusBadRequest
				}
				if err != nil || response.Code != want {
					t.Fatalf("revoked submission: status=%d error=%v body=%s", response.Code, err, response.Body)
				}
				if count := workflowCount(t, "SELECT count(*) FROM attachments"); count != 0 {
					t.Fatalf("rejected submission retained %d attachments", count)
				}
				if count := workflowCount(t, "SELECT count(*) FROM posts"); count != 1 {
					t.Fatalf("rejected submission retained %d posts", count)
				}
				if count := workflowCount(t, "SELECT count(*) FROM comments WHERE content='Original comment'"); count != 1 {
					t.Fatal("rejected submission changed the original comment")
				}

				if _, err := mediaFixtureSQL("UPDATE users SET status=$1 WHERE id=$2", enum.UserActive, f.user.ID); err != nil {
					t.Fatal(err)
				}
				if _, err := mediaFixtureSQL("DELETE FROM user_mutes WHERE user_id=$1", f.user.ID); err != nil {
					t.Fatal(err)
				}
				response, err = f.requestWithParams(submission.handler, http.MethodPost, "/api/preparation", string(encoded), submission.params)
				if err != nil || response.Code != http.StatusOK || len(keys) != 1 || writes != 2 {
					t.Fatalf("healthy retry: status=%d error=%v keys=%d writes=%d body=%s", response.Code, err, len(keys), writes, response.Body)
				}
			})
		}
	}
}
