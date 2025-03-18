package dto

import (
	"time"
)

type FileInfo struct {
	Name        string    `json:"name"`
	BlobKey     string    `json:"blobKey"`
	Size        int64     `json:"size"`
	ContentType string    `json:"contentType"`
	CreatedAt   time.Time `json:"createdAt"`
	IsInUse     bool      `json:"isInUse"`
	UsedIn      []string  `json:"usedIn,omitempty"`
}
