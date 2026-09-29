package cmd

import (
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
)

type OpenPageEdit struct {
	PageID       int
	SubmissionID string
	Result       *entity.PageEditSession
}

type SyncPageEdit struct {
	PageID         int
	Update         []byte
	StateVector    []byte
	AcceptedUpdate []byte
	Result         *entity.PageEditSync
}

type PublishPageEdit struct {
	PageID       int
	Status       entity.PageStatus
	SubmissionID string
	Replayed     bool
	Result       *entity.Page
}

type UploadPageEditBanner struct {
	PageID       int
	SubmissionID string
	Image        *dto.ImageUpload
	Result       string
}
