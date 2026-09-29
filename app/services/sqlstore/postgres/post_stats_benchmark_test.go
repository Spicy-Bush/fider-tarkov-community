package postgres

import (
	"context"
	"fmt"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
)

func recentStatsFixture(t testing.TB, count int) (*dbx.Trx, context.Context) {
	t.Helper()
	trx := exportSelectionFixture(t, count)

	for _, statement := range []string{
		"ALTER TABLE posts DISABLE TRIGGER USER",
		"ALTER TABLE post_votes DISABLE TRIGGER USER",
		"ALTER TABLE comments DISABLE TRIGGER USER",
		`INSERT INTO post_votes (tenant_id, post_id, user_id, vote_type, created_at)
		 SELECT tenant_id, id, 1, CASE WHEN number % 5 = 0 THEN -1 ELSE 1 END,
		     CURRENT_DATE - make_interval(days => number % 60)
		 FROM posts WHERE tenant_id = 1 AND status <> 6`,
		`INSERT INTO comments (tenant_id, post_id, user_id, content, created_at, deleted_at)
		 SELECT tenant_id, id, 1, 'Earlier comment',
		     CURRENT_DATE - make_interval(days => number % 60),
		     CASE WHEN number % 7 = 0 THEN NOW() END
		 FROM posts WHERE tenant_id = 1 AND status <> 6`,
		"ANALYZE posts",
		"ANALYZE post_votes",
		"ANALYZE comments",
	} {
		if _, err := trx.Execute(statement); err != nil {
			t.Fatal(err)
		}
	}

	return trx, context.WithValue(context.Background(), app.TransactionCtxKey, trx)
}

func TestPostStatsMatchFullRecalculation(t *testing.T) {
	trx, ctx := recentStatsFixture(t, 1000)
	_, err := trx.Execute(`
		CREATE TEMPORARY TABLE expected_stats ON COMMIT DROP AS
		SELECT post.id,
			COALESCE((SELECT SUM(vote_type) FROM post_votes vote
				WHERE vote.tenant_id = post.tenant_id AND vote.post_id = post.id
					AND vote.created_at > CURRENT_DATE - INTERVAL '30 days'), 0) AS votes,
			(SELECT COUNT(*) FROM comments comment
				WHERE comment.tenant_id = post.tenant_id AND comment.post_id = post.id
					AND comment.deleted_at IS NULL
					AND comment.created_at > CURRENT_DATE - INTERVAL '30 days') AS comments
		FROM posts post
		WHERE status NOT IN ($1, $2)
	`, int(enum.PostDeleted), int(enum.PostArchived))
	if err != nil {
		t.Fatal(err)
	}

	if err := refreshPostStats(ctx, &cmd.RefreshPostStats{}); err != nil {
		t.Fatal(err)
	}

	var mismatches int
	err = trx.Scalar(&mismatches, `
		SELECT COUNT(*) FROM expected_stats expected JOIN posts USING (id)
		WHERE (expected.votes, expected.comments)
			IS DISTINCT FROM (posts.recent_votes, posts.recent_comments)
	`)
	if err != nil || mismatches != 0 {
		t.Fatalf("mismatched counters=%d error=%v", mismatches, err)
	}
}

func BenchmarkRefreshPostStats(b *testing.B) {
	for _, count := range []int{1000, 100000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			_, ctx := recentStatsFixture(b, count)
			if err := refreshPostStats(ctx, &cmd.RefreshPostStats{}); err != nil {
				b.Fatal(err)
			}

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				if err := refreshPostStats(ctx, &cmd.RefreshPostStats{}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
