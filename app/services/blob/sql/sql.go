package sql

import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/readlimit"

	"github.com/Spicy-Bush/fider-tarkov-community/app"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/blob"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
)

func init() {
	bus.Register(Service{})
}

type Service struct{}

var backend = blob.Backend{
	Read:         getBlobByKey,
	Write:        storeBlob,
	Remove:       deleteBlob,
	ListKeys:     listBlobs,
	ScanMetadata: scanBlobMetadata,
}

func (s Service) Name() string {
	return "SQL"
}

func (s Service) Category() string {
	return "blobstorage"
}

func (s Service) Enabled() bool {
	return env.Config.BlobStorage.Type == "sql"
}

func (s Service) Init() {
	backend.Register()
}

type dbBlob struct {
	Key         string `db:"key"`
	ContentType string `db:"content_type"`
	Size        int64  `db:"size"`
	Content     []byte `db:"file"`
	TooLarge    bool   `db:"too_large"`
}

func listBlobs(ctx context.Context, q *query.ListBlobs) error {
	prefix := strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(q.Prefix)

	return using(ctx, func(trx *dbx.Trx, tenantID sql.NullInt64) error {
		blobs := []*dbBlob{}
		err := trx.Select(&blobs, `SELECT key FROM blobs WHERE key LIKE $1 ESCAPE '\' AND (tenant_id = $2 OR ($2 IS NULL AND tenant_id IS NULL))`, prefix+"%", tenantID)
		if err != nil {
			return errors.Wrap(err, "failed list blobs")
		}

		files := make([]string, len(blobs))
		for i, b := range blobs {
			files[i] = b.Key
		}

		sort.Strings(files)
		q.Result = files

		return nil
	})
}

func getBlobByKey(ctx context.Context, q *query.GetBlobByKey) error {
	return using(ctx, func(trx *dbx.Trx, tenantID sql.NullInt64) error {
		b := dbBlob{}
		err := trx.Get(&b, `
			SELECT content_type, size,
			    $3>0 AND (size>$3 OR octet_length(file)>$3) AS too_large,
			    CASE WHEN $3>0 AND (size>$3 OR octet_length(file)>$3) THEN NULL ELSE file END AS file
			FROM blobs WHERE key=$1 AND (tenant_id=$2 OR ($2 IS NULL AND tenant_id IS NULL))
		`, q.Key, tenantID, q.MaxBytes)
		if err != nil {
			if err == app.ErrNotFound {
				return blob.ErrNotFound
			}
			return errors.Wrap(err, "failed to get blob with key '%s'", q.Key)
		}
		if b.TooLarge {
			return readlimit.ErrTooLarge
		}

		q.Result = &dto.Blob{
			Size:        b.Size,
			ContentType: b.ContentType,
			Content:     b.Content,
		}

		return nil
	})
}

func storeBlob(ctx context.Context, c *cmd.StoreBlob) error {
	return using(ctx, func(trx *dbx.Trx, tenantID sql.NullInt64) error {
		now := time.Now()
		_, err := trx.Execute(`
		INSERT INTO blobs (tenant_id, key, size, content_type, file, created_at, modified_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (tenant_id, key)
		DO UPDATE SET size = $3, content_type = $4, file = $5, modified_at = $7
		`, tenantID, c.Key, int64(len(c.Content)), c.ContentType, c.Content, now, now)
		if err != nil {
			return errors.Wrap(err, "failed to store blob with key '%s'", c.Key)
		}

		return nil
	})
}

func deleteBlob(ctx context.Context, c *cmd.DeleteBlob) error {
	return using(ctx, func(trx *dbx.Trx, tenantID sql.NullInt64) error {
		_, err := trx.Execute("DELETE FROM blobs WHERE key = $1 AND (tenant_id = $2 OR ($2 IS NULL AND tenant_id IS NULL))", c.Key, tenantID)
		if err != nil {
			return errors.Wrap(err, "failed to delete blob with key '%s'", c.Key)
		}

		return nil
	})
}

func using(ctx context.Context, handler func(*dbx.Trx, sql.NullInt64) error) error {
	var tenantID sql.NullInt64
	tenant, ok := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	if ok {
		_ = tenantID.Scan(tenant.ID)
	}
	return dbx.InTransaction(ctx, func(_ context.Context, trx *dbx.Trx) error {
		return handler(trx, tenantID)
	})
}
