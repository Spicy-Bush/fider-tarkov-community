package postgres_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/api"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestDiscussionSortingAndPagination(t *testing.T) {
	for _, kind := range []string{"post", "page"} {
		t.Run(kind, func(t *testing.T) {
			f := newPostWorkflow(t)
			var postID, pageID *int
			var discussion *entity.Discussion
			params := web.StringMap{}
			if kind == "post" {
				post := &cmd.AddNewPost{Title: "Sorted discussion", Description: "Post comments"}
				if err := bus.Dispatch(f.ctx, post); err != nil {
					t.Fatal(err)
				}

				postID = &post.Result.ID
				discussion = entity.PostDiscussion(post.Result)
				params["number"] = fmt.Sprint(post.Result.Number)
			} else {
				page := &cmd.CreatePage{
					Title:         "Sorted Page",
					Slug:          "sorted-page",
					Content:       "Page comments",
					Status:        entity.PageStatusPublished,
					Visibility:    entity.PageVisibilityPublic,
					AllowComments: true,
				}
				if err := bus.Dispatch(f.ctx, page); err != nil {
					t.Fatal(err)
				}

				pageID = &page.Result.ID
				discussion = entity.PageDiscussion(page.Result)
				params["id"] = fmt.Sprint(page.Result.ID)
			}

			type comment struct {
				id         int
				parent     int
				likes      int
				dislikes   int
				replies    int
				createdAt  time.Time
				deleted    bool
				hasReplies bool
			}
			var comments []comment
			transaction, err := mediaFixtureTransaction(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer transaction.Rollback()

			add := func(parent, likes, dislikes int) {
				t.Helper()
				var parentID *int
				if parent >= 0 {
					parentID = &comments[parent].id
				}

				created := comment{
					parent:    parent,
					likes:     likes,
					dislikes:  dislikes,
					createdAt: time.Date(2026, 9, 26, 12, 0, len(comments)%3, 0, time.UTC),
				}
				err := transaction.Scalar(&created.id, `
                    INSERT INTO comments (tenant_id, post_id, page_id, parent_id, user_id, content, created_at)
                    VALUES ($1, $2, $3, $4, $5, 'Sorted comment', $6) RETURNING id
				`, f.tenant.ID, postID, pageID, parentID, f.user.ID, created.createdAt)
				if err != nil {
					t.Fatal(err)
				}

				_, err = transaction.Execute(`
                    INSERT INTO reactions (comment_id, user_id, emoji, created_on)
                    SELECT $1::integer, n, '👍', NOW() FROM generate_series(1, $2) n
                    UNION ALL
                    SELECT $1::integer, n, '👎', NOW() FROM generate_series(1, $3) n
                    UNION ALL
                    SELECT $1::integer, n, '❤️', NOW() FROM generate_series(1, $4) n
                `, created.id, likes, dislikes, (len(comments)/5)%4)
				if err != nil {
					t.Fatal(err)
				}

				for ancestor := parent; ancestor >= 0; ancestor = comments[ancestor].parent {
					comments[ancestor].replies++
				}
				if parent >= 0 {
					comments[parent].hasReplies = true
				}

				comments = append(comments, created)
			}

			for index := 0; index < 60; index++ {
				add(-1, index%4, (index/4)%4)
			}
			for index := 0; index < 60; index++ {
				add(0, (index/4)%4, index%4)
			}
			for parent := 1; parent < 120; parent++ {
				for reply := 0; reply < parent%4; reply++ {
					add(parent, 0, 0)
				}
			}
			for _, index := range []int{1, 4, 5, 60, 61, 62, 121} {
				comments[index].deleted = true
				if _, err := transaction.Execute("UPDATE comments SET deleted_at = NOW() WHERE id = $1", comments[index].id); err != nil {
					t.Fatal(err)
				}
				for ancestor := comments[index].parent; ancestor >= 0; ancestor = comments[ancestor].parent {
					comments[ancestor].replies--
				}
			}

			if err := transaction.Commit(); err != nil {
				t.Fatal(err)
			}

			for _, order := range []string{"", "liked", "disliked", "replies", "latest"} {
				for _, parent := range []int{-1, 0} {
					t.Run(fmt.Sprintf("%s/parent=%d", order, parent), func(t *testing.T) {
						score := func(comment comment) int {
							switch order {
							case "", "liked":
								return comment.likes
							case "disliked":
								return comment.dislikes
							case "replies":
								return comment.replies
							default:
								return 0
							}
						}

						var expected []comment
						for _, candidate := range comments {
							if candidate.parent == parent && (!candidate.deleted || candidate.hasReplies) {
								expected = append(expected, candidate)
							}
						}

						sort.Slice(expected, func(left, right int) bool {
							first, second := expected[left], expected[right]
							if score(first) != score(second) {
								return score(first) > score(second)
							}

							if !first.createdAt.Equal(second.createdAt) {
								return first.createdAt.After(second.createdAt)
							}

							return first.id > second.id
						})

						parameters := url.Values{"sort": {order}}
						if parent >= 0 {
							parameters.Set("parentId", fmt.Sprint(comments[parent].id))
						}

						var received []int
						deletedAnchor := 0
						for page := 0; page < 10; page++ {
							storagePage := &query.GetDiscussionComments{Discussion: discussion, Sort: order}
							if parent >= 0 {
								storagePage.ParentID = &comments[parent].id
							}

							if len(received) > 0 {
								anchor := expected[len(received)-1]
								storagePage.After = anchor.createdAt
								storagePage.AfterID = anchor.id
								storagePage.AfterScore = score(anchor)
							}

							if err := bus.Dispatch(f.ctx, storagePage); err != nil {
								t.Fatal(err)
							}

							wantRows := min(26, len(expected)-len(received))
							if len(storagePage.Result) != wantRows {
								t.Fatalf("storage page %d has %d rows, want %d", page, len(storagePage.Result), wantRows)
							}

							for index, actual := range storagePage.Result {
								want := expected[len(received)+index]
								if actual.ID != want.id || actual.SortScore != score(want) {
									t.Fatalf("storage page %d row %d: got %+v, want %+v", page, index, actual, want)
								}
							}

							response, err := f.requestWithParams(api.ListDiscussion(), http.MethodGet, "/api/discussion?"+parameters.Encode(), "", params)
							if err != nil || response.Code != http.StatusOK {
								t.Fatalf("list comments: %v, HTTP %d, %s", err, response.Code, response.Body)
							}

							var result entity.DiscussionPage
							if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
								t.Fatal(err)
							}

							wantCount := min(25, len(expected)-len(received))
							if len(result.Comments) != wantCount {
								t.Fatalf("page %d has %d comments, want %d", page, len(result.Comments), wantCount)
							}

							for _, actual := range result.Comments {
								want := expected[len(received)]
								if (actual.State == "deleted") != want.deleted || actual.HasReplies != want.hasReplies {
									t.Fatalf("comment %d has state=%s replies=%v, want %+v", actual.ID, actual.State, actual.HasReplies, want)
								}
								received = append(received, actual.ID)
							}

							if result.Next == "" {
								break
							}

							anchor := expected[len(received)-1]
							if deletedAnchor == 0 && !anchor.deleted {
								if _, err := mediaFixtureSQL("UPDATE comments SET deleted_at = NOW() WHERE id = $1", anchor.id); err != nil {
									t.Fatal(err)
								}
								deletedAnchor = anchor.id
								t.Cleanup(func() {
									if _, err := mediaFixtureSQL("UPDATE comments SET deleted_at = NULL WHERE id = $1", anchor.id); err != nil {
										t.Error(err)
									}
								})
							}

							parameters.Set("after", result.Next)
						}

						if deletedAnchor == 0 {
							t.Fatal("pagination did not exercise a deleted cursor anchor")
						}

						want := make([]int, len(expected))
						for index, comment := range expected {
							want[index] = comment.id
						}

						if !slices.Equal(received, want) {
							t.Fatalf("sorted pagination differs:\ngot %v\nwant %v", received, want)
						}
					})
				}
			}
		})
	}
}

