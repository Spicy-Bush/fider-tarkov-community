package postgres_test

import (
	"context"
	"slices"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
	"github.com/lib/pq"
)

func seedAttachmentKeys(t testing.TB, f postWorkflow, postID int, keys []string) {
	t.Helper()
	_, err := mediaFixtureSQL(`
        INSERT INTO attachments (tenant_id, post_id, user_id, attachment_bkey)
        SELECT $1, $2, $3, key FROM unnest($4::text[]) AS key
    `, f.tenant.ID, postID, f.user.ID, pq.Array(keys))
	if err != nil {
		t.Fatal(err)
	}
}

func TestAttachmentChanges(t *testing.T) {
	tests := []struct {
		name    string
		changes []*dto.ImageUpload
		want    []string
		invalid bool
	}{
		{
			name:    "retained references preserve stored files",
			changes: []*dto.ImageUpload{{BlobKey: "a"}, {BlobKey: "b"}, {BlobKey: "a"}},
			want:    []string{"a", "b"},
		},
		{
			name:    "unknown references cannot attach a file",
			changes: []*dto.ImageUpload{{BlobKey: "foreign"}},
			want:    []string{"a", "b"},
			invalid: true,
		},
		{
			name:    "removing twice is harmless",
			changes: []*dto.ImageUpload{{BlobKey: "a", Remove: true}, {BlobKey: "a", Remove: true}},
			want:    []string{"b"},
		},
		{
			name:    "retained reference cannot reverse removal",
			changes: []*dto.ImageUpload{{BlobKey: "a", Remove: true}, {BlobKey: "a"}},
			want:    []string{"b"},
		},
		{
			name: "failed upload preserves stored attachments",
			changes: []*dto.ImageUpload{
				{BlobKey: "a", Remove: true},
				{BlobKey: "foreign", Upload: &dto.ImageUploadData{}},
			},
			want:    []string{"a", "b"},
			invalid: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newPostWorkflow(t)
			post := &cmd.AddNewPost{Title: "Attachment changes", Description: "Stored images"}
			if err := bus.Dispatch(f.ctx, post); err != nil {
				t.Fatal(err)
			}

			seedAttachmentKeys(t, f, post.Result.ID, []string{"a", "b"})

			err := bus.Dispatch(f.ctx, &cmd.UpdatePost{
				Post:        post.Result,
				Title:       post.Title,
				Description: post.Description,
				Attachments: test.changes,
			})
			if (err != nil) != test.invalid {
				t.Fatalf("error = %v, invalid = %v", err, test.invalid)
			}
			if test.invalid {
				if _, ok := err.(*validate.Result); !ok {
					t.Fatalf("invalid attachment should return validation failure: %T %v", err, err)
				}
			}

			stored := &query.GetPostAttachments{PostID: post.Result.ID}
			if err := bus.Dispatch(f.ctx, stored); err != nil {
				t.Fatal(err)
			}

			slices.Sort(stored.Result)
			if !slices.Equal(stored.Result, test.want) {
				t.Fatalf("attachments = %v, want %v", stored.Result, test.want)
			}

			foreign := context.WithValue(f.ctx, app.TenantCtxKey, &entity.Tenant{ID: 2})
			if err := bus.Dispatch(foreign, &cmd.UpdatePost{
				Post:        post.Result,
				Title:       "Other tenant change",
				Description: "Other tenant change",
				Attachments: []*dto.ImageUpload{
					{BlobKey: test.want[0]},
					{BlobKey: test.want[0], Remove: true},
				},
			}); errors.Cause(err) != app.ErrNotFound {
				t.Fatalf("foreign post update: want not found, got %v", err)
			}

			if err := bus.Dispatch(f.ctx, stored); err != nil {
				t.Fatal(err)
			}

			slices.Sort(stored.Result)
			if !slices.Equal(stored.Result, test.want) {
				t.Fatalf("another tenant changed stored attachments: %v", stored.Result)
			}

			if count := workflowCount(t, "SELECT COUNT(*) FROM posts WHERE id = $1 AND title = $2", post.Result.ID, post.Title); count != 1 {
				t.Fatal("another tenant changed the post")
			}
		})
	}
}
