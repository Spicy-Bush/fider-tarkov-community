package adsselect

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"
)

type Click struct {
	OpportunityID string `json:"opportunity"`
	TenantID      int    `json:"tenant"`
	Destination   string `json:"destination"`
	Expires       int64  `json:"expires"`
}

func (click Click) Token(secret string) (string, error) {
	return signToken("sponsor-click:", click, secret)
}

func signToken(purpose string, value any, secret string) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}

	payload := base64.RawURLEncoding.EncodeToString(encoded)
	signature := hmac.New(sha256.New, []byte(secret))
	signature.Write([]byte(purpose + payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(signature.Sum(nil)), nil
}

func readToken(purpose, token, secret string) ([]byte, error) {
	invalid := errors.New("invalid sponsorship token")
	payload, supplied, ok := strings.Cut(token, ".")
	if !ok || len(token) > 8192 {
		return nil, invalid
	}

	signature, err := base64.RawURLEncoding.DecodeString(supplied)
	if err != nil {
		return nil, invalid
	}

	expected := hmac.New(sha256.New, []byte(secret))
	expected.Write([]byte(purpose + payload))
	if !hmac.Equal(signature, expected.Sum(nil)) {
		return nil, invalid
	}

	encoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, invalid
	}

	return encoded, nil
}

func ReadClick(token, secret string, tenantID int, now time.Time) (*Click, error) {
	invalid := errors.New("invalid sponsorship link")
	encoded, err := readToken("sponsor-click:", token, secret)
	if err != nil {
		return nil, err
	}

	var click Click
	if err := json.Unmarshal(encoded, &click); err != nil {
		return nil, invalid
	}

	destination, err := url.Parse(click.Destination)
	if err != nil || destination.Host == "" || (destination.Scheme != "https" && destination.Scheme != "http") {
		return nil, invalid
	}

	if click.TenantID != tenantID || click.Expires <= now.Unix() || len(click.OpportunityID) != 32 {
		return nil, invalid
	}

	return &click, nil
}
