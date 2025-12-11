package cmd

import (
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

type AddVote struct {
	Post     *entity.Post
	User     *entity.User
	VoteType enum.VoteType
}

type RemoveVote struct {
	Post *entity.Post
	User *entity.User
}

type MarkPostAsDuplicate struct {
	Post     *entity.Post
	Original *entity.Post
	Text     string
}