func TestDiscussionReportCapabilities(t *testing.T) {
	for _, kind := range []string{"post", "page"} {
		t.Run(kind, func(t *testing.T) {
			f := newPostWorkflow(t)
			create := &cmd.CreateComment{
				Content:      "First comment",
				SubmissionID: "first-comment",
			}
			if kind == "post" {
				post := &cmd.AddNewPost{Title: "Reporting discussion", Description: "Post comments"}
				if err := bus.Dispatch(f.ctx, post); err != nil {
					t.Fatal(err)
				}

				create.PostNumber = post.Result.Number
			} else {
				page := &cmd.CreatePage{
					Title:         "Reporting Page",
					Slug:          "reporting-page",
					Content:       "Page comments",
					Status:        entity.PageStatusPublished,
					Visibility:    entity.PageVisibilityPublic,
					AllowComments: true,
				}
				if err := bus.Dispatch(f.ctx, page); err != nil {
					t.Fatal(err)
				}

				create.PageID = page.Result.ID
			}

			visitor := &query.GetUserByID{UserID: 2}
			if err := bus.Dispatch(f.ctx, visitor, create); err != nil {
				t.Fatal(err)
			}

			firstID := create.Result.ID
			create.Content = "Second comment"
			create.SubmissionID = "second-comment"
			if err := bus.Dispatch(f.ctx, create); err != nil {
				t.Fatal(err)
			}

			secondID := create.Result.ID
			f.user = visitor.Result
			assertReport := func(id int, want bool) {
				t.Helper()
				params := web.StringMap{"id": fmt.Sprint(id)}
				response, err := f.requestWithParams(api.ReadDiscussionComment(), http.MethodGet, "/api/comments/"+params["id"], "", params)
				if err != nil || response.Code != http.StatusOK {
					t.Fatalf("read comment: %v, HTTP %d, %s", err, response.Code, response.Body)
				}

				var context entity.CommentContext
				if err := json.Unmarshal(response.Body.Bytes(), &context); err != nil {
					t.Fatal(err)
				}

				if len(context.Comments) != 1 || context.Comments[0].Permissions.Report != want {
					t.Fatalf("comment %d report permission should be %v: %s", id, want, response.Body)
				}
			}

			assertReport(firstID, true)
			assertReport(secondID, true)
			var reportID int
			if err := dbx.Connection().QueryRow(`
                INSERT INTO reports (tenant_id, reporter_id, reported_type, reported_id, reason, status, created_at)
                VALUES ($1, $2, 'comment', $3, 'Review', 'pending', NOW()) RETURNING id
            `, f.tenant.ID, visitor.Result.ID, firstID).Scan(&reportID); err != nil {
				t.Fatal(err)
			}

			assertReport(firstID, false)
			assertReport(secondID, true)

			f.tenant.GeneralSettings = &entity.GeneralSettings{ReportLimitsPerDay: 1}
			assertReport(firstID, false)
			assertReport(secondID, false)

			_, err := dbx.Connection().Exec("UPDATE reports SET status = 'resolved', created_at = CURRENT_DATE - INTERVAL '1 day' WHERE id = $1", reportID)
			if err != nil {
				t.Fatal(err)
			}

			assertReport(firstID, true)
			assertReport(secondID, true)

			f.tenant.GeneralSettings.ReportingGloballyDisabled = true
			assertReport(firstID, false)
			assertReport(secondID, false)
		})
	}
}

