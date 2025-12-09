package query

import (
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

type PostIsReferenced struct {
	PostID int

	Result bool
}

type CountPostPerStatus struct {
	Result map[enum.PostStatus]int
}

type GetPostByID struct {
	PostID int

	Result *entity.Post
}

type GetPostBySlug struct {
	Slug string

	Result *entity.Post
}

type GetPostByNumber struct {
	Number int

	Result *entity.Post
}

type SearchPosts struct {
	Query       string
	View        string
	Limit       string
	Offset      string `json:"offset"`
	Statuses    []enum.PostStatus
	Tags        []string
	MyVotesOnly bool
	MyPostsOnly bool
	NotMyVotes  bool
	Untagged    bool
	Date        string
	TagLogic    string `json:"taglogic"`
	Count       int    `json:"count,omitempty"`

	Result []*entity.Post
}

type GetUserPostCount struct {
	UserID int
	Since  time.Time
	Result int
}

type GetUserCommentCount struct {
	UserID int
	Since  time.Time
	Result int
}

type GetAllPosts struct {
	Result []*entity.Post
}

type GetPostsByIDs struct {
	PostIDs []int

	Result []*entity.Post
}

func (q *SearchPosts) SetStatusesFromStrings(statuses []string) {
	for _, v := range statuses {
		var postStatus enum.PostStatus
		if err := postStatus.UnmarshalText([]byte(v)); err == nil {
			q.Statuses = append(q.Statuses, postStatus)
		}
	}
}
