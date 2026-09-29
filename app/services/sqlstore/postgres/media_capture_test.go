package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/lib/pq"
)

func TestMediaCaptureSkipsPlainTextAndKeepsEncodedReferences(t *testing.T) {
	dbx.Seed()
	t.Cleanup(dbx.Seed)
	plain := strings.Repeat("An ordinary paragraph. ", 2000)
	for index, content := range []string{
		`![image](/static/images/assets/captured.webp)`,
		`![image](/static%2Fimages/assets/captured.webp)`,
		`![image](&#47;static&sol;images&sol;assets/captured.webp)`,
		`url(\2f static/images/assets/captured.webp)`,
	} {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			_, trx := mediaReferenceTransaction(t)
			var postID int
			if err := trx.Get(&postID, `
				INSERT INTO posts (tenant_id,user_id,title,slug,number,status,created_at,description)
				VALUES (1,1,'Reference source',$1,$2,1,NOW(),$3) RETURNING id
			`, fmt.Sprintf("reference-source-%d", index), 100+index, plain); err != nil {
				t.Fatal(err)
			}

			check := func(wantPending, wantReferences int) {
				t.Helper()
				var pending, references int
				if err := trx.Scalar(&pending, "SELECT COUNT(*) FROM media_reference_changes WHERE owner_id=$1 AND kind='post'", postID); err != nil {
					t.Fatal(err)
				}
				if err := trx.Scalar(&references, "SELECT COUNT(*) FROM media_asset_refs WHERE owner_id=$1 AND kind='post'", postID); err != nil {
					t.Fatal(err)
				}
				if pending != wantPending || references != wantReferences {
					t.Fatalf("pending=%d references=%d, want pending=%d references=%d", pending, references, wantPending, wantReferences)
				}
			}

			check(0, 0)
			if _, err := trx.Execute("UPDATE posts SET description=$2 WHERE id=$1", postID, content); err != nil {
				t.Fatal(err)
			}
			check(1, 0)
			if err := flushMediaReferences(trx); err != nil {
				t.Fatal(err)
			}
			check(0, 1)

			if _, err := trx.Execute("UPDATE posts SET description=description WHERE id=$1", postID); err != nil {
				t.Fatal(err)
			}
			check(0, 1)

			if _, err := trx.Execute("UPDATE posts SET description=$2 WHERE id=$1", postID, plain); err != nil {
				t.Fatal(err)
			}
			check(0, 0)
			if err := trx.Commit(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMediaCaptureCompletesAtOwningTransaction(t *testing.T) {
	dbx.Seed()
	t.Cleanup(dbx.Seed)
	ctx, trx := mediaReferenceTransaction(t)
	const first = "avatars/captured-first"
	const final = "avatars/captured-final"

	err := using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if _, err := trx.Execute("UPDATE users SET avatar_bkey=$1 WHERE id=1", first); err != nil {
			return err
		}

		return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
			_, err := trx.Execute("UPDATE users SET avatar_bkey=$1 WHERE id=1", final)
			return err
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	var pending int
	if err := trx.Scalar(&pending, "SELECT COUNT(*) FROM media_reference_changes WHERE transaction_id=txid_current()"); err != nil {
		t.Fatal(err)
	}
	if pending != 1 {
		t.Fatalf("nested writes retained %d source snapshots", pending)
	}
	if err := trx.Commit(); err != nil {
		t.Fatal(err)
	}

	var refs, changes int
	if err := dbx.Connection().QueryRow("SELECT COUNT(*) FROM media_asset_refs WHERE key=ANY($1)", pq.Array([]string{first, final})).Scan(&refs); err != nil {
		t.Fatal(err)
	}
	if err := dbx.Connection().QueryRow("SELECT COUNT(*) FROM media_reference_changes").Scan(&changes); err != nil {
		t.Fatal(err)
	}
	if refs != 1 || changes != 0 {
		t.Fatalf("committed refs=%d pending=%d", refs, changes)
	}

	var key string
	if err := dbx.Connection().QueryRow("SELECT key FROM media_asset_refs WHERE tenant_id=1 AND kind='user' AND owner_id=1").Scan(&key); err != nil {
		t.Fatal(err)
	}
	if key != final {
		t.Fatalf("published obsolete key %q", key)
	}
}

func TestMediaCaptureRejectsUnfinishedDirectWrite(t *testing.T) {
	dbx.Seed()
	t.Cleanup(dbx.Seed)
	_, err := dbx.Connection().Exec("UPDATE users SET avatar_bkey='avatars/untracked' WHERE id=1")
	var constraint *pq.Error
	if !errors.As(err, &constraint) || constraint.Code != "23514" {
		t.Fatalf("write bypassed reference completion: %v", err)
	}

	var key string
	if err := dbx.Connection().QueryRow("SELECT avatar_bkey FROM users WHERE id=1").Scan(&key); err != nil {
		t.Fatal(err)
	}
	if key != "" {
		t.Fatalf("rejected write persisted %q", key)
	}
}

func TestMediaCaptureRetainsCompletionBarrierAfterExtractionFailure(t *testing.T) {
	dbx.Seed()
	t.Cleanup(dbx.Seed)
	_, trx := mediaReferenceTransaction(t)

	_, err := trx.Execute(`INSERT INTO moderation_checks
		(tenant_id, content_type, content_id, text_content, blob_keys, state)
		VALUES (1, 'avatar', 1, '', '[1]'::jsonb, 'pending')`)
	if err != nil {
		t.Fatal(err)
	}

	if err := flushMediaReferences(trx); err == nil {
		t.Fatal("malformed metadata was accepted")
	}

	var constraint *pq.Error
	if err := trx.Commit(); !errors.As(err, &constraint) || constraint.Code != "23514" {
		t.Fatalf("failed extraction bypassed reference completion: %v", err)
	}
}
