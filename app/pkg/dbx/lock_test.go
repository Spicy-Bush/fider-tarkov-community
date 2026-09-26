package dbx_test

import (
	"context"
	"testing"

	. "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/assert"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
)

func TestTryLock_MultipleProcesses_SameKey(t *testing.T) {
	RegisterT(t)

	trx1, _ := dbx.BeginTx(context.Background())
	locked1, err1 := dbx.TryLock(trx1, "KEY_1")
	defer trx1.MustRollback()

	trx2, _ := dbx.BeginTx(context.Background())
	locked2, err2 := dbx.TryLock(trx2, "KEY_1")
	defer trx2.MustRollback()

	Expect(locked1).IsTrue()
	Expect(err1).IsNil()
	Expect(locked2).IsFalse()
	Expect(err2).IsNil()

	Expect(trx1.Commit()).IsNil()

	locked2, err2 = dbx.TryLock(trx2, "KEY_1")
	Expect(locked2).IsTrue()
	Expect(err2).IsNil()

	Expect(trx2.Commit()).IsNil()
}

func TestTryLock_MultipleProcesses_DifferentKey(t *testing.T) {
	RegisterT(t)

	trx1, _ := dbx.BeginTx(context.Background())
	locked1, err1 := dbx.TryLock(trx1, "KEY_1")
	defer trx1.MustRollback()

	trx2, _ := dbx.BeginTx(context.Background())
	locked2, err2 := dbx.TryLock(trx2, "KEY_2")
	defer trx2.MustRollback()

	Expect(locked1).IsTrue()
	Expect(err1).IsNil()
	Expect(locked2).IsTrue()
	Expect(err2).IsNil()

	Expect(trx1.Commit()).IsNil()
	Expect(trx2.Commit()).IsNil()
}
