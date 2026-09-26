package dbx

import (
	"hash/fnv"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
)

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func TryLock(trx *Trx, key string) (bool, error) {
	var locked bool
	if err := trx.Scalar(&locked, "SELECT pg_try_advisory_xact_lock($1)", hash(key)); err != nil {
		return false, errors.Wrap(err, "failed to acquire advisory lock")
	}

	return locked, nil
}