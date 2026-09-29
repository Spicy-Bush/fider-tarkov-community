package blob

import (
	"context"
	"errors"
	"path"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/gosimple/slug"
)

// ErrNotFound is returned when given blob is not found
var ErrNotFound = errors.New("Blob not found")

// ErrInvalidKeyFormat is returned when blob key is in invalid format
var ErrInvalidKeyFormat = errors.New("Blob key is in invalid format")

// SanitizeFileName replaces invalid characters from given filename
func SanitizeFileName(fileName string) string {
	fileName = strings.TrimSpace(fileName)
	ext := filepath.Ext(fileName)
	if ext != "" {
		return slug.Make(fileName[0:len(fileName)-len(ext)]) + ext
	}
	return slug.Make(fileName)
}

func ValidateKey(key string) error {
	if len(key) == 0 || len(key) > 512 || !utf8.ValidString(key) {
		return ErrInvalidKeyFormat
	}
	if key == "." || key == ".." || strings.HasPrefix(key, "/") || strings.HasPrefix(key, "../") || path.Clean(key) != key {
		return ErrInvalidKeyFormat
	}
	for _, character := range key {
		if character == '\\' || unicode.IsSpace(character) || unicode.IsControl(character) {
			return ErrInvalidKeyFormat
		}
	}
	return nil
}

func ValidatePrefix(prefix string) error {
	if prefix == "" {
		return nil
	}
	return ValidateKey(strings.TrimSuffix(prefix, "/"))
}

// EnsureAuthorizedPrefix panics if the path is invalid under the given context
func EnsureAuthorizedPrefix(ctx context.Context, path string) {
	// if it's running under the context of a tenant, any prefix is valid
	if ctx.Value(app.TenantCtxKey) != nil {
		return
	}

	// 'tenants' prefix is not valid when running outside a tenant context
	if path == "tenants" || strings.HasPrefix(path, "tenants/") {
		panic(errors.New("Unauthorized access to 'tenants' path."))
	}
}

// trusted callers can read a proposal after getting ownership / authority
func AuthorizeRead(ctx context.Context, q *query.GetBlobByKey) error {
	if err := ValidateKey(q.Key); err != nil {
		return ErrNotFound
	}
	EnsureAuthorizedPrefix(ctx, q.Key)
	if q.ForBackup {
		user, _ := ctx.Value(app.UserCtxKey).(*entity.User)
		tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
		if tenant == nil || !entity.Can(user, tenant, entity.ExportBackup) {
			return ErrNotFound
		}
		return nil
	}
	if strings.HasPrefix(q.Key, "files/") {
		user, _ := ctx.Value(app.UserCtxKey).(*entity.User)
		tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
		if !entity.Can(user, tenant, entity.ManageFiles) {
			return ErrNotFound
		}
	}
	if !strings.HasPrefix(q.Key, "avatars/") || q.AllowUnpublishedAvatar {
		return nil
	}
	published := &query.IsAvatarPublished{Key: q.Key}
	if err := bus.Dispatch(ctx, published); err != nil {
		return err
	}
	if !published.Result {
		return ErrNotFound
	}
	return nil
}
