package adsselect

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
)

type Page struct {
	ID        string `json:"id"`
	TenantID  int    `json:"tenant"`
	Session   string `json:"session"`
	Kind      string `json:"kind"`
	ContentID int    `json:"content"`
	PostIDs   []int  `json:"posts,omitempty"`
	Expires   int64  `json:"expires"`
}

func SessionKey(session string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(session)))
}

func (page Page) Token(secret string) (string, error) {
	return signToken("sponsor-page:", page, secret)
}

func ReadPage(token, secret string, tenantID int, session string, now time.Time) (*Page, error) {
	payload, err := readToken("sponsor-page:", token, secret)
	if err != nil {
		return nil, err
	}

	var page Page
	if err := json.Unmarshal(payload, &page); err != nil {
		return nil, err
	}
	if page.TenantID != tenantID || page.Session != SessionKey(session) || page.Expires <= now.Unix() || len(page.ID) != 32 {
		return nil, fmt.Errorf("invalid sponsorship page")
	}

	return &page, nil
}
