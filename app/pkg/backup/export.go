package backup

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/lib/pq"
)

func exportTable(ctx context.Context, tableName string) ([]byte, error) {
	trx := ctx.Value(app.TransactionCtxKey).(*dbx.Trx)
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	columnName := "tenant_id"
	if tableName == "tenants" {
		columnName = "id"
	}

	statement := fmt.Sprintf(
		"SELECT * FROM %s WHERE %s = $1",
		pq.QuoteIdentifier(tableName),
		pq.QuoteIdentifier(columnName),
	)

	rows, err := trx.Query(statement, tenant.ID)
	if err != nil {
		return nil, err
	}

	data, err := jsonify(rows)
	if err != nil {
		return nil, err
	}

	return json.Marshal(data)
}

func jsonify(rows *sql.Rows) ([]map[string]any, error) {
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	allResults := make([]map[string]any, 0)

	for rows.Next() {
		results := make(map[string]any)
		values := make([]any, len(columns))
		scanArgs := make([]any, len(values))
		for i := range values {
			scanArgs[i] = &values[i]
		}

		err = rows.Scan(scanArgs...)
		if err != nil {
			return nil, err
		}

		for i, value := range values {
			switch value := value.(type) {
			case nil:
				results[columns[i]] = nil

			case []byte:
				s := string(value)
				x, err := strconv.Atoi(s)

				if err != nil {
					results[columns[i]] = s
				} else {
					results[columns[i]] = x
				}

			default:
				results[columns[i]] = value
			}
		}

		allResults = append(allResults, results)
	}

	return allResults, rows.Err()
}
