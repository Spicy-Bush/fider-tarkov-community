package postgres_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/jobs"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
)

func TestPostStatsExpireWithoutNewActivity(t *testing.T) {
	f := newPostWorkflow(t)

	for _, days := range []int{31, 30, 1} {
		post := &cmd.AddNewPost{Title: fmt.Sprintf("Quiet suggestion %d", days), Description: "Recent activity must expire"}
		if err := bus.Dispatch(f.ctx, post); err != nil {
			t.Fatal(err)
		}

		if _, err := dbx.Connection().Exec(`
			INSERT INTO comments (tenant_id, post_id, user_id, content, created_at)
			VALUES ($1, $2, $3, 'Earlier comment', CURRENT_DATE - make_interval(days => $4))
		`, f.tenant.ID, post.Result.ID, f.user.ID, days); err != nil {
			t.Fatal(err)
		}

		if _, err := dbx.Connection().Exec(`
			INSERT INTO post_votes (tenant_id, post_id, user_id, vote_type, created_at)
			VALUES ($1, $2, $3, 1, CURRENT_DATE - make_interval(days => $4))
		`, f.tenant.ID, post.Result.ID, f.user.ID, days); err != nil {
			t.Fatal(err)
		}

		if _, err := dbx.Connection().Exec(`
			UPDATE posts
			SET recent_votes = 10, recent_comments = 10, last_activity_at = NOW() - INTERVAL '40 days'
			WHERE id = $1
		`, post.Result.ID); err != nil {
			t.Fatal(err)
		}

		lastRun := time.Now().Add(-time.Minute)
		if err := (jobs.RefreshPostStatsJobHandler{}).Run(jobs.Context{Context: f.ctx, LastSuccessfulRun: &lastRun}); err != nil {
			t.Fatal(err)
		}

		var votes, comments int
		if err := dbx.Connection().QueryRow("SELECT recent_votes, recent_comments FROM posts WHERE id = $1", post.Result.ID).Scan(&votes, &comments); err != nil {
			t.Fatal(err)
		}

		want := 1
		if days >= 30 {
			want = 0
		}
		if votes != want || comments != want {
			t.Errorf("%d-day-old activity: votes=%d comments=%d, want %d", days, votes, comments, want)
		}
	}

	unchanged := &cmd.RefreshPostStats{}
	if err := bus.Dispatch(f.ctx, unchanged); err != nil {
		t.Fatal(err)
	}

	if unchanged.RowsUpdated != 0 {
		t.Errorf("unchanged activity rewrote %d posts", unchanged.RowsUpdated)
	}
}
