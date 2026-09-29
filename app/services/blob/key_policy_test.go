package blob_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/blob"
)

func TestCanonicalBlobKeys(t *testing.T) {
	for _, key := range []string{"file", "images/one.png", "images/.hidden", "images/two%20words", "images/画像.png"} {
		if err := blob.ValidateKey(key); err != nil {
			t.Errorf("valid key %q: %v", key, err)
		}
	}
	for _, key := range []string{"", ".", "..", "../outside", "/absolute", "a//b", "a/./b", "a/../b", "a/", "a\\b", "a\x00b", "a\nb", "a\tb", "a b", "a\u00a0b", string([]byte{255}), strings.Repeat("x", 513)} {
		if err := blob.ValidateKey(key); err != blob.ErrInvalidKeyFormat {
			t.Errorf("invalid key accepted %q: %v", key, err)
		}
	}
	for _, prefix := range []string{"", "images/", "images/a"} {
		if err := blob.ValidatePrefix(prefix); err != nil {
			t.Errorf("valid prefix %q: %v", prefix, err)
		}
	}
	for _, prefix := range []string{"/", "../", "images//", "images/../"} {
		if err := blob.ValidatePrefix(prefix); err == nil {
			t.Errorf("invalid prefix accepted %q", prefix)
		}
	}
}

func TestAdministrativeBlobReadPolicy(t *testing.T) {
	ctx := context.WithValue(context.Background(), app.TenantCtxKey, &entity.Tenant{ID: 1})
	for _, role := range []enum.Role{enum.RoleVisitor, enum.RoleHelper, enum.RoleModerator, enum.RoleCollaborator, enum.RoleAdministrator} {
		for _, status := range []enum.UserStatus{enum.UserActive, enum.UserBlocked, enum.UserDeleted, 0} {
			user := &entity.User{ID: 1, Role: role, Status: status}
			actor := context.WithValue(ctx, app.UserCtxKey, user)
			err := blob.AuthorizeRead(actor, &query.GetBlobByKey{Key: "files/private"})
			allowed := role == enum.RoleAdministrator && status == enum.UserActive
			if (err == nil) != allowed {
				t.Errorf("role=%s status=%d authorized=%t err=%v", role, status, allowed, err)
			}
		}
	}
	if err := blob.AuthorizeRead(ctx, &query.GetBlobByKey{Key: "files/private"}); err != blob.ErrNotFound {
		t.Fatalf("anonymous file access: %v", err)
	}
}

func TestAvatarPublicationReadPolicy(t *testing.T) {
	bus.Reset()
	published := false
	bus.AddHandler(func(ctx context.Context, q *query.IsAvatarPublished) error {
		q.Result = published
		return nil
	})
	q := &query.GetBlobByKey{Key: "avatars/pending"}
	if err := blob.AuthorizeRead(context.Background(), q); err != blob.ErrNotFound {
		t.Fatalf("unpublished avatar exposed: %v", err)
	}
	published = true
	if err := blob.AuthorizeRead(context.Background(), q); err != nil {
		t.Fatal(err)
	}
	published = false
	q.AllowUnpublishedAvatar = true
	if err := blob.AuthorizeRead(context.Background(), q); err != nil {
		t.Fatalf("trusted moderation read failed: %v", err)
	}
}

func TestBackupBlobReadRequiresExportPermission(t *testing.T) {
	bus.Reset()
	ctx := context.WithValue(context.Background(), app.TenantCtxKey, &entity.Tenant{ID: 1})
	for _, role := range []enum.Role{enum.RoleVisitor, enum.RoleHelper, enum.RoleModerator, enum.RoleCollaborator, enum.RoleAdministrator} {
		for _, status := range []enum.UserStatus{enum.UserActive, enum.UserBlocked, enum.UserDeleted, 0} {
			user := &entity.User{ID: 1, Role: role, Status: status}
			actor := context.WithValue(ctx, app.UserCtxKey, user)
			for _, key := range []string{"files/private", "avatars/unpublished"} {
				err := blob.AuthorizeRead(actor, &query.GetBlobByKey{Key: key, ForBackup: true})
				allowed := role == enum.RoleAdministrator && status == enum.UserActive
				if (err == nil) != allowed {
					t.Errorf("key=%s role=%s status=%d authorized=%t err=%v", key, role, status, allowed, err)
				}
			}
		}
	}
	if err := blob.AuthorizeRead(ctx, &query.GetBlobByKey{Key: "files/private", ForBackup: true}); err != blob.ErrNotFound {
		t.Fatalf("anonymous backup read accepted: %v", err)
	}
}
