package postgres

import (
	"encoding/json"
	"fmt"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

type commandReceipt struct {
	TenantID     int
	UserID       int
	Kind         string
	SubmissionID string
	Fingerprint  string
}

func (receipt commandReceipt) read(trx *dbx.Trx, result any) (bool, error) {
	if !validate.ValidSubmissionID(receipt.SubmissionID) {
		return false, validate.Failed("Invalid submission identity.")
	}

	identity := fmt.Sprintf("command:%s:%d:%d:%s", receipt.Kind, receipt.TenantID, receipt.UserID, receipt.SubmissionID)
	if _, err := trx.Execute("SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", identity); err != nil {
		return false, err
	}

	var stored struct {
		Fingerprint string `db:"fingerprint"`
		Result      string `db:"result"`
	}
	err := trx.Get(&stored, `
		SELECT fingerprint, result::text FROM command_receipts
		WHERE tenant_id=$1 AND user_id=$2 AND kind=$3 AND submission_id=$4
	`, receipt.TenantID, receipt.UserID, receipt.Kind, receipt.SubmissionID)
	if err == app.ErrNotFound {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	if stored.Fingerprint != receipt.Fingerprint {
		return false, app.ErrConflict
	}

	if result == nil {
		return true, nil
	}

	return true, json.Unmarshal([]byte(stored.Result), result)
}

func (receipt commandReceipt) save(trx *dbx.Trx, result any) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}

	_, err = trx.Execute(`
		INSERT INTO command_receipts (tenant_id, user_id, kind, submission_id, fingerprint, result)
		VALUES ($1,$2,$3,$4,$5,$6)
	`, receipt.TenantID, receipt.UserID, receipt.Kind, receipt.SubmissionID, receipt.Fingerprint, encoded)
	return err
}
