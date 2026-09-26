package email

import "errors"

// RecipientRejected identifies a permanent refusal of one or more recipients.
type RecipientRejected struct {
	Cause error
}

func (e *RecipientRejected) Error() string { return e.Cause.Error() }
func (e *RecipientRejected) Unwrap() error { return e.Cause }

func IsRecipientRejected(err error) bool {
	var rejected *RecipientRejected
	return errors.As(err, &rejected)
}