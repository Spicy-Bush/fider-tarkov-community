package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/blob"
	"github.com/lib/pq"
)

func readMediaAssetReferences(trx *dbx.Trx, tenantID int, key string) ([]*dto.FileReference, error) {
	references := []*dto.FileReference{}
	err := trx.Select(&references, `
		SELECT kind, id, field, scope
		FROM media_references WHERE tenant_id=$1 AND key=$2
		ORDER BY kind, id, field
		LIMIT 50
	`, tenantID, key)
	return references, err
}

func mediaReferenceTransaction(t *testing.T) (context.Context, *dbx.Trx) {
	t.Helper()
	ctx := context.WithValue(context.Background(), app.TenantCtxKey, &entity.Tenant{ID: 1})
	ctx = context.WithValue(ctx, app.UserCtxKey, &entity.User{ID: 1, Role: enum.RoleAdministrator, Status: enum.UserActive})
	trx, err := dbx.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { trx.MustRollback() })
	return context.WithValue(ctx, app.TransactionCtxKey, trx), trx
}

func indexMediaFixture(t *testing.T, trx *dbx.Trx) {
	t.Helper()
	if err := BackfillMediaReferences(context.Background(), trx, 202609281700); err != nil {
		t.Fatal(err)
	}
}

func TestMediaBackfillPreservesOwnersAcrossBatches(t *testing.T) {
	_, trx := mediaReferenceTransaction(t)
	_, err := trx.Execute(`
		INSERT INTO posts (tenant_id, user_id, title, slug, number, status, created_at, description)
		SELECT CASE WHEN item <= 205 THEN 1 ELSE 2 END,
		       CASE WHEN item <= 205 THEN 1 ELSE 4 END,
		       'Backfill ' || item, 'backfill-' || item, NULL, 0, NOW(),
		       '![Image](/static/images/files/backfill.png)'
		FROM generate_series(1, 315) item
	`)
	if err != nil {
		t.Fatal(err)
	}

	for attempt := 0; attempt < 2; attempt++ {
		indexMediaFixture(t, trx)

		for tenantID, wanted := range map[int]int{1: 205, 2: 110} {
			var count int
			err := trx.Scalar(&count, `
				SELECT count(*) FROM media_asset_refs
				WHERE tenant_id=$1 AND key='files/backfill.png' AND kind='post' AND field='description'
			`, tenantID)
			if err != nil {
				t.Fatal(err)
			}
			if count != wanted {
				t.Fatalf("backfill %d, tenant %d: references=%d, want %d", attempt, tenantID, count, wanted)
			}
		}
	}
}

func TestMediaReferenceText(t *testing.T) {
	for _, test := range []struct {
		name string
		text string
		want []string
	}{
		{name: "empty", text: "ordinary content", want: []string{}},
		{name: "Markdown and query", text: `![image](/static/images/assets/picture?size=200)`, want: []string{"assets/picture"}},
		{name: "HTML and fragment", text: `<img src="https://cdn.example/static/images/pages/banner.webp#fragment">`, want: []string{"pages/banner.webp"}},
		{name: "encoded URL", text: `/static%2Fimages/assets/encoded%2Dname`, want: []string{"assets/encoded-name"}},
		{name: "HTML entities", text: `&#47;static&#x2F;images&sol;logos/image`, want: []string{"logos/image"}},
		{name: "escaped slash", text: `\/static\/images\/assets\/escaped`, want: []string{"assets/escaped"}},
		{name: "CSS escape", text: `url(\2f static/images/assets/css)`, want: []string{"assets/css"}},
		{name: "Markdown extension", text: `![image](/static/images/assets/image.webp)`, want: []string{"assets/image.webp"}},
		{name: "Markdown nested parentheses", text: `![image](/static/images/assets/image(one).webp)after`, want: []string{"assets/image(one).webp"}},
		{name: "encoded closing parenthesis", text: `![image](/static/images/assets/image%29.webp)`, want: []string{"assets/image).webp"}},
		{name: "quoted closing parenthesis", text: `<img src="/static/images/assets/image).webp">`, want: []string{"assets/image).webp"}},
		{name: "Unicode", text: `<img src="/static/images/custom/%E7%94%BB%E5%83%8F.png">`, want: []string{"custom/画像.png"}},
		{name: "literal percent", text: `/static/images/assets/name%2523`, want: []string{"assets/name%23"}},
		{name: "encoded fragment", text: `/static/images/assets/name%23part#section`, want: []string{"assets/name#part"}},
		{name: "encoded query", text: `/static/images/assets/name%3Fpart?size=200`, want: []string{"assets/name?part"}},
		{name: "OAuth logo", text: `<img src="/static/images/oauth/logo">`, want: []string{"oauth/logo"}},
		{name: "adjacent Markdown", text: `![saved](/static/images/assets/image)next`, want: []string{"assets/image"}},
		{name: "parenthesis in key", text: `![saved](/static/images/assets/image(one))`, want: []string{"assets/image(one)"}},
		{name: "encoded route", text: `/%73tatic/%69mages/assets/image`, want: []string{"assets/image"}},
		{name: "malformed UTF-8", text: `/static/images/assets/name%FF`, want: []string{"assets/name%FF"}},
		{name: "encoded fragment and query", text: `/static/images/assets/name%23part?size=200`, want: []string{"assets/name#part"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := mediaTextKeys(test.text)
			if !slices.Equal(result, test.want) {
				t.Fatalf("keys=%v want=%v", result, test.want)
			}
		})
	}
}

