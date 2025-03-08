package postgres

import (
	"context"
	"strconv"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
)

type dbVote struct {
	User *struct {
		ID            int    `db:"id"`
		Name          string `db:"name"`
		Email         string `db:"email"`
		AvatarType    int64  `db:"avatar_type"`
		AvatarBlobKey string `db:"avatar_bkey"`
	} `db:"user"`
	CreatedAt time.Time `db:"created_at"`
	VoteType  int       `db:"vote_type"`
}

func (v *dbVote) toModel(ctx context.Context) *entity.Vote {
	vote := &entity.Vote{
		CreatedAt: v.CreatedAt,
		VoteType:  enum.VoteType(v.VoteType),
		User: &entity.VoteUser{
			ID:        v.User.ID,
			Name:      v.User.Name,
			Email:     v.User.Email,
			AvatarURL: buildAvatarURL(ctx, enum.AvatarType(v.User.AvatarType), v.User.ID, v.User.Name, v.User.AvatarBlobKey),
		},
	}
	return vote
}

func addVote(ctx context.Context, c *cmd.AddVote) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if !c.Post.CanBeVoted() {
			return nil
		}

		_, err := trx.Execute(`DELETE FROM post_votes WHERE user_id = $1 AND post_id = $2 AND tenant_id = $3`,
			c.User.ID, c.Post.ID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to remove existing vote")
		}

		_, err = trx.Execute(
			`INSERT INTO post_votes (tenant_id, user_id, post_id, created_at, vote_type) VALUES ($1, $2, $3, $4, $5)`,
			tenant.ID, c.User.ID, c.Post.ID, time.Now(), int(c.VoteType),
		)

		if err != nil {
			return errors.Wrap(err, "failed add vote to post")
		}

		return nil
	})
}

func removeVote(ctx context.Context, c *cmd.RemoveVote) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if !c.Post.CanBeVoted() {
			return nil
		}

		_, err := trx.Execute(`DELETE FROM post_votes WHERE user_id = $1 AND post_id = $2 AND tenant_id = $3`, c.User.ID, c.Post.ID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to remove vote from post")
		}

		return nil
	})
}

func listPostVotes(ctx context.Context, q *query.ListPostVotes) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		q.Result = make([]*entity.Vote, 0)
		sqlLimit := "ALL"
		if q.Limit > 0 {
			sqlLimit = strconv.Itoa(q.Limit)
		}

		emailColumn := "''"
		if q.IncludeEmail {
			emailColumn = "u.email"
		}

		votes := []*dbVote{}
		err := trx.Select(&votes, `
		SELECT 
			pv.created_at, 
			pv.vote_type,
			u.id AS user_id,
			u.name AS user_name,
			`+emailColumn+` AS user_email,
			u.avatar_type AS user_avatar_type,
			u.avatar_bkey AS user_avatar_bkey
		FROM post_votes pv
		INNER JOIN users u
		ON u.id = pv.user_id
		AND u.tenant_id = pv.tenant_id 
		WHERE pv.post_id = $1  
		AND pv.tenant_id = $2
		ORDER BY pv.created_at
		LIMIT `+sqlLimit, q.PostID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to get votes of post")
		}

		q.Result = make([]*entity.Vote, len(votes))
		for i, vote := range votes {
			q.Result[i] = vote.toModel(ctx)
		}

		return nil
	})
}
