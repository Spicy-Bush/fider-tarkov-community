package cmd

import "github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"

type SetCommentReaction struct {
	CommentID int
	Emoji     string
	Active    bool

	Result     *entity.Comment
	Discussion *entity.Discussion
}
