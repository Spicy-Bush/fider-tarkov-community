package cmd

import (
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
)

type CreatePage struct {
	Title           string
	Slug            string
	Content         string
	Excerpt         string
	BannerImage     *dto.ImageUpload
	Status          entity.PageStatus
	Visibility      entity.PageVisibility
	AllowedRoles    []string
	ParentPageID    *int
	AllowComments   bool
	AllowReactions  bool
	ShowTOC         bool
	ScheduledFor    *time.Time
	Authors         []int
	Topics          []int
	Tags            []int
	MetaDescription string
	CanonicalURL    string
	Result          *entity.Page
}

type UpdatePage struct {
	PageID          int
	Title           string
	Slug            string
	Content         string
	Excerpt         string
	BannerImage     *dto.ImageUpload
	Status          entity.PageStatus
	Visibility      entity.PageVisibility
	AllowedRoles    []string
	ParentPageID    *int
	AllowComments   bool
	AllowReactions  bool
	ShowTOC         bool
	ScheduledFor    *time.Time
	Authors         []int
	Topics          []int
	Tags            []int
	MetaDescription string
	CanonicalURL    string
}

type DeletePage struct {
	PageID int
}

type SavePageDraft struct {
	PageID          int
	Title           string
	Slug            string
	Content         string
	Excerpt         string
	BannerImageBKey string
	MetaDescription string
	ShowTOC         bool
	DraftData       map[string]interface{}
}

type TogglePageReaction struct {
	Page   *entity.Page
	Emoji  string
	Result bool
}

type TogglePageSubscription struct {
	PageID int
	Result bool
}

type PublishScheduledPages struct {
	Result int
}

type CreatePageTopic struct {
	Name        string
	Slug        string
	Description string
	Color       string
	Result      *entity.PageTopic
}

type UpdatePageTopic struct {
	ID          int
	Name        string
	Slug        string
	Description string
	Color       string
}

type DeletePageTopic struct {
	ID int
}

type CreatePageTag struct {
	Name   string
	Slug   string
	Result *entity.PageTag
}

type UpdatePageTag struct {
	ID   int
	Name string
	Slug string
}

type DeletePageTag struct {
	ID int
}

type AddPageComment struct {
	Page    *entity.Page
	Content string
	Result  *entity.Comment
}

type RefreshPageEmbeddedData struct {
	PageID int
}
