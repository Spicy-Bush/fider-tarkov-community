package dbx

import (
	"context"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
)

func InTransaction(ctx context.Context, operation func(context.Context, *Trx) error) error {
	trx, owned, err := GetOrBeginTx(ctx)
	if err != nil {
		return err
	}
	if !owned {
		return operation(ctx, trx)
	}
	defer trx.Rollback()

	ctx = context.WithValue(ctx, app.TransactionCtxKey, trx)
	if err := operation(ctx, trx); err != nil {
		return trx.RollbackWithCause(err)
	}
	return trx.Commit()
}

func (trx *Trx) RollbackWithCause(cause error) error {
	if cleanup := trx.Rollback(); cleanup != nil {
		return errors.Wrap(cause, "rollback also failed: %v", cleanup)
	}
	return cause
}