func TestMediaReferenceAmbiguousTextKeepsDecodedKeys(t *testing.T) {
	for _, test := range []struct {
		name string
		text string
		key  string
	}{
		{name: "HTML escaped ampersand", text: `<img src="/static/images/assets/one&amp;two">`, key: "assets/one&two"},
		{name: "named HTML entity", text: `![saved](/static/images/assets/caf&eacute;)`, key: "assets/café"},
		{name: "two codepoint entity", text: `![saved](/static/images/assets/&NotEqualTilde;)`, key: "assets/≂̸"},
		{name: "legacy numeric entity", text: `![saved](/static/images/assets/&#128;)`, key: "assets/€"},
		{name: "numeric entity without semicolon", text: `![saved](/static/images/assets/&#233)`, key: "assets/é"},
		{name: "literal named entity", text: `/static/images/assets/name%26eacute%3B`, key: "assets/name&eacute;"},
		{name: "single HTML decode", text: `![saved](/static/images/assets/name&amp;eacute;)`, key: "assets/name&eacute;"},
		{name: "HTML URL newline", text: `![saved](/static/images/assets/one&#10;two)`, key: "assets/onetwo"},
		{name: "CSS punctuation escape", text: `url(/static/images/assets/one\!two)`, key: "assets/one!two"},
		{name: "HTML closing parenthesis", text: `![image](/static/images/assets/image&#41;tail.webp)`, key: "assets/image)tail.webp"},
		{name: "CSS closing parenthesis", text: `url(/static/images/assets/image\)tail.webp)`, key: "assets/image)tail.webp"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if keys := mediaTextKeys(test.text); !slices.Contains(keys, test.key) {
				t.Fatalf("missing key %q from %q: keys=%v", test.key, test.text, keys)
			}
		})
	}
}

func TestMediaReferencesProtectPageDraftAndSettings(t *testing.T) {
	ctx, trx := mediaReferenceTransaction(t)
	var pageID int
	err := trx.Get(&pageID, `INSERT INTO pages
		(tenant_id, title, slug, content, banner_image_bkey, status, created_by_id, updated_by_id)
		VALUES (1, 'Retained Page', 'media-reference-page', '![image](/static/images/assets/shared)',
		'pages/banner', 'unpublished', 1, 1) RETURNING id`)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := trx.Execute(`INSERT INTO media_assets(tenant_id,key,name,content_type,size,created_at,is_public,storage_source)
		VALUES(1,'pages/draft-banner','Banner','image/png',1,NOW(),true,$1),
		      (1,'assets/draft-only','Draft image','image/png',1,NOW(),true,$1)`, blob.StorageSource()); err != nil {
		t.Fatal(err)
	}
	if _, err := trx.Execute(`
		INSERT INTO page_drafts(tenant_id, user_id, page_id, title, content, excerpt, banner_image_bkey, draft_data)
		VALUES(1, 1, $1, 'Unpublished work', '<img src="/static/images/assets/shared">',
		       '![Draft illustration](/static/images/assets/draft-only)', 'pages/draft-banner', '{"retained": true}')
	`, pageID); err != nil {
		t.Fatal(err)
	}

	if _, err := trx.Execute(`UPDATE tenants SET custom_css=$1, message_banner=$2 WHERE id=1`,
		`body { background: url('/static/images/assets/css'); }`,
		`![banner](/static/images/assets/shared)`); err != nil {
		t.Fatal(err)
	}
	indexMediaFixture(t, trx)

	for key, want := range map[string]int{
		"assets/shared":      3,
		"pages/banner":       1,
		"pages/draft-banner": 1,
		"assets/draft-only":  1,
		"assets/css":         1,
	} {
		references, err := readMediaAssetReferences(trx, 1, key)
		if err != nil || len(references) != want {
			t.Fatalf("%s references=%v err=%v", key, references, err)
		}
	}

	blocked, err := unlinkMediaAssetReferences(ctx, trx, 1, "assets/shared", mediaReferenceRemoval{})
	if err != nil || len(blocked) != 3 {
		t.Fatalf("embedded references blocked=%v err=%v", blocked, err)
	}

	blocked, err = unlinkMediaAssetReferences(ctx, trx, 1, "pages/draft-banner", mediaReferenceRemoval{})
	if err != nil || len(blocked) != 1 || blocked[0].Scope != "draft" {
		t.Fatalf("saved banner was not protected: blocked=%v err=%v", blocked, err)
	}
	blocked, err = unlinkMediaAssetReferences(ctx, trx, 1, "pages/draft-banner", mediaReferenceRemoval{IncludeDrafts: true})
	if err != nil || len(blocked) != 0 {
		t.Fatalf("explicit draft cleanup failed: blocked=%v err=%v", blocked, err)
	}

	var retained bool
	if err := trx.Scalar(&retained, `
		SELECT banner_image_bkey='' AND title='Unpublished work'
		       AND content='<img src="/static/images/assets/shared">'
		       AND excerpt='![Draft illustration](/static/images/assets/draft-only)'
		       AND draft_data='{"retained": true}'::jsonb
		FROM page_drafts WHERE tenant_id=1 AND page_id=$1 AND user_id=1
	`, pageID); err != nil || !retained {
		t.Fatalf("draft cleanup changed saved work: retained=%v err=%v", retained, err)
	}

	if _, err := trx.Execute(`DELETE FROM pages WHERE id=$1`, pageID); err != nil {
		t.Fatal(err)
	}
	remaining, err := readMediaAssetReferences(trx, 1, "assets/shared")
	if err != nil || len(remaining) != 1 || remaining[0].Kind != "tenant" {
		t.Fatalf("cascade leaves references=%v err=%v", remaining, err)
	}
}

