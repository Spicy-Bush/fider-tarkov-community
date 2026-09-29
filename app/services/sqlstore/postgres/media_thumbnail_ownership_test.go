package postgres

import (
	"context"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
)

func TestMediaThumbnailReadPreservesCatalog(t *testing.T) {
	for _, state := range []string{"ready", "unknown", "missing", "different-provider"} {
		t.Run(state, func(t *testing.T) {
			ctx, key, original := thumbnailFixture(t, "files")
			var mutation string
			switch state {
			case "unknown":
				mutation = `UPDATE media_assets SET name=NULL, content_type=NULL, size=NULL, storage_source=NULL, cataloged_at=NULL WHERE tenant_id=1 AND key=$1`
			case "missing":
				mutation = `DELETE FROM media_assets WHERE tenant_id=1 AND key=$1`
			case "different-provider":
				mutation = `UPDATE media_assets SET storage_source='other-provider' WHERE tenant_id=1 AND key=$1`
			}
			if mutation != "" {
				if _, err := dbx.Connection().Exec(mutation, key); err != nil {
					t.Fatal(err)
				}
			}

			readCatalog := func() string {
				var record string
				if err := dbx.Connection().QueryRow(`SELECT COALESCE((SELECT to_jsonb(a)::text FROM media_assets a WHERE tenant_id=1 AND key=$1), 'null')`, key).Scan(&record); err != nil {
					t.Fatal(err)
				}
				return record
			}
			before := readCatalog()
			reads := 0
			bus.AddHandler(func(ctx context.Context, q *query.GetBlobByKey) error {
				reads++
				q.Result = &dto.Blob{Content: original, ContentType: "image/png"}
				return nil
			})

			for attempt := 0; attempt < 2; attempt++ {
				q := &query.GetMediaThumbnail{Key: key, Size: 200}
				if err := getMediaThumbnail(ctx, q); err != nil || q.Result == nil {
					t.Fatalf("thumbnail failed: %v", err)
				}
				if after := readCatalog(); after != before {
					t.Fatalf("thumbnail mutated catalog\nbefore: %s\nafter: %s", before, after)
				}
			}

			wantReads := 2
			if state == "ready" {
				wantReads = 1
			}
			if reads != wantReads {
				t.Fatalf("provider reads: got %d want %d", reads, wantReads)
			}
		})
	}
}
