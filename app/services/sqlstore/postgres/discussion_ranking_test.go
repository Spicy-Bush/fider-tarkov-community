package postgres

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestDiscussionTreeCacheBoundsAndSelectionOwnership(t *testing.T) {
	cache := discussionTreeCache{
		entries:   make(map[discussionTreeKey]discussionTree),
		maxRanks:  6,
		maxOwners: 2,
	}

	first := discussionTreeKey{TenantID: 1, Kind: "post", OwnerID: 1}
	second := discussionTreeKey{TenantID: 2, Kind: "post", OwnerID: 1}
	page := discussionTreeKey{TenantID: 1, Kind: "page", OwnerID: 1}

	ranks := []discussionReplyRank{{ID: 3}, {ID: 2}, {ID: 1}}
	cache.store(first, discussionTree{Generation: 1, Ranks: ranks})

	selected, found := cache.lookup(first, 1, &query.GetDiscussionComments{})
	if !found || len(selected.IDs) != 3 || selected.IDs[0] != 3 {
		t.Fatal("cached ranking was not selected")
	}

	selected.IDs[0] = -1
	again, _ := cache.lookup(first, 1, &query.GetDiscussionComments{})
	if again.IDs[0] != 3 {
		t.Fatal("a response changed the retained ranking")
	}

	if _, found := cache.lookup(second, 1, &query.GetDiscussionComments{}); found {
		t.Fatal("another tenant reused a ranking")
	}

	if _, found := cache.lookup(page, 1, &query.GetDiscussionComments{}); found {
		t.Fatal("a Page reused a post ranking")
	}

	if _, found := cache.lookup(first, 2, &query.GetDiscussionComments{}); found {
		t.Fatal("a newer tree reused an older generation")
	}

	cache.store(first, discussionTree{Generation: 2, Ranks: ranks})
	cache.store(second, discussionTree{Generation: 3, Ranks: ranks})
	if cache.rankCount != 6 || len(cache.entries) != 2 {
		t.Fatal("replacement retained obsolete ranks")
	}

	cache.store(page, discussionTree{Generation: 4, Ranks: ranks})
	if cache.rankCount != 6 || len(cache.entries) != 2 {
		t.Fatal("eviction exceeded the rank or owner bound")
	}

	if _, found := cache.lookup(first, 2, &query.GetDiscussionComments{}); found {
		t.Fatal("eviction retained the oldest owner")
	}

	cache.store(first, discussionTree{Generation: 5, Oversized: true})
	selected, found = cache.lookup(first, 5, &query.GetDiscussionComments{})
	if !found || !selected.Uncached || len(selected.IDs) != 0 {
		t.Fatal("an oversized owner was retained as a partial ranking")
	}
}