func TestMediaForceUnlinksTypedReferencesWithinTenant(t *testing.T) {
	ctx, trx := mediaReferenceTransaction(t)
	const key = "logos/shared-reference"
	if _, err := trx.Execute(`UPDATE users SET avatar_bkey=$1 WHERE id IN (1,4)`, key); err != nil {
		t.Fatal(err)
	}
	if _, err := trx.Execute(`UPDATE tenants SET logo_bkey=$1 WHERE id=1`, key); err != nil {
		t.Fatal(err)
	}
	if _, err := trx.Execute(`INSERT INTO pages
		(tenant_id,title,slug,content,banner_image_bkey,created_by_id,updated_by_id)
		VALUES (1,'Banner','media-force-page','',$1,1,1)`, key); err != nil {
		t.Fatal(err)
	}
	indexMediaFixture(t, trx)

	blocked, err := unlinkMediaAssetReferences(ctx, trx, 1, key, mediaReferenceRemoval{Force: true})
	if err != nil || len(blocked) != 0 {
		t.Fatalf("unlink blocked=%v err=%v", blocked, err)
	}
	local, err := readMediaAssetReferences(trx, 1, key)
	if err != nil || len(local) != 0 {
		t.Fatalf("local references=%v err=%v", local, err)
	}
	foreign, err := readMediaAssetReferences(trx, 2, key)
	if err != nil || len(foreign) != 1 {
		t.Fatalf("foreign references=%v err=%v", foreign, err)
	}
}

func TestMediaReferencesRetainDeletedAttachmentsAndModeration(t *testing.T) {
	_, trx := mediaReferenceTransaction(t)
	var postID int
	if err := trx.Get(&postID, `INSERT INTO posts(title,slug,description,tenant_id,user_id,status,created_at)
		VALUES ('Deleted','media-deleted','',1,1,6,NOW()) RETURNING id`); err != nil {
		t.Fatal(err)
	}
	if _, err := trx.Execute(`INSERT INTO attachments(tenant_id,user_id,post_id,attachment_bkey)
		VALUES (1,1,$1,'attachments/deleted')`, postID); err != nil {
		t.Fatal(err)
	}
	if _, err := trx.Execute(`INSERT INTO moderation_checks
		(tenant_id,content_type,content_id,state,text_content,blob_keys,fallback_profile)
		VALUES (1,'avatar',2,'pending','','["avatars/proposed"]', '{"avatar_bkey":"avatars/fallback"}')`); err != nil {
		t.Fatal(err)
	}
	indexMediaFixture(t, trx)

	for _, key := range []string{"attachments/deleted", "avatars/proposed", "avatars/fallback"} {
		references, err := readMediaAssetReferences(trx, 1, key)
		if err != nil || len(references) != 1 {
			t.Fatalf("%s references=%v err=%v", key, references, err)
		}
	}
}

func TestMediaRemovalScopeMatchesListingAndUnlink(t *testing.T) {
	for _, scope := range []string{"active", "deleted", "draft"} {
		for _, removal := range []mediaReferenceRemoval{
			{},
			{IncludeDeleted: true},
			{IncludeDrafts: true},
			{IncludeDeleted: true, IncludeDrafts: true},
			{Force: true},
		} {
			ctx, trx := mediaReferenceTransaction(t)
			const key = "attachments/scope-policy"
			var commentID int

			if scope == "draft" {
				var pageID int
				if err := trx.Scalar(&pageID, `
					INSERT INTO pages(tenant_id,title,slug,content,status,created_by_id,updated_by_id)
					VALUES(1,'Draft','scope-policy','','draft',1,1) RETURNING id
				`); err != nil {
					t.Fatal(err)
				}

				if err := trx.Scalar(&commentID, `
					INSERT INTO comments(tenant_id,user_id,page_id,content,created_at)
					VALUES(1,1,$1,'Image',NOW()) RETURNING id
				`, pageID); err != nil {
					t.Fatal(err)
				}
			} else {
				var postID int
				status := 1
				if scope == "deleted" {
					status = 6
				}

				if err := trx.Scalar(&postID, `
					INSERT INTO posts(tenant_id,user_id,title,slug,status,created_at)
					VALUES(1,1,'Post','scope-policy',$1,NOW()) RETURNING id
				`, status); err != nil {
					t.Fatal(err)
				}

				if err := trx.Scalar(&commentID, `
					INSERT INTO comments(tenant_id,user_id,post_id,content,created_at)
					VALUES(1,1,$1,'Image',NOW()) RETURNING id
				`, postID); err != nil {
					t.Fatal(err)
				}
			}

			if _, err := trx.Execute(`
				INSERT INTO attachments(tenant_id,user_id,comment_id,attachment_bkey) VALUES(1,1,$1,$2);
			`, commentID, key); err != nil {
				t.Fatal(err)
			}

			indexMediaFixture(t, trx)
			included := scope == "deleted" && removal.IncludeDeleted || scope == "draft" && removal.IncludeDrafts
			var protected bool
			if err := trx.Scalar(&protected, `
				SELECT has_protected_references FROM media_reference_flags(1,ARRAY[$1],$2,$3)
			`, key, removal.IncludeDeleted, removal.IncludeDrafts); err != nil {
				t.Fatal(err)
			}

			if protected == included {
				t.Fatalf("scope=%s removal=%+v protected=%v", scope, removal, protected)
			}

			blocked, err := unlinkMediaAssetReferences(ctx, trx, 1, key, removal)
			wantBlocked := !included && !removal.Force
			if err != nil || (len(blocked) > 0) != wantBlocked {
				t.Fatalf("scope=%s removal=%+v blocked=%v err=%v", scope, removal, blocked, err)
			}

			var remains bool
			if err := trx.Scalar(&remains, "SELECT EXISTS(SELECT 1 FROM attachments WHERE tenant_id=1 AND attachment_bkey=$1)", key); err != nil {
				t.Fatal(err)
			}

			if remains != wantBlocked {
				t.Fatalf("scope=%s removal=%+v attachment retained=%v", scope, removal, remains)
			}

			trx.MustRollback()
		}
	}
}

