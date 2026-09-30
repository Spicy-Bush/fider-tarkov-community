package query

import "github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"

type ListPostVotes struct {
	PostID       int
	Preview      bool
	IncludeEmail bool

	Result []*entity.Vote
}
