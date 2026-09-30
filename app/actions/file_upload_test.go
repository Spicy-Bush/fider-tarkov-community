package actions

import "testing"

func TestFileUploadIdentity(t *testing.T) {
	identity := NewFileUploadID(1, 2)
	if !validFileUploadID(identity, 1, 2) {
		t.Fatal("issued identity was rejected")
	}

	if identity == NewFileUploadID(1, 2) {
		t.Fatal("separate uploads received the same identity")
	}

	for _, item := range []struct {
		name     string
		identity string
		tenantID int
		userID   int
	}{
		{"another account", identity, 1, 3},
		{"another tenant", identity, 3, 2},
		{"changed nonce", "!" + identity[1:], 1, 2},
		{"missing signature", identity[:32], 1, 2},
		{"empty", "", 1, 2},
	} {
		t.Run(item.name, func(t *testing.T) {
			if validFileUploadID(item.identity, item.tenantID, item.userID) {
				t.Fatal("identity did not belong to this upload account")
			}
		})
	}
}