func TestMediaReferenceScopeFollowsDiscussionOwner(t *testing.T) {
	_, trx := mediaReferenceTransaction(t)
	var pageID, commentID, attachmentID int
	if err := trx.Get(&pageID, `INSERT INTO pages
		(tenant_id,title,slug,content,status,created_by_id,updated_by_id)
		VALUES (1,'Discussion','media-scope-page','','draft',1,1) RETURNING id`); err != nil {
		t.Fatal(err)
	}
	if err := trx.Get(&commentID, `INSERT INTO comments(tenant_id,user_id,page_id,content,created_at)
		VALUES (1,1,$1,'![image](/static/images/assets/discussion)',NOW()) RETURNING id`, pageID); err != nil {
		t.Fatal(err)
	}
	if err := trx.Get(&attachmentID, `INSERT INTO attachments(tenant_id,user_id,comment_id,attachment_bkey)
		VALUES (1,1,$1,'assets/discussion') RETURNING id`, commentID); err != nil {
		t.Fatal(err)
	}
	indexMediaFixture(t, trx)

	assertScopes := func(want string) {
		t.Helper()
		references, err := readMediaAssetReferences(trx, 1, "assets/discussion")
		if err != nil || len(references) != 2 {
			t.Fatalf("references=%v err=%v", references, err)
		}
		for _, reference := range references {
			if reference.Scope != want {
				t.Fatalf("%s scope=%s want=%s", reference.Kind, reference.Scope, want)
			}
		}
	}

	assertScopes("draft")
	if _, err := trx.Execute(`UPDATE pages SET status='published' WHERE id=$1`, pageID); err != nil {
		t.Fatal(err)
	}
	assertScopes("active")

	if _, err := trx.Execute(`UPDATE comments SET deleted_at=NOW() WHERE id=$1`, commentID); err != nil {
		t.Fatal(err)
	}
	assertScopes("deleted")

	if _, err := trx.Execute(`UPDATE comments SET deleted_at=NULL WHERE id=$1`, commentID); err != nil {
		t.Fatal(err)
	}
	assertScopes("active")

	var deletedPostID int
	if err := trx.Get(&deletedPostID, `INSERT INTO posts(title,slug,description,tenant_id,user_id,status,created_at)
		VALUES ('Deleted','media-scope-deleted','',1,1,6,NOW()) RETURNING id`); err != nil {
		t.Fatal(err)
	}
	if _, err := trx.Execute(`UPDATE attachments SET comment_id=NULL, post_id=$2 WHERE id=$1`, attachmentID, deletedPostID); err != nil {
		t.Fatal(err)
	}

	references, err := readMediaAssetReferences(trx, 1, "assets/discussion")
	if err != nil || references[0].Kind != "attachment" || references[0].Scope != "deleted" || references[1].Scope != "active" {
		t.Fatalf("moved attachment references=%v err=%v", references, err)
	}
}

func TestMediaCleanupPreservesDeletedTextAndRejectsReuse(t *testing.T) {
	ctx, trx := mediaReferenceTransaction(t)
	const key = "assets/deleted-text"
	const content = "Retain this explanation. ![saved](/static/images/assets/deleted-text) More text."
	if _, err := trx.Execute(`INSERT INTO media_assets(tenant_id,key,name,content_type,size)
		VALUES (1,$1,'Deleted image','image/png',1)`, key); err != nil {
		t.Fatal(err)
	}

	var postID int
	if err := trx.Get(&postID, `INSERT INTO posts(title,slug,description,tenant_id,user_id,status,created_at)
		VALUES ('Deleted','media-cleanup-deleted',$1,1,1,6,NOW()) RETURNING id`, content); err != nil {
		t.Fatal(err)
	}
	if _, err := trx.Execute(`INSERT INTO attachments(tenant_id,user_id,post_id,attachment_bkey)
		VALUES (1,1,$1,$2)`, postID, key); err != nil {
		t.Fatal(err)
	}
	indexMediaFixture(t, trx)

	blocked, err := unlinkMediaAssetReferences(ctx, trx, 1, key, mediaReferenceRemoval{})
	if err != nil || len(blocked) != 2 {
		t.Fatalf("default cleanup blocked=%v err=%v", blocked, err)
	}
	blocked, err = unlinkMediaAssetReferences(ctx, trx, 1, key, mediaReferenceRemoval{IncludeDeleted: true})
	if err != nil || len(blocked) != 0 {
		t.Fatalf("selected cleanup blocked=%v err=%v", blocked, err)
	}
	if _, err := trx.Execute(`UPDATE media_assets SET deletion_requested_at=NOW(), deleted_at=NOW()
		WHERE tenant_id=1 AND key=$1`, key); err != nil {
		t.Fatal(err)
	}

	var saved string
	if err := trx.Get(&saved, `SELECT description FROM posts WHERE id=$1`, postID); err != nil || saved != content {
		t.Fatalf("saved text=%q err=%v", saved, err)
	}
	references, err := readMediaAssetReferences(trx, 1, key)
	if err != nil || len(references) != 1 || references[0].Kind != "post" {
		t.Fatalf("historical references=%v err=%v", references, err)
	}

	if _, err := trx.Execute(`UPDATE posts SET description=description || ' An unrelated edit.', status=1 WHERE id=$1`, postID); err != nil {
		t.Fatal(err)
	}
	if err := flushMediaReferences(trx); err != nil {
		t.Fatal(err)
	}
	if err := saveModerationCheck(trx, 1, "post", postID, content, "[]"); err != nil {
		t.Fatalf("moderation of retained text: %v", err)
	}
	if err := flushMediaReferences(trx); err != nil {
		t.Fatal(err)
	}

	for _, write := range []func() error{
		func() error {
			changes := attachmentChanges{uploaded: []string{key}}
			return changes.apply(ctx, postID, 0)
		},
		func() error {
			return setPostResponse(ctx, &cmd.SetPostResponse{
				Post: &entity.Post{ID: postID}, Text: "![new](/static/images/assets/deleted-text)", Status: enum.PostOpen,
			})
		},
	} {
		if _, err := trx.Execute(`SAVEPOINT media_reuse`); err != nil {
			t.Fatal(err)
		}
		err := write()
		if err == nil {
			err = flushMediaReferences(trx)
		}
		if err == nil {
			t.Fatal("new reference to deleted image was accepted")
		}
		if _, err := trx.Execute(`ROLLBACK TO SAVEPOINT media_reuse`); err != nil {
			t.Fatal(err)
		}
	}

	references, err = readMediaAssetReferences(trx, 1, key)
	if err != nil || len(references) != 2 || references[0].Scope != "active" || references[1].Scope != "active" {
		t.Fatalf("restored historical references=%v err=%v", references, err)
	}
}