func TestDiscussionChainPrefetch(t *testing.T) {
	f := newPostWorkflow(t)
	post := &cmd.AddNewPost{Title: "Long discussion", Description: "A chain with a branch"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}

	ids := make([]int, 32)
	for index := range ids {
		var parentID *int
		if index > 0 {
			parentID = &ids[index-1]
		}

		err := mediaFixtureScalar(&ids[index], `
            INSERT INTO comments (tenant_id, post_id, parent_id, user_id, content, created_at)
            VALUES ($1, $2, $3, $4, 'Chain comment', NOW()) RETURNING id
        `, f.tenant.ID, post.Result.ID, parentID, f.user.ID)
		if err != nil {
			t.Fatal(err)
		}
	}

	read := func(parentID int, depth int) entity.DiscussionPage {
		t.Helper()
		params := web.StringMap{"number": fmt.Sprint(post.Result.Number)}
		path := fmt.Sprintf("/api/posts/%d/comments?parentId=%d&depth=%d&sort=replies", post.Result.Number, parentID, depth)
		response, err := f.requestWithParams(api.ListDiscussion(), http.MethodGet, path, "", params)
		if err != nil || response.Code != http.StatusOK {
			t.Fatalf("prefetch failed: %v, HTTP %d, %s", err, response.Code, response.Body)
		}

		var result entity.DiscussionPage
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}

		return result
	}

	first := read(ids[0], 10)
	if len(first.Comments) != 1 || len(first.Replies) != 9 || first.Replies[8].ID != ids[10] {
		t.Fatalf("ten-level prefetch exceeded or missed its boundary: %+v", first)
	}

	second := read(ids[10], 10)
	if len(second.Replies) != 9 || second.Replies[8].ID != ids[20] {
		t.Fatal("next ten levels did not continue the chain")
	}

	_, err := mediaFixtureSQL(`
        INSERT INTO comments (tenant_id, post_id, parent_id, user_id, content, created_at)
        VALUES ($1, $2, $3, $4, 'Another child', NOW())
    `, f.tenant.ID, post.Result.ID, ids[5], f.user.ID)
	if err != nil {
		t.Fatal(err)
	}

	branched := read(ids[0], 10)
	if len(branched.Replies) != 4 || branched.Replies[3].ID != ids[5] {
		t.Fatal("prefetch crossed a branch that requires sibling pagination")
	}

	shallow := read(ids[0], 1)
	if len(shallow.Replies) != 0 {
		t.Fatal("a one-level request fetched descendants")
	}

	for branch := 0; branch < 25; branch++ {
		parentID := ids[20]
		for depth := 0; depth < 10; depth++ {
			var id int
			err := mediaFixtureScalar(&id, `
                INSERT INTO comments (tenant_id, post_id, parent_id, user_id, content, created_at)
                VALUES ($1, $2, $3, $4, 'Wide chain', NOW()) RETURNING id
            `, f.tenant.ID, post.Result.ID, parentID, f.user.ID)
			if err != nil {
				t.Fatal(err)
			}

			parentID = id
		}
	}

	bounded := read(ids[20], 10)
	if len(bounded.Comments) != 25 || len(bounded.Replies) != 100 || bounded.Next == "" {
		t.Fatalf("wide prefetch returned %d siblings and %d descendants, cursor=%q", len(bounded.Comments), len(bounded.Replies), bounded.Next)
	}

	depths := make(map[int]int)
	for _, comment := range bounded.Comments {
		depths[comment.ID] = 1
	}

	for _, reply := range bounded.Replies {
		parentDepth, present := depths[*reply.ParentID]
		if !present || parentDepth >= 10 {
			t.Fatalf("prefetch lost the ancestry or exceeded the depth for comment %d", reply.ID)
		}

		depths[reply.ID] = parentDepth + 1
	}
}

