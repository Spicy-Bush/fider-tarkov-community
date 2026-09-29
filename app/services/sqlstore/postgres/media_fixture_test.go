package postgres_test

import (
	"context"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/sqlstore/postgres"
)

func mediaFixtureSQL(statement string, args ...any) (int64, error) {
	trx, err := mediaFixtureTransaction(context.Background())
	if err != nil {
		return 0, err
	}
	defer trx.Rollback()

	changed, err := trx.Execute(statement, args...)
	if err != nil {
		return 0, err
	}
	return changed, trx.Commit()
}

func mediaFixtureScalar(target any, statement string, args ...any) error {
	trx, err := mediaFixtureTransaction(context.Background())
	if err != nil {
		return err
	}
	defer trx.Rollback()

	if err := trx.Scalar(target, statement, args...); err != nil {
		return err
	}
	return trx.Commit()
}

func mediaFixtureTransaction(ctx context.Context) (*dbx.Trx, error) {
	trx, err := dbx.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	trx.BeforeCommit = postgres.CompleteMediaReferencesForTest
	return trx, nil
}