func TestMediaCleanupDoesNotDetachActiveOwners(t *testing.T) {
	ctx, trx := mediaReferenceTransaction(t)
	const key = "pages/shared-banner"
	for _, status := range []string{"draft", "published"} {
		if _, err := trx.Execute(`INSERT INTO pages
			(tenant_id,title,slug,content,banner_image_bkey,status,created_by_id,updated_by_id)
			VALUES (1,$1,$1,'',$2,$3,1,1)`, "media-cleanup-"+status, key, status); err != nil {
			t.Fatal(err)
		}
	}
	indexMediaFixture(t, trx)

	blocked, err := unlinkMediaAssetReferences(ctx, trx, 1, key, mediaReferenceRemoval{IncludeDrafts: true})
	if err != nil || len(blocked) != 1 || blocked[0].Scope != "active" {
		t.Fatalf("active owner blocked=%v err=%v", blocked, err)
	}
	references, err := readMediaAssetReferences(trx, 1, key)
	if err != nil || len(references) != 2 {
		t.Fatalf("blocked operation partially detached references=%v err=%v", references, err)
	}

	blocked, err = unlinkMediaAssetReferences(ctx, trx, 1, key, mediaReferenceRemoval{Force: true})
	if err != nil || len(blocked) != 0 {
		t.Fatalf("force blocked=%v err=%v", blocked, err)
	}
	references, err = readMediaAssetReferences(trx, 1, key)
	if err != nil || len(references) != 0 {
		t.Fatalf("force retained typed references=%v err=%v", references, err)
	}
}

func TestMediaForceCancelsPendingAvatar(t *testing.T) {
	ctx, trx := mediaReferenceTransaction(t)
	const key = "avatars/pending-cleanup"
	if _, err := trx.Execute(`UPDATE users SET avatar_bkey=$1 WHERE id=2`, key); err != nil {
		t.Fatal(err)
	}
	if _, err := trx.Execute(`INSERT INTO moderation_checks
		(tenant_id,content_type,content_id,state,text_content,blob_keys,fallback_profile)
		VALUES (1,'avatar',2,'running','',jsonb_build_array($1::text),'{"avatar_bkey":"avatars/previous"}')`, key); err != nil {
		t.Fatal(err)
	}
	indexMediaFixture(t, trx)

	blocked, err := unlinkMediaAssetReferences(ctx, trx, 1, key, mediaReferenceRemoval{Force: true})
	if err != nil || len(blocked) != 0 {
		t.Fatalf("force blocked=%v err=%v", blocked, err)
	}
	var state string
	if err := trx.Get(&state, `SELECT state FROM moderation_checks WHERE tenant_id=1 AND content_type='avatar' AND content_id=2`); err != nil || state != "canceled" {
		t.Fatalf("moderation state=%s err=%v", state, err)
	}
	for _, asset := range []string{key, "avatars/previous"} {
		references, err := readMediaAssetReferences(trx, 1, asset)
		if err != nil || len(references) != 0 {
			t.Fatalf("%s references=%v err=%v", asset, references, err)
		}
	}
}

func TestMediaDeletionLocksTenantBeforeUsers(t *testing.T) {
	dbx.Seed()
	t.Cleanup(dbx.Seed)
	const key = "assets/shared-logo-avatar"

	_, fixture := mediaReferenceTransaction(t)
	if _, err := fixture.Execute(`UPDATE tenants SET logo_bkey = $1 WHERE id = 1`, key); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.Execute(`UPDATE users SET avatar_bkey = $1 WHERE id = 1`, key); err != nil {
		t.Fatal(err)
	}
	indexMediaFixture(t, fixture)
	fixture.MustCommit()

	permissions := mediaRaceTransaction(t)
	if _, err := permissions.Exec(`SELECT id FROM tenants WHERE id = 1 FOR UPDATE`); err != nil {
		t.Fatal(err)
	}
	if _, err := permissions.Exec(`SET LOCAL lock_timeout = '100ms'`); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	deletion, err := dbx.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer deletion.Rollback()

	var processID int
	if err := deletion.Scalar(&processID, "SELECT pg_backend_pid()"); err != nil {
		t.Fatal(err)
	}
	completed := make(chan error, 1)
	go func() {
		_, err := deletion.Execute("SELECT lock_media_reference_owners($1,$2)", 1, key)
		completed <- err
	}()

	for {
		var waiting bool
		if err := dbx.Connection().QueryRowContext(ctx, `
			SELECT COALESCE(wait_event_type = 'Lock', false) FROM pg_stat_activity WHERE pid = $1
		`, processID).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case err := <-completed:
			t.Fatalf("media owner locking finished without waiting for the tenant: %v", err)
		case <-ctx.Done():
			t.Fatal("media deletion never reached the held tenant")
		case <-time.After(time.Millisecond):
		}
	}

	_, actorErr := permissions.Exec(`SELECT id FROM users WHERE id = 1 FOR SHARE`)
	if err := permissions.Rollback(); err != nil && err != sql.ErrTxDone {
		t.Fatal(err)
	}
	if err := <-completed; err != nil {
		t.Fatal(err)
	}
	if actorErr != nil {
		t.Fatalf("media deletion held the actor while waiting for the tenant: %v", actorErr)
	}
}

