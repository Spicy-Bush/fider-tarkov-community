package sql

import (
	"context"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
)

func scanBlobMetadata(ctx context.Context, q *query.ScanBlobMetadata) error {
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	cursor := q.Cursor
	for {
		var stored []*dto.BlobMetadata
		err := dbx.InTransaction(ctx, func(ctx context.Context, trx *dbx.Trx) error {
			return trx.Select(&stored, `
				SELECT key, size, content_type, modified_at
				FROM blobs WHERE tenant_id=$1 AND key>$2
				ORDER BY key LIMIT $3
			`, tenant.ID, cursor, q.BatchSize)
		})
		if err != nil {
			return err
		}
		batch := make([]dto.BlobMetadata, len(stored))
		for index, file := range stored {
			batch[index] = *file
		}
		if len(batch) > 0 {
			cursor = batch[len(batch)-1].Key
		}
		complete := len(batch) < q.BatchSize
		if err := q.Accept(batch, cursor, complete); err != nil {
			return err
		}
		if complete {
			return nil
		}
	}
}
