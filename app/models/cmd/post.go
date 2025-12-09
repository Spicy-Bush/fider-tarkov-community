package cmd

import (
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

type AddNewPost struct {
	Title       string
	Description string

	Result *entity.Post
}

type UpdatePost struct {
	Post        *entity.Post
	Title       string
	Description string

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
	Since       *time.Time
	RowsUpdated int64
}
