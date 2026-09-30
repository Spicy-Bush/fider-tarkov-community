package backup

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
	"github.com/lib/pq"
)

func Create(ctx context.Context) (*bytes.Buffer, error) {
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	user, _ := ctx.Value(app.UserCtxKey).(*entity.User)
	if !entity.Can(user, tenant, entity.ExportBackup) {
		return nil, validate.Unauthorized()
	}

	trx, err := dbx.BeginTxWithOptions(ctx, &sql.TxOptions{
		Isolation: sql.LevelRepeatableRead,
		ReadOnly:  true,
	})
	if err != nil {
		return nil, err
	}

	defer trx.Rollback()
	ctx = context.WithValue(ctx, app.TransactionCtxKey, trx)

	buffer := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buffer)

	excluded := []string{
		"blobs", "media_thumbnails", "media_inventory", "media_inventory_candidates",
		"media_reference_changes",
		"sponsor_opportunities",
	}
	indirect := []string{"tenants", "reactions", "page_reactions", "page_subscriptions"}
	var tables []*struct {
		Name string `db:"table_name"`
	}
	if err := trx.Select(&tables, `
		SELECT columns.table_name::text
		FROM information_schema.columns
		JOIN information_schema.tables USING (table_schema, table_name)
		WHERE columns.table_schema = current_schema()
		  AND tables.table_type = 'BASE TABLE'
		  AND columns.column_name = 'tenant_id'
		  AND columns.table_name <> ALL($1)
		UNION SELECT unnest($2::text[])
		ORDER BY 1
	`, pq.Array(excluded), pq.Array(indirect)); err != nil {
		return nil, err
	}

	for _, table := range tables {
		err := addTableDataToZipFile(ctx, zipWriter, table.Name)
		if err != nil {
			return nil, err
		}
	}

	listBlobs := &query.ListBlobs{}
	if err := bus.Dispatch(ctx, listBlobs); err != nil {
		return nil, err
	}

	for _, bkey := range listBlobs.Result {
		err := addBlobToZipFile(ctx, zipWriter, bkey)
		if err != nil {
			return nil, err
		}
	}
	if listBlobs.Skipped > 0 {
		warning, err := zipWriter.Create("warnings.json")
		if err != nil {
			return nil, err
		}
		if err := json.NewEncoder(warning).Encode(map[string]any{
			"skippedBlobs": listBlobs.Skipped,
			"reason":       "Stored files with invalid names could not be exported.",
		}); err != nil {
			return nil, err
		}
	}

	err = zipWriter.Close()
	if err != nil {
		return nil, errors.Wrap(err, "failed to close zip file")
	}

	if err := trx.Commit(); err != nil {
		return nil, err
	}

	return buffer, nil
}

func addBlobToZipFile(ctx context.Context, zipWriter *zip.Writer, bkey string) error {
	getBlob := &query.GetBlobByKey{Key: bkey, ForBackup: true}
	if err := bus.Dispatch(ctx, getBlob); err != nil {
		return errors.Wrap(err, "failed to get blob with key %s", bkey)
	}

	fileName := fmt.Sprintf("blobs/%s", bkey)
	fileWriter, err := zipWriter.Create(fileName)
	if err != nil {
		return errors.Wrap(err, "failed to create %s in zip file", fileName)
	}
	_, err = fileWriter.Write(getBlob.Result.Content)
	if err != nil {
		return errors.Wrap(err, "failed to write %s to zip file", fileName)
	}

	return nil
}

func addTableDataToZipFile(ctx context.Context, zipWriter *zip.Writer, tableName string) error {
	tableData, err := exportTable(ctx, tableName)
	if err != nil {
		return errors.Wrap(err, "failed to export %s table", tableName)
	}

	fileWriter, err := zipWriter.Create(fmt.Sprintf("%s.json", tableName))
	if err != nil {
		return errors.Wrap(err, "failed to create %s.json in zip file", tableName)
	}
	_, err = fileWriter.Write(tableData)
	if err != nil {
		return errors.Wrap(err, "failed to write %s.json to zip file", tableName)
	}

	return nil
}
