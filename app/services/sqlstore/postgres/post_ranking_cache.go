package postgres

import (
	"context"
	"database/sql"
	"strconv"
	"sync"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
)

type postRankingKey struct {
	TenantID int
	View     string
}

type postRanking struct {
	Snapshot string
	IDs      []int
	Limit    int
	Expires  time.Time
}

var postRankings sync.Map

func searchPosts(ctx context.Context, q *query.SearchPosts) error {
	user, _ := ctx.Value(app.UserCtxKey).(*entity.User)
	transaction, _ := ctx.Value(app.TransactionCtxKey).(*dbx.Trx)
	limit, limitErr := strconv.Atoi(q.Limit)
	if user != nil || transaction != nil || limitErr != nil || limit < 1 || limit > 100 ||
		(q.Offset != "" && q.Offset != "0") || len(q.Statuses) > 0 || len(q.IDs) > 0 ||
		q.Date != "" || q.Query != "" || len(q.Tags) > 0 || q.Untagged || q.MyPostsOnly || q.MyVotesOnly || q.NotMyVotes {
		return loadSearchPosts(ctx, q)
	}

	switch q.View {
	case "trending", "controversial", "recently-updated":
	default:
		return loadSearchPosts(ctx, q)
	}

	trx, err := dbx.BeginTxWithOptions(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return err
	}
	defer trx.Rollback()
	ctx = context.WithValue(ctx, app.TransactionCtxKey, trx)

	var snapshot string
	if err := trx.Scalar(&snapshot, "SELECT txid_current_snapshot()::text"); err != nil {
		return err
	}
	tenant := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	key := postRankingKey{TenantID: tenant.ID, View: q.View}
	if saved, found := postRankings.Load(key); found {
		cached := saved.(postRanking)
		if cached.Snapshot == snapshot && cached.Limit >= limit && time.Now().Before(cached.Expires) {
			ids := cached.IDs[:min(limit, len(cached.IDs))]
			posts := &query.GetPostsByIDs{PostIDs: ids}
			if err := getPostsByIDs(ctx, posts); err != nil {
				return err
			}

			// Both reads use the same snapshot, including visibility and status.
			if len(posts.Result) != len(ids) {
				panic("cached post ranking disagrees with its database snapshot")
			}
			byID := make(map[int]*entity.Post, len(posts.Result))
			for _, post := range posts.Result {
				byID[post.ID] = post
			}
			q.Result = make([]*entity.Post, len(ids))
			for index, id := range ids {
				q.Result[index] = byID[id]
			}
			return trx.Commit()
		}
	}

	if err := loadSearchPosts(ctx, q); err != nil {
		return err
	}
	if err := trx.Commit(); err != nil {
		return err
	}
	ids := make([]int, len(q.Result))
	for index, post := range q.Result {
		ids[index] = post.ID
	}
	postRankings.Store(key, postRanking{
		Snapshot: snapshot,
		IDs:      ids,
		Limit:    limit,
		Expires:  time.Now().Add(time.Minute),
	})
	return nil
}
