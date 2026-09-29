package postgres

import (
	"context"
	"encoding/json"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
)

type mediaReferenceSource struct {
	TenantID   int                       `db:"tenant_id" json:"tenant_id"`
	Kind       string                    `db:"kind" json:"kind"`
	OwnerID    int                       `db:"owner_id" json:"owner_id"`
	Source     string                    `db:"source" json:"-"`
	References []extractedMediaReference `json:"refs"`
}

func flushMediaReferences(trx *dbx.Trx) error {
	var sources []*mediaReferenceSource
	if err := trx.Select(&sources, `
		SELECT tenant_id, kind, owner_id, source::text
		FROM media_reference_changes WHERE transaction_id=txid_current_if_assigned()
	`); err != nil {
		return err
	}
	if len(sources) == 0 {
		return nil
	}

	return saveMediaReferences(trx, sources)
}

func saveMediaReferences(trx *dbx.Trx, sources []*mediaReferenceSource) error {
	for index := range sources {
		references, err := extractMediaReferences(sources[index].Kind, sources[index].Source)
		if err != nil {
			return err
		}
		sources[index].References = references
	}
	return writeMediaReferences(trx, sources)
}

func writeMediaReferences(trx *dbx.Trx, sources []*mediaReferenceSource) error {
	content, err := json.Marshal(sources)
	if err != nil {
		return err
	}

	_, err = trx.Execute(`SELECT replace_media_references($1::jsonb)`, string(content))
	return err
}

// Existing content must be protected before the new catalog becomes visible.
func BackfillMediaReferences(ctx context.Context, trx *dbx.Trx, version int) error {
	if version != 202609281700 {
		return nil
	}
	trx.BeforeCommit = flushMediaReferences

	var kinds []*struct {
		Kind string `db:"kind"`
	}
	if err := trx.Select(&kinds, `SELECT DISTINCT kind FROM media_reference_sources ORDER BY kind`); err != nil {
		return err
	}

	for _, kind := range kinds {
		var tenantID, ownerID int
		for {
			var sources []*mediaReferenceSource
			if err := trx.Select(&sources, `
				SELECT tenant_id, kind, owner_id, source::text
				FROM media_reference_sources
				WHERE kind=$1 AND (tenant_id, owner_id)>($2, $3)
				ORDER BY tenant_id, owner_id LIMIT 100
			`, kind.Kind, tenantID, ownerID); err != nil {
				return err
			}
			if len(sources) == 0 {
				break
			}

			if err := saveMediaReferences(trx, sources); err != nil {
				return err
			}
			last := sources[len(sources)-1]
			tenantID, ownerID = last.TenantID, last.OwnerID
		}
	}

	return nil
}