func TestDiscussionMutationResponsesAndReportPreview(t *testing.T) {
	for _, kind := range []string{"post", "page"} {
		t.Run(kind, func(t *testing.T) {
			f := newPostWorkflow(t)
			create := &cmd.CreateComment{
				Content:      "Ancestor content",
				SubmissionID: "ancestor",
			}
			if kind == "post" {
				post := &cmd.AddNewPost{Title: "Mutation discussion", Description: "Full owner preview"}
				if err := bus.Dispatch(f.ctx, post); err != nil {
					t.Fatal(err)
				}
				create.PostNumber = post.Result.Number
			} else {
				page := &cmd.CreatePage{
					Title: "Mutation Page", Slug: "mutation-page", Content: "Full owner preview",
					Status: entity.PageStatusPublished, Visibility: entity.PageVisibilityPublic,
					AllowComments: true, AllowReactions: true,
				}
				if err := bus.Dispatch(f.ctx, page); err != nil {
					t.Fatal(err)
				}
				create.PageID = page.Result.ID
			}
			if err := bus.Dispatch(f.ctx, create); err != nil {
				t.Fatal(err)
			}
			rootID := create.Result.ID
			create.ParentID = &rootID
			create.Content = "Reply content"
			create.SubmissionID = "reply"
			if err := bus.Dispatch(f.ctx, create); err != nil {
				t.Fatal(err)
			}
			commentID := create.Result.ID
			params := web.StringMap{"id": fmt.Sprint(commentID), "reaction": "👍"}
			response, err := f.requestWithParams(api.ReactToDiscussionComment(), http.MethodPut,
				"/api/comments/"+params["id"]+"/reactions/thumb", `{"active":true}`, params)
			if err != nil || response.Code != http.StatusOK {
				t.Fatalf("reaction failed: %v HTTP%d %s", err, response.Code, response.Body)
			}
			var reaction map[string]json.RawMessage
			if err := json.Unmarshal(response.Body.Bytes(), &reaction); err != nil {
				t.Fatal(err)
			}
			var counts []entity.ReactionCounts
			if err := json.Unmarshal(reaction["reactionCounts"], &counts); err != nil {
				t.Fatal(err)
			}
			if len(reaction) != 1 || len(counts) != 1 || counts[0].Count != 1 || !counts[0].IncludesMe {
				t.Fatalf("reaction returned unrelated or incorrect fields: %s", response.Body)
			}

			for _, operation := range []struct {
				handler web.HandlerFunc
				hidden  bool
			}{{handlers.HideCommentModeration(), true}, {handlers.ApproveCommentModeration(), false}} {
				response, err := f.requestWithParams(operation.handler, http.MethodPut,
					"/api/comments/"+params["id"], "", params)
				if err != nil || response.Code != http.StatusOK {
					t.Fatalf("moderation failed: %v HTTP%d %s", err, response.Code, response.Body)
				}
				var updated entity.Comment
				if err := json.Unmarshal(response.Body.Bytes(), &updated); err != nil {
					t.Fatal(err)
				}
				if updated.ID != commentID || updated.ModerationPending != operation.hidden || updated.Content != "Reply content" {
					t.Fatalf("moderation did not return the changed comment: %s", response.Body)
				}
			}

			var reportID int
			if err := dbx.Connection().QueryRow(`
                INSERT INTO reports (tenant_id, reporter_id, reported_type, reported_id, reason, status, created_at)
                VALUES ($1, $2, 'comment', $3, 'Review', 'pending', NOW()) RETURNING id
            `, f.tenant.ID, f.user.ID, commentID).Scan(&reportID); err != nil {
				t.Fatal(err)
			}
			response, err = f.requestWithParams(handlers.GetReportDetails(), http.MethodGet,
				"/api/reports/details", "", web.StringMap{"id": fmt.Sprint(reportID)})
			if err != nil || response.Code != http.StatusOK {
				t.Fatalf("report preview failed: %v HTTP%d %s", err, response.Code, response.Body)
			}
			var details map[string]json.RawMessage
			if err := json.Unmarshal(response.Body.Bytes(), &details); err != nil {
				t.Fatal(err)
			}
			var preview map[string]any
			if err := json.Unmarshal(details[kind], &preview); err != nil {
				t.Fatal(err)
			}
			field := "content"
			if kind == "post" {
				field = "description"
			}
			if preview[field] != "Full owner preview" {
				t.Fatalf("focused owner lost report content: %s", response.Body)
			}

			response, err = f.requestWithParams(api.DeleteDiscussionComment(), http.MethodDelete,
				"/api/comments/"+params["id"], "", params)
			if err != nil || response.Code != http.StatusOK {
				t.Fatalf("delete failed: %v HTTP%d %s", err, response.Code, response.Body)
			}
			var deleted entity.Comment
			if err := json.Unmarshal(response.Body.Bytes(), &deleted); err != nil {
				t.Fatal(err)
			}
			if deleted.ID != commentID || deleted.State != "deleted" || deleted.Content != "" || deleted.User != nil {
				t.Fatalf("deletion did not return its projected placeholder: %s", response.Body)
			}
		})
	}
}
