package backup

import (
	"context"
	"errors"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/lib/pq"
)

func TestExportTableReturnsRowStreamFailure(t *testing.T) {
	ctx := context.Background()
	trx, err := dbx.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}

	defer trx.Rollback()
	ctx = context.WithValue(ctx, app.TransactionCtxKey, trx)
	ctx = context.WithValue(ctx, app.TenantCtxKey, &entity.Tenant{ID: 1})

	_, err = trx.Execute(`CREATE TEMPORARY VIEW backup_row_error AS
		SELECT 1 AS tenant_id, i, 1 / (3 - i) AS value
		FROM generate_series(1, 3) i`)
	if err != nil {
		t.Fatal(err)
	}

	data, err := exportTable(ctx, "backup_row_error")
	var queryError *pq.Error
	if !errors.As(err, &queryError) || queryError.Code != "22012" {
		t.Fatalf("row-stream failure was lost: %v", err)
	}

	if len(data) != 0 {
		t.Fatalf("failed export returned partial JSON: %s", data)
	}
}
