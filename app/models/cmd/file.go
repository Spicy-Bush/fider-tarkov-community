package cmd

import (
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

type UploadImageFile struct {
	Name         string
	Content      []byte
	Type         enum.FileUploadType
	SubmissionID string

	Result *dto.FileInfo
}

type DeleteFiles struct {
	BlobKeys       []string
	Force          bool
	IncludeDeleted bool
	IncludeDrafts  bool
	Result         dto.FileDeletion
}

type PruneFiles struct {
	Search         string
	Type           string
	Before         time.Time
	Cursor         string
	IncludeDeleted bool
	IncludeDrafts  bool
	Result         dto.FileDeletion
}

type RetryMediaDeletions struct{}

type RenameImageFile struct {
	BlobKey string
	Name    string

	Result *dto.FileInfo
}
