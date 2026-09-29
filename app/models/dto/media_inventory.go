package dto

import "time"

type BlobMetadata struct {
	Key         string    `db:"key"`
	ContentType string    `db:"content_type"`
	Size        int64     `db:"size"`
	ModifiedAt  time.Time `db:"modified_at"`
}

type MediaInventory struct {
	State     string `json:"state"`
	Scanned   int64  `json:"scanned"`
	Skipped   int64  `json:"skipped"`
	LastError string `json:"lastError,omitempty"`
}