func mediaRacePage(t *testing.T, key string) int {
	t.Helper()
	var pageID int
	err := dbx.Connection().QueryRow(`INSERT INTO pages
		(tenant_id,title,slug,content,status,created_by_id,updated_by_id)
		VALUES (1,'Race',$1,'','draft',1,1) RETURNING id`, "media-race-"+key[len("assets/"):]).Scan(&pageID)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if _, err := dbx.Connection().Exec(`DELETE FROM pages WHERE id=$1`, pageID); err != nil {
			t.Error(err)
		}
		if _, err := dbx.Connection().Exec(`DELETE FROM media_assets WHERE tenant_id=1 AND key=$1`, key); err != nil {
			t.Error(err)
		}
	})
	return pageID
}

func mediaRaceTransaction(t *testing.T) *sql.Tx {
	t.Helper()
	transaction, err := dbx.Connection().Begin()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { transaction.Rollback() })
	return transaction
}

func expectMediaLockConflict(t *testing.T, statement string, arguments ...any) {
	t.Helper()
	transaction := mediaRaceTransaction(t)
	if _, err := transaction.Exec(`SET LOCAL lock_timeout='50ms'`); err != nil {
		t.Fatal(err)
	}
	_, err := transaction.Exec(statement, arguments...)
	var databaseError *pq.Error
	if !errors.As(err, &databaseError) || databaseError.Code != "55P03" {
		t.Fatalf("expected conflicting database lock, got %v", err)
	}
	transaction.Rollback()
}

