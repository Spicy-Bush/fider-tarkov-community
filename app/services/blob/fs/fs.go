package fs

import (
	"context"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Spicy-Bush/fider-tarkov-community/app"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/blob"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
)

var perm os.FileMode = 0744

func init() {
	bus.Register(Service{})
}

type Service struct{}

func (s Service) Name() string {
	return "FileSystem"
}

func (s Service) Category() string {
	return "blobstorage"
}

func (s Service) Enabled() bool {
	return env.Config.BlobStorage.Type == "fs"
}

func (s Service) Init() {
	bus.AddHandler(listBlobs)
	bus.AddHandler(getBlobByKey)
	bus.AddHandler(storeBlob)
	bus.AddHandler(deleteBlob)
}

func listBlobs(ctx context.Context, q *query.ListBlobs) error {
	basePath := basePath(ctx, q.Prefix)
	files := make([]string, 0)

	err := filepath.Walk(basePath,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				files = append(files, q.Prefix+path[len(basePath)+1:])
			}
			return nil
		})
	if err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to read dir '%s'", basePath)
	}

	sort.Strings(files)
	q.Result = files
	return nil
}

func getBlobByKey(ctx context.Context, q *query.GetBlobByKey) error {
	// see: filemanagement page
	if strings.HasPrefix(q.Key, "files/") {
		user, ok := ctx.Value(app.UserCtxKey).(*entity.User)
		if !ok || user == nil || !user.IsAdministrator() {
			return blob.ErrNotFound
		}
	}
	fullPath := keyFullPath(ctx, q.Key)
	stats, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return blob.ErrNotFound
		}
		return errors.Wrap(err, "failed to get stats '%s' from FileSystem", q.Key)
	}

	file, err := os.ReadFile(fullPath)
	if err != nil {
		return errors.Wrap(err, "failed to get read '%s' from FileSystem", q.Key)
	}

	q.Result = &dto.Blob{
		Content:     file,
		ContentType: http.DetectContentType(file),
		Size:        stats.Size(),
	}
	return nil
}

func storeBlob(ctx context.Context, c *cmd.StoreBlob) error {
	if err := blob.ValidateKey(c.Key); err != nil {
		return errors.Wrap(err, "failed to validate blob key '%s'", c.Key)
	}

	fullPath := keyFullPath(ctx, c.Key)
	err := os.MkdirAll(filepath.Dir(fullPath), perm)

	if err != nil {
		return errors.Wrap(err, "failed to create folder '%s' on FileSystem", fullPath)
	}

	err = os.WriteFile(fullPath, c.Content, perm)
	if err != nil {
		return errors.Wrap(err, "failed to create file '%s' on FileSystem", fullPath)
	}

	return nil
}

func deleteBlob(ctx context.Context, c *cmd.DeleteBlob) error {
	fullPath := keyFullPath(ctx, c.Key)
	err := os.Remove(fullPath)
	if err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to delete file '%s' from FileSystem", c.Key)
	}
	return nil
}

func keyFullPath(ctx context.Context, key string) string {
	return basePath(ctx, key)
}

func basePath(ctx context.Context, segment string) string {
	blob.EnsureAuthorizedPrefix(ctx, segment)

	startPath := env.Config.BlobStorage.FS.Path
	tenant, ok := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	if ok {
		return path.Join(startPath, "tenants", strconv.Itoa(tenant.ID), segment)
	}
	return path.Join(startPath, segment)
}
