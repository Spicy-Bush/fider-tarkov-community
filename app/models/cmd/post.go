package cmd

import (
	"context"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

type AddNewPost struct {
	Title       string
	Description string
	Attachments []*dto.ImageUpload

	Result *entity.Post
}

type UpdatePost struct {
	Post        *entity.Post
	Title       string
	Description string
	Attachments []*dto.ImageUpload

	Result *entity.Post
}

type SetPostResponse struct {
	Post   *entity.Post
	Text   string
	Status enum.PostStatus
}

type LockPost struct {
	Post        *entity.Post
	LockMessage string
}

type UnlockPost struct {
	Post *entity.Post
}

type RefreshPostStats struct {
	RowsUpdated int64
}

type ArchivePost struct {
	Post *entity.Post
}

type UnarchivePost struct {
	Post   *entity.Post
	Reason string
}

type BulkArchivePosts struct {
	PostIDs []int
}

type SubmitPost struct {
	SubmissionID string
	Fingerprint  string
	BaseURL      string
	Attachments  []*dto.ImageUpload
	Validate     func(context.Context) error
	Create       func(context.Context) (*entity.Post, error)
	Result       *dto.PostSubmissionReceipt
}
