package query

import (
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
)

type GetCommentByID struct {
	CommentID      int
	IncludeDeleted bool

	Result *entity.Comment
}

type GetDiscussion struct {
	PostNumber int
	PageID     int
	CommentID  int
	LockOwner  bool

	Result *entity.Discussion
}

type GetDiscussionComments struct {
	Discussion *entity.Discussion
	ParentID   *int
	Sort       string
	After      time.Time
	AfterID    int
	AfterScore int

	Result []*entity.Comment
}

type GetCommentAncestors struct {
	CommentID int
	Discussion *entity.Discussion

	Result []*entity.Comment
}

type GetDiscussionChainReplies struct {
	ParentIDs []int
	Depth     int

	Result []*entity.Comment
}
