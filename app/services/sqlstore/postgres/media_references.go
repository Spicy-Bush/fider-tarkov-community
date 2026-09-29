package postgres

import (
	"context"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
)

type mediaReferenceRemoval struct {
	Force          bool
	IncludeDeleted bool
	IncludeDrafts  bool
}

func unlinkMediaAssetReferences(ctx context.Context, trx *dbx.Trx, tenantID int, key string, removal mediaReferenceRemoval) ([]*dto.FileReference, error) {
	if err := flushMediaReferences(trx); err != nil {
		return nil, err
	}
	if _, err := trx.Execute("SELECT lock_media_reference_owners($1,$2)", tenantID, key); err != nil {
		return nil, err
	}
	if _, err := trx.Execute(`SELECT key FROM media_assets WHERE tenant_id=$1 AND key=$2 FOR UPDATE`, tenantID, key); err != nil {
		return nil, err
	}

	blocked := []*dto.FileReference{}
	err := trx.Select(&blocked, `
		SELECT kind, id, field, scope
		FROM media_references
		WHERE tenant_id=$1 AND key=$2 AND media_reference_blocks(scope,$3,$4,$5)
		ORDER BY kind, id, field LIMIT 50
	`, tenantID, key, removal.Force, removal.IncludeDeleted, removal.IncludeDrafts)
	if err != nil || len(blocked) > 0 {
		return blocked, err
	}
	if err := unlinkPageEditBanners(trx, tenantID, key); err != nil {
		return nil, err
	}

	if _, err := trx.Execute(`SELECT unlink_media_reference_fields($1,$2,$3,$4,$5)`,
		tenantID, key, removal.Force, removal.IncludeDeleted, removal.IncludeDrafts); err != nil {
		return nil, err
	}

	return nil, flushMediaReferences(trx)
}
