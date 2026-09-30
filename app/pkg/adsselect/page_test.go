package adsselect

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSponsorPageBindsSessionTenantAndExpiry(t *testing.T) {
	now := time.Now()
	page := Page{ID: strings.Repeat("a", 32), TenantID: 12, Session: SessionKey("browser"), Kind: "home", PostIDs: []int{10, 20}, Expires: now.Add(time.Hour).Unix()}
	token, err := page.Token("secret")
	if err != nil {
		t.Fatal(err)
	}

	read, err := ReadPage(token, "secret", 12, "browser", now)
	if err != nil || !reflect.DeepEqual(*read, page) {
		t.Fatalf("page changed: %+v, %v", read, err)
	}

	for _, test := range []struct {
		token   string
		tenant  int
		session string
		now     time.Time
	}{
		{token, 13, "browser", now},
		{token, 12, "other browser", now},
		{token, 12, "browser", now.Add(time.Hour)},
		{"A" + token[1:], 12, "browser", now},
	} {
		if _, err := ReadPage(test.token, "secret", test.tenant, test.session, test.now); err == nil {
			t.Fatal("accepted an invalid page token")
		}
	}
}
