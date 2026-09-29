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

	// A cursor avoids rescanning the source view for each batch
	if _, err := trx.Execute(`
		DECLARE media_reference_backfill NO SCROLL CURSOR FOR
		SELECT tenant_id, kind, owner_id, source::text FROM media_reference_sources
	`); err != nil {
		return err
	}

	for {
		var sources []*mediaReferenceSource
		if err := trx.Select(&sources, `FETCH FORWARD 100 FROM media_reference_backfill`); err != nil {
			return err
		}

		if len(sources) == 0 {
			_, err := trx.Execute(`CLOSE media_reference_backfill`)
			return err
		}

		if err := saveMediaReferences(trx, sources); err != nil {
			return err
		}
	}
}