func TestMediaReferencePublicationLocksInventoryAndDeletion(t *testing.T) {
	for _, indexed := range []bool{false, true} {
		name := "unindexed"
		if indexed {
			name = "indexed"
		}
		t.Run(name, func(t *testing.T) {
			key := "assets/race-" + name
			pageID := mediaRacePage(t, key)
			if indexed {
				if _, err := dbx.Connection().Exec(`INSERT INTO media_assets(tenant_id,key,name,content_type,size)
					VALUES (1,$1,'Race','image/png',1)`, key); err != nil {
					t.Fatal(err)
				}
			}

			_, writer := mediaReferenceTransaction(t)
			if _, err := writer.Execute(`UPDATE pages SET content=$2 WHERE id=$1`, pageID, "![saved](/static/images/"+key+")"); err != nil {
				t.Fatal(err)
			}
			if err := flushMediaReferences(writer); err != nil {
				t.Fatal(err)
			}
			if indexed {
				expectMediaLockConflict(t, `UPDATE media_assets SET deletion_requested_at=NOW() WHERE tenant_id=1 AND key=$1`, key)
			} else {
				expectMediaLockConflict(t, `INSERT INTO media_assets(tenant_id,key,name,content_type,size)
					VALUES (1,$1,'Race','image/png',1)`, key)
			}
			if err := writer.Commit(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMediaTombstoneRejectsWaitingPublication(t *testing.T) {
	const key = "assets/race-tombstone"
	pageID := mediaRacePage(t, key)
	deleting := mediaRaceTransaction(t)
	if _, err := deleting.Exec(`INSERT INTO media_assets
		(tenant_id,key,name,content_type,size,deletion_requested_at,deleted_at)
		VALUES (1,$1,'Race','image/png',1,NOW(),NOW())`, key); err != nil {
		t.Fatal(err)
	}
	_, waiting := mediaReferenceTransaction(t)
	if _, err := waiting.Execute(`SET LOCAL lock_timeout='50ms'`); err != nil {
		t.Fatal(err)
	}
	if _, err := waiting.Execute(`UPDATE pages SET content=$2 WHERE id=$1`, pageID, "![saved](/static/images/"+key+")"); err != nil {
		t.Fatal(err)
	}
	err := flushMediaReferences(waiting)
	var databaseError *pq.Error
	if !errors.As(err, &databaseError) || databaseError.Code != "55P03" {
		t.Fatalf("publication did not wait for deletion: %v", err)
	}
	waiting.Rollback()
	if err := deleting.Commit(); err != nil {
		t.Fatal(err)
	}

	_, writer := mediaReferenceTransaction(t)
	if _, err := writer.Execute(`UPDATE pages SET content=$2 WHERE id=$1`, pageID, "![saved](/static/images/"+key+")"); err != nil {
		t.Fatal(err)
	}
	err = flushMediaReferences(writer)
	if !errors.As(err, &databaseError) || databaseError.Code != "23503" {
		t.Fatalf("publication after tombstone: %v", err)
	}
}

func TestMediaDraftCleanupSerializesPublication(t *testing.T) {
	const key = "assets/race-publish"
	pageID := mediaRacePage(t, key)
	_, fixture := mediaReferenceTransaction(t)
	if _, err := fixture.Execute(`UPDATE pages SET banner_image_bkey=$2 WHERE id=$1`, pageID, key); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.Execute(`INSERT INTO media_assets(tenant_id,key,name,content_type,size)
		VALUES (1,$1,'Race','image/png',1)
		ON CONFLICT(tenant_id,key) DO UPDATE SET name=EXCLUDED.name,
		content_type=EXCLUDED.content_type,size=EXCLUDED.size,storage_source='sql'`, key); err != nil {
		t.Fatal(err)
	}
	indexMediaFixture(t, fixture)
	fixture.MustCommit()

	ctx, deleting := mediaReferenceTransaction(t)
	blocked, err := unlinkMediaAssetReferences(ctx, deleting, 1, key, mediaReferenceRemoval{IncludeDrafts: true})
	if err != nil || len(blocked) != 0 {
		t.Fatalf("cleanup blocked=%v err=%v", blocked, err)
	}
	expectMediaLockConflict(t, `UPDATE pages SET status='published' WHERE id=$1`, pageID)
	if _, err := deleting.Execute(`UPDATE media_assets SET deletion_requested_at=NOW(),deleted_at=NOW()
		WHERE tenant_id=1 AND key=$1`, key); err != nil {
		t.Fatal(err)
	}
	deleting.MustCommit()

	if _, err := dbx.Connection().Exec(`UPDATE pages SET status='published' WHERE id=$1`, pageID); err != nil {
		t.Fatal(err)
	}
	var banner string
	if err := dbx.Connection().QueryRow(`SELECT banner_image_bkey FROM pages WHERE id=$1`, pageID).Scan(&banner); err != nil || banner != "" {
		t.Fatalf("published cleaned banner=%q err=%v", banner, err)
	}
}

func TestMediaContentWritersPublishAndReplaceReferences(t *testing.T) {
	dbx.Seed()
	bus.Init(Service{})
	ctx, trx := mediaReferenceTransaction(t)
	ctx = context.WithValue(ctx, app.RequestCtxKey, web.Request{URL: &url.URL{Scheme: "http", Host: "localhost:3000"}})
	tenant := &query.GetTenantByDomain{Domain: "demo"}
	if err := getTenantByDomain(ctx, tenant); err != nil {
		t.Fatal(err)
	}
	ctx = context.WithValue(ctx, app.TenantCtxKey, tenant.Result)
	const before = "/static/images/assets/writer-before"
	const after = "/static/images/assets/writer-after"

	post := &cmd.AddNewPost{Title: "Reference ownership", Description: before}
	if err := addNewPost(ctx, post); err != nil {
		t.Fatal(err)
	}
	page := &cmd.CreatePage{Title: "Reference ownership", Content: before, Status: entity.PageStatusPublished}
	if err := createPage(ctx, page); err != nil {
		t.Fatal(err)
	}
	response := &cmd.CreateCannedResponse{Title: "Reference ownership", Type: "response", Content: before}
	if err := createCannedResponse(ctx, response); err != nil {
		t.Fatal(err)
	}
	if err := UpdateMessageBanner(ctx, &cmd.UpdateMessageBanner{MessageBanner: before}); err != nil {
		t.Fatal(err)
	}
	if err := saveProfileAvatar(ctx, &cmd.SaveProfileAvatar{UserID: 1, AvatarType: enum.AvatarTypeCustom, BlobKey: "assets/writer-before"}); err != nil {
		t.Fatal(err)
	}

	assertOwners := func(key string, expected int) {
		t.Helper()
		if err := flushMediaReferences(trx); err != nil {
			t.Fatal(err)
		}

		references, err := readMediaAssetReferences(trx, 1, key)
		if err != nil || len(references) != expected {
			t.Fatalf("%s references=%v expected=%d err=%v", key, references, expected, err)
		}
	}
	assertOwners("assets/writer-before", 5)

	if err := updatePost(ctx, &cmd.UpdatePost{Post: post.Result, Title: post.Result.Title, Description: after}); err != nil {
		t.Fatal(err)
	}
	if err := updatePage(ctx, &cmd.UpdatePage{PageID: page.Result.ID, Title: page.Result.Title, Content: after, Status: page.Result.Status}); err != nil {
		t.Fatal(err)
	}
	if err := updateCannedResponse(ctx, &cmd.UpdateCannedResponse{ID: response.Result.ID, Title: response.Result.Title, Type: "response", Content: after}); err != nil {
		t.Fatal(err)
	}
	if err := UpdateMessageBanner(ctx, &cmd.UpdateMessageBanner{MessageBanner: after}); err != nil {
		t.Fatal(err)
	}
	if err := saveProfileAvatar(ctx, &cmd.SaveProfileAvatar{UserID: 1, AvatarType: enum.AvatarTypeCustom, BlobKey: "assets/writer-after"}); err != nil {
		t.Fatal(err)
	}

	assertOwners("assets/writer-before", 0)
	assertOwners("assets/writer-after", 5)
}

func TestRetainedPageDraftIsConsumedAfterSuccessfulOwnSave(t *testing.T) {
	dbx.Seed()
	bus.Init(Service{})
	ctx, trx := mediaReferenceTransaction(t)
	ctx = context.WithValue(ctx, app.RequestCtxKey, web.Request{URL: &url.URL{Scheme: "http", Host: "localhost:3000"}})

	page := &cmd.CreatePage{Title: "Retained draft", Content: "Published text", Status: entity.PageStatusPublished}
	if err := createPage(ctx, page); err != nil {
		t.Fatal(err)
	}
	const key = "pages/retained-banner"
	if _, err := trx.Execute(`
		INSERT INTO media_assets(tenant_id, key, name, content_type, size, storage_source)
		VALUES(1, $1, 'Saved banner', 'image/png', 100, $2)
	`, key, blob.StorageSource()); err != nil {
		t.Fatal(err)
	}
	if _, err := trx.Execute(`
		INSERT INTO page_drafts(tenant_id, page_id, user_id, title, slug, content, show_toc, banner_image_bkey)
		VALUES(1, $1, 1, 'Own saved work', 'saved-work', 'Recovered draft text', true, $2),
		      (1, $1, 2, 'Other saved work', 'other-work', 'Other draft text', false, $2)
	`, page.Result.ID, key); err != nil {
		t.Fatal(err)
	}
	indexMediaFixture(t, trx)

	draft := &query.GetPageDraft{PageID: page.Result.ID, UserID: 1}
	if err := getPageDraft(ctx, draft); err != nil {
		t.Fatal(err)
	}
	if draft.Result.Title != "Own saved work" || draft.Result.Slug != "saved-work" ||
		draft.Result.Content != "Recovered draft text" || !draft.Result.ShowTOC || draft.Result.BannerImageBKey != key {
		t.Fatalf("saved editor fields were not restored: %+v", draft.Result)
	}

	encoded, err := json.Marshal(draft.Result)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if _, present := fields["draftData"]; present {
		t.Fatal("unused opaque draft data was exposed to the editor")
	}

	invalidParent := 999999
	edit := &cmd.UpdatePage{
		PageID: page.Result.ID, Title: page.Title, Content: "Recovered draft text",
		Status: page.Status, BannerImage: &dto.ImageUpload{BlobKey: key}, ParentPageID: &invalidParent,
	}
	if err := updatePage(ctx, edit); err == nil {
		t.Fatal("invalid Page parent was accepted")
	}
	var count int
	if err := trx.Scalar(&count, `SELECT count(*) FROM page_drafts WHERE page_id=$1`, page.Result.ID); err != nil || count != 2 {
		t.Fatalf("failed save consumed work: drafts=%d err=%v", count, err)
	}

	edit.ParentPageID = nil
	if err := updatePage(ctx, edit); err != nil {
		t.Fatal(err)
	}
	if edit.Result.BannerImageBKey != key {
		t.Fatal("saved draft banner was lost")
	}
	if err := trx.Scalar(&count, `SELECT count(*) FROM page_drafts WHERE page_id=$1 AND user_id=2`, page.Result.ID); err != nil || count != 1 {
		t.Fatalf("another author's retained work was consumed: drafts=%d err=%v", count, err)
	}
	if err := trx.Scalar(&count, `SELECT count(*) FROM page_drafts WHERE page_id=$1 AND user_id=1`, page.Result.ID); err != nil || count != 0 {
		t.Fatalf("completed draft remained available for stale import: drafts=%d err=%v", count, err)
	}
}

func TestMediaConfigurationLinksProtectImages(t *testing.T) {
	dbx.Seed()
	ctx, trx := mediaReferenceTransaction(t)
	const key = "assets/configuration-image"
	const imageURL = "/static/images/" + key
	const content = `{"image":{"url":"https://site.example` + imageURL + `"}}`

	var navigationID, webhookID int
	if err := trx.Scalar(&navigationID, `INSERT INTO navigation_links
		(tenant_id,title,url,display_order,location)
		VALUES (1,'Image guide',$1,0,'footer') RETURNING id`, imageURL); err != nil {
		t.Fatal(err)
	}
	if err := trx.Scalar(&webhookID, `INSERT INTO webhooks
		(tenant_id,name,type,status,url,content,http_method,http_headers)
		VALUES (1,'Static embed',1,1,'https://example.invalid/events',$1,'POST','{}') RETURNING id`, content); err != nil {
		t.Fatal(err)
	}
	if _, err := trx.Execute(`INSERT INTO navigation_links
		(tenant_id,title,url,display_order,location)
		VALUES (2,'Other tenant image',$1,0,'footer')`, imageURL); err != nil {
		t.Fatal(err)
	}

	blocked, err := unlinkMediaAssetReferences(ctx, trx, 1, key, mediaReferenceRemoval{IncludeDeleted: true, IncludeDrafts: true})
	if err != nil || len(blocked) != 2 || blocked[0].Kind != "navigation" || blocked[1].Kind != "webhook" {
		t.Fatalf("active configuration references=%v error=%v", blocked, err)
	}
	blocked, err = unlinkMediaAssetReferences(ctx, trx, 1, key, mediaReferenceRemoval{Force: true})
	if err != nil || len(blocked) != 0 {
		t.Fatalf("explicit deletion blocked=%v error=%v", blocked, err)
	}

	var savedURL, savedContent string
	if err := trx.Scalar(&savedURL, "SELECT url FROM navigation_links WHERE id=$1", navigationID); err != nil {
		t.Fatal(err)
	}
	if err := trx.Scalar(&savedContent, "SELECT content FROM webhooks WHERE id=$1", webhookID); err != nil {
		t.Fatal(err)
	}
	if savedURL != imageURL || savedContent != content {
		t.Fatal("forced deletion rewrote saved configuration text")
	}

	if _, err := trx.Execute("UPDATE webhooks SET url=$2, content='' WHERE id=$1", webhookID, imageURL); err != nil {
		t.Fatal(err)
	}
	if _, err := trx.Execute("DELETE FROM navigation_links WHERE id=$1", navigationID); err != nil {
		t.Fatal(err)
	}
	if err := flushMediaReferences(trx); err != nil {
		t.Fatal(err)
	}

	references, err := readMediaAssetReferences(trx, 1, key)
	if err != nil || len(references) != 1 || references[0].Kind != "webhook" || references[0].Field != "url" {
		t.Fatalf("changed configuration references=%v error=%v", references, err)
	}
	foreign, err := readMediaAssetReferences(trx, 2, key)
	if err != nil || len(foreign) != 1 || foreign[0].Kind != "navigation" {
		t.Fatalf("other tenant references=%v error=%v", foreign, err)
	}

	if _, err := trx.Execute("DELETE FROM webhooks WHERE id=$1", webhookID); err != nil {
		t.Fatal(err)
	}
	references, err = readMediaAssetReferences(trx, 1, key)
	if err != nil || len(references) != 0 {
		t.Fatalf("deleted configuration references=%v error=%v", references, err)
	}
}
