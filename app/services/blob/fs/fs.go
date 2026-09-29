package fs

import (
	"context"
	"io/fs"
	"net/http"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/rand"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/readlimit"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/blob"
)

const perm os.FileMode = 0744

// The space keeps staging outside the public blob key namespace.
const stagingDirectory = ".fider staging"

var backend = blob.Backend{
	Read:         getBlobByKey,
	Write:        storeBlob,
	Remove:       deleteBlob,
	ListKeys:     listBlobs,
	ScanMetadata: scanBlobMetadata,
}

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
	backend.Register()
}

func listBlobs(ctx context.Context, q *query.ListBlobs) error {
	root, err := openRoot(ctx, false)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer root.Close()

	files := make([]string, 0)
	start := path.Dir(q.Prefix)
	err = fs.WalkDir(root.FS(), start, func(key string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == stagingDirectory || (ctx.Value(app.TenantCtxKey) == nil && key == "tenants") {
				return fs.SkipDir
			}
		}
		if entry.Type().IsRegular() && strings.HasPrefix(key, q.Prefix) {
			files = append(files, key)
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to list blobs from FileSystem")
	}

	sort.Strings(files)
	q.Result = files
	return nil
}

func getBlobByKey(ctx context.Context, q *query.GetBlobByKey) error {
	root, err := openRoot(ctx, false)
	if os.IsNotExist(err) {
		return blob.ErrNotFound
	}
	if err != nil {
		return err
	}
	defer root.Close()

	file, err := root.Open(q.Key)
	if os.IsNotExist(err) {
		return blob.ErrNotFound
	}
	if err != nil {
		return errors.Wrap(err, "failed to open blob '%s' from FileSystem", q.Key)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return blob.ErrNotFound
	}
	if q.MaxBytes > 0 && info.Size() > q.MaxBytes {
		return readlimit.ErrTooLarge
	}
	content, err := readlimit.ReadAll(file, q.MaxBytes)
	if err != nil {
		return errors.Wrap(err, "failed to read blob '%s' from FileSystem", q.Key)
	}

	q.Result = &dto.Blob{
		Content:     content,
		ContentType: http.DetectContentType(content),
		Size:        int64(len(content)),
	}
	return nil
}

func storeBlob(ctx context.Context, c *cmd.StoreBlob) error {
	root, err := openRoot(ctx, true)
	if err != nil {
		return err
	}
	defer root.Close()

	if err := root.MkdirAll(path.Dir(c.Key), perm); err != nil {
		return errors.Wrap(err, "failed to create blob directory on FileSystem")
	}
	staging := path.Join(path.Dir(c.Key), stagingDirectory)
	if err := root.MkdirAll(staging, 0700); err != nil {
		return errors.Wrap(err, "failed to create blob staging directory on FileSystem")
	}
	temporaryKey := path.Join(staging, rand.String(32))
	file, err := root.OpenFile(temporaryKey, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return errors.Wrap(err, "failed to prepare blob '%s' on FileSystem", c.Key)
	}
	defer root.Remove(temporaryKey)

	_, writeErr := file.Write(c.Content)
	closeErr := file.Close()
	if writeErr != nil {
		return errors.Wrap(writeErr, "failed to write blob '%s' on FileSystem", c.Key)
	}
	if closeErr != nil {
		return errors.Wrap(closeErr, "failed to close blob '%s' on FileSystem", c.Key)
	}

	if err := root.Rename(temporaryKey, c.Key); err != nil {
		return errors.Wrap(err, "failed to write blob '%s' on FileSystem", c.Key)
	}
	return nil
}

func deleteBlob(ctx context.Context, c *cmd.DeleteBlob) error {
	root, err := openRoot(ctx, false)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer root.Close()

	if err := root.Remove(c.Key); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to delete blob '%s' from FileSystem", c.Key)
	}
	return nil
}

func openRoot(ctx context.Context, create bool) (*os.Root, error) {
	if create {
		if err := os.MkdirAll(env.Config.BlobStorage.FS.Path, perm); err != nil {
			return nil, err
		}
	}
	root, err := os.OpenRoot(env.Config.BlobStorage.FS.Path)
	if err != nil {
		return nil, err
	}
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	if tenant == nil {
		return root, nil
	}

	for _, directory := range []string{"tenants", strconv.Itoa(tenant.ID)} {
		if create {
			if err := root.Mkdir(directory, perm); err != nil && !os.IsExist(err) {
				root.Close()
				return nil, err
			}
		}
		info, err := root.Lstat(directory)
		if err != nil {
			root.Close()
			return nil, err
		}
		if !info.IsDir() {
			root.Close()
			return nil, blob.ErrInvalidKeyFormat
		}
		next, err := root.OpenRoot(directory)
		root.Close()
		if err != nil {
			return nil, err
		}
		root = next
	}
	return root, nil
}
