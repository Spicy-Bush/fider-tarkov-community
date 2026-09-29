package backup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/lib/pq"
)

func TestExportPreservesBinaryAndTextColumns(t *testing.T) {
	ctx := context.WithValue(context.Background(), app.TenantCtxKey, &entity.Tenant{ID: 1})
	trx, err := dbx.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer trx.Rollback()
	ctx = context.WithValue(ctx, app.TransactionCtxKey, trx)

	_, err = trx.Execute(`CREATE TEMPORARY TABLE backup_binary (
		tenant_id integer, id integer, document bytea, title text, quantity numeric
	)`)
	if err != nil {
		t.Fatal(err)
	}

	documents := [][]byte{{0, 255, 195, 40}, []byte("00123"), {}, nil}
	for id, document := range documents {
		var columnValue any = document
		if document == nil {
			columnValue = nil
		}

		_, err := trx.Execute(`
			INSERT INTO backup_binary (tenant_id, id, document, title, quantity)
			VALUES (1, $1, $2, '00123 café', 7)
		`, id, columnValue)
		if err != nil {
			t.Fatal(err)
		}
	}

	data, err := exportTable(ctx, "backup_binary")
	if err != nil {
		t.Fatal(err)
	}

	var restored []struct {
		ID       int    `json:"id"`
		Document []byte `json:"document"`
		Title    string `json:"title"`
		Quantity int    `json:"quantity"`
	}
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("backup binary data cannot be decoded: %v", err)
	}
	if len(restored) != len(documents) {
		t.Fatalf("exported %d rows, want %d", len(restored), len(documents))
	}

	for _, row := range restored {
		expected := documents[row.ID]
		if !bytes.Equal(row.Document, expected) || (row.Document == nil) != (expected == nil) {
			t.Errorf("binary column changed for row %d: got %x, want %x", row.ID, row.Document, expected)
		}
		if row.Title != "00123 café" || row.Quantity != 7 {
			t.Errorf("text or numeric column changed: %+v", row)
		}
	}
}

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
