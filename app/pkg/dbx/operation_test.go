package dbx_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
)

func TestOperationRetainsQueryFailureWhenConnectionDies(t *testing.T) {
	var primary error
	err := dbx.InTransaction(context.Background(), func(_ context.Context, trx *dbx.Trx) error {
		var pid int
		if err := trx.Scalar(&pid, "SELECT pg_backend_pid()"); err != nil {
			return err
		}
		if _, err := dbx.Connection().Exec("SELECT pg_terminate_backend($1)", pid); err != nil {
			return err
		}
		var value int
		primary = trx.Scalar(&value, "SELECT 1")
		return primary
	})
	if primary == nil || !errors.Is(err, errors.Unwrap(primary)) || !strings.Contains(err.Error(), "trx.Scalar") {
		t.Fatalf("query failure was masked by cleanup: primary=%v returned=%v", primary, err)
	}
}

func TestOperationCannotCommitAnAlreadyRolledBackTransaction(t *testing.T) {
	err := dbx.InTransaction(context.Background(), func(_ context.Context, trx *dbx.Trx) error {
		return trx.Rollback()
	})
	if err == nil {
		t.Fatal("rolled-back operation reported a successful commit")
	}
}

func TestOperationRowStreamErrorsAreNotEmptySuccess(t *testing.T) {
	for _, operation := range []string{"get", "exists", "count", "select"} {
		t.Run(operation, func(t *testing.T) {
			err := dbx.InTransaction(context.Background(), func(_ context.Context, trx *dbx.Trx) error {
				query := "SELECT i AS id, 1 / (3 - i) AS value FROM generate_series(1, 3) i"
				var row struct {
					ID    int `db:"id"`
					Value int `db:"value"`
				}
				switch operation {
				case "get":
					return trx.Get(&row, query)
				case "exists":
					_, err := trx.Exists(query)
					return err
				case "count":
					_, err := trx.Count(query)
					return err
				default:
					var rows []*struct {
						ID    int `db:"id"`
						Value int `db:"value"`
					}
					return trx.Select(&rows, query)
				}
			})
			if err == nil || !strings.Contains(err.Error(), "division by zero") {
				t.Fatalf("database failure became a successful or empty result: %v", err)
			}
		})
	}
}
