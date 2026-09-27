package cmd

import (
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
)

type CreateComment struct {
	PostNumber   int
	PageID       int
	ParentID     *int
	Content      string
	Attachments  []*dto.ImageUpload
	SubmissionID string
	BaseURL      string

	Result     *entity.Comment
	Discussion *entity.Discussion
	Created    bool
}

type UpdateComment struct {
	CommentID    int
	Content      string
	Attachments  []*dto.ImageUpload
	BaseURL      string
	SubmissionID string

	Result     *entity.Comment
	Discussion *entity.Discussion
}

type DeleteComment struct {
	CommentID int

	Result     *entity.Comment
	Discussion *entity.Discussion
}