func TestDiscussionTreePreparationCancellationAndOversize(t *testing.T) {
	dbx.Seed()
	t.Cleanup(dbx.Seed)
	bus.Init(Service{})
	baseURL, _ := url.Parse("http://localhost:3000")
	ctx := context.WithValue(context.Background(), app.RequestCtxKey, web.Request{URL: baseURL})

	tenant := &query.GetTenantByDomain{Domain: "demo"}
	if err := bus.Dispatch(ctx, tenant); err != nil {
		t.Fatal(err)
	}

	ctx = context.WithValue(ctx, app.TenantCtxKey, tenant.Result)

	user := &query.GetUserByID{UserID: 1}
	if err := bus.Dispatch(ctx, user); err != nil {
		t.Fatal(err)
	}

	ctx = context.WithValue(ctx, app.UserCtxKey, user.Result)

	post := &cmd.AddNewPost{Title: "Bounded tree preparation", Description: "Cancellation and oversized trees"}
	if err := bus.Dispatch(ctx, post); err != nil {
		t.Fatal(err)
	}

	for index := 0; index < 3; index++ {
		comment := &cmd.CreateComment{
			PostNumber:   post.Result.Number,
			Content:      "A root comment",
			SubmissionID: fmt.Sprintf("root-%d", index),
		}
		if err := bus.Dispatch(ctx, comment); err != nil {
			t.Fatal(err)
		}
	}

	owner := &query.GetDiscussion{PostNumber: post.Result.Number}
	if err := bus.Dispatch(ctx, owner); err != nil {
		t.Fatal(err)
	}

	ranking := &query.GetDiscussionComments{Discussion: owner.Result, Sort: "replies"}
	key := discussionTreeKey{TenantID: tenant.Result.ID, Kind: "post", OwnerID: post.Result.ID}

	discussionTrees.builds <- struct{}{}
	discussionTrees.builds <- struct{}{}
	slotsHeld := true
	t.Cleanup(func() {
		if slotsHeld {
			<-discussionTrees.builds
			<-discussionTrees.builds
		}
	})

	reader, err := dbx.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Rollback()

	waitContext, cancel := context.WithTimeout(ctx, 30*time.Millisecond)
	_, waitErr := prepareDiscussionReplies(waitContext, reader, ranking, "post_id", tenant.Result.ID)
	reader.Rollback()
	cancel()
	<-discussionTrees.builds
	<-discussionTrees.builds
	slotsHeld = false

	if waitErr == nil {
		t.Fatal("a canceled request passed a saturated preparation gate")
	}

	blocker, err := dbx.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback()

	if _, err := blocker.Execute("LOCK TABLE comments IN ACCESS EXCLUSIVE MODE"); err != nil {
		t.Fatal(err)
	}

	readContext, cancel := context.WithCancel(ctx)
	defer cancel()

	reader, err = dbx.BeginTx(readContext)
	if err != nil {
		t.Fatal(err)
	}
	defer func(transaction *dbx.Trx) {
		cancel()
		transaction.Rollback()
	}(reader)

	var readerPID int
	if err := reader.Scalar(&readerPID, "SELECT pg_backend_pid()"); err != nil {
		t.Fatal(err)
	}

	prepared := make(chan error, 1)
	go func() {
		_, err := prepareDiscussionReplies(readContext, reader, ranking, "post_id", tenant.Result.ID)
		prepared <- err
	}()

	deadline := time.Now().Add(5 * time.Second)
	for {
		var blocked bool
		err := dbx.Connection().QueryRow(`
			SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid = $1 AND wait_event_type = 'Lock')
		`, readerPID).Scan(&blocked)
		if err != nil {
			t.Fatal(err)
		}

		if blocked {
			break
		}

		if time.Now().After(deadline) {
			t.Fatal("tree preparation did not reach the blocked query")
		}

		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	readErr := <-prepared
	reader.Rollback()

	if err := blocker.Rollback(); err != nil {
		t.Fatal(err)
	}

	if readErr == nil || len(discussionTrees.builds) != 0 {
		t.Fatal("failed preparation retained its build slot")
	}

	reader, err = dbx.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Rollback()

	selection, err := prepareDiscussionReplies(ctx, reader, ranking, "post_id", tenant.Result.ID)
	if err != nil || len(selection.IDs) != 3 {
		t.Fatalf("healthy preparation did not recover: %+v, %v", selection, err)
	}

	oversized, err := readDiscussionTree(reader, key, "post_id", 2, nil, nil)
	if err != nil || !oversized.Oversized || len(oversized.Ranks) != 0 {
		t.Fatalf("the bounded read returned a partial tree: %+v, %v", oversized, err)
	}

	if err := reader.Scalar(&oversized.Generation,
		"SELECT generation FROM discussion_tree_versions WHERE post_id = $1", post.Result.ID); err != nil {
		t.Fatal(err)
	}

	discussionTrees.store(key, oversized)
	if err := bus.Dispatch(ctx, ranking); err != nil {
		t.Fatal(err)
	}

	if len(ranking.Result) != 3 {
		t.Fatal("oversized owner fallback truncated the discussion")
	}
}
