package postgres_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/api"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestDiscussionRecordSelection(t *testing.T) {
	for _, kind := range []string{"post", "page"} {
		t.Run(kind, func(t *testing.T) {
			f := newPostWorkflow(t)
			var postID, pageID *int
			params := web.StringMap{}
			path := "/api/posts/comments"

			if kind == "post" {
				post := &cmd.AddNewPost{Title: "Reading position", Description: "Fresh records"}
				if err := bus.Dispatch(f.ctx, post); err != nil {
					t.Fatal(err)
				}

				postID = &post.Result.ID
				params["number"] = fmt.Sprint(post.Result.Number)
			} else {
				page := &cmd.CreatePage{
					Title:          "Reading position",
					Slug:           "reading-position",
					Content:        "Fresh records",
					Status:         entity.PageStatusPublished,
					Visibility:     entity.PageVisibilityPublic,
					AllowComments:  true,
					AllowReactions: true,
				}
				if err := bus.Dispatch(f.ctx, page); err != nil {
					t.Fatal(err)
				}

				pageID = &page.Result.ID
				params["id"] = fmt.Sprint(page.Result.ID)
				path = "/api/pages/comments"
			}

			transaction, err := mediaFixtureTransaction(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer transaction.Rollback()

			rows, err := transaction.Query(`
                INSERT INTO comments (tenant_id, post_id, page_id, user_id, content, created_at)
                SELECT $1, $2, $3, 2, 'Record ' || n, NOW() FROM generate_series(1, 50) n
                RETURNING id
            `, f.tenant.ID, postID, pageID)
			if err != nil {
				t.Fatal(err)
			}

			var ids []string
			for rows.Next() {
				var id int
				if err := rows.Scan(&id); err != nil {
					t.Fatal(err)
				}

				ids = append(ids, fmt.Sprint(id))
			}

			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			if err := rows.Close(); err != nil {
				t.Fatal(err)
			}

			if err := transaction.Commit(); err != nil {
				t.Fatal(err)
			}

			response, err := f.requestWithParams(api.ListDiscussion(), http.MethodGet, path+"?ids="+strings.Join(ids, ","), "", params)
			if err != nil || response.Code != http.StatusOK {
				t.Fatalf("read records: %v, HTTP %d, %s", err, response.Code, response.Body)
			}

			var records entity.DiscussionPage
			if err := json.Unmarshal(response.Body.Bytes(), &records); err != nil {
				t.Fatal(err)
			}
			if len(records.Comments) != 50 || records.Next != "" {
				t.Fatalf("record selection was paginated: %s", response.Body)
			}

			for _, selection := range []string{"0", "-1", "1,x", "1%29%3BDROP", strings.Join(append(ids, ids[0]), ","), ids[0] + "&parentId=1"} {
				response, err := f.requestWithParams(api.ListDiscussion(), http.MethodGet, path+"?ids="+selection, "", params)
				if err != nil || response.Code != http.StatusBadRequest {
					t.Fatalf("invalid selection %q: %v, HTTP %d", selection, err, response.Code)
				}
			}

			other := &cmd.AddNewPost{Title: "Another discussion", Description: "Unrelated comments"}
			if err := bus.Dispatch(f.ctx, other); err != nil {
				t.Fatal(err)
			}

			var otherID int
			if err := mediaFixtureScalar(&otherID, `
                INSERT INTO comments (tenant_id, post_id, user_id, content, created_at)
                VALUES ($1, $2, 2, 'Another discussion', NOW()) RETURNING id
            `, f.tenant.ID, other.Result.ID); err != nil {
				t.Fatal(err)
			}

			response, err = f.requestWithParams(api.ListDiscussion(), http.MethodGet, path+"?ids="+ids[0]+","+fmt.Sprint(otherID)+",2147483647", "", params)
			if err != nil || response.Code != http.StatusOK {
				t.Fatalf("mixed owner selection: %v, HTTP %d", err, response.Code)
			}
			if err := json.Unmarshal(response.Body.Bytes(), &records); err != nil {
				t.Fatal(err)
			}
			if len(records.Comments) != 1 || fmt.Sprint(records.Comments[0].ID) != ids[0] {
				t.Fatalf("mixed owner selection exposed another discussion: %s", response.Body)
			}

			if _, err := mediaFixtureSQL("UPDATE comments SET moderation_pending = TRUE WHERE id = $1", ids[0]); err != nil {
				t.Fatal(err)
			}

			for _, role := range []enum.Role{0, enum.RoleVisitor, enum.RoleHelper, enum.RoleModerator, enum.RoleCollaborator, enum.RoleAdministrator} {
				f.user = &entity.User{ID: 1, Role: role, Status: enum.UserActive}
				if role == 0 {
					f.user = nil
				}

				response, err := f.requestWithParams(api.ListDiscussion(), http.MethodGet, path+"?ids="+ids[0], "", params)
				if err != nil || response.Code != http.StatusOK {
					t.Fatalf("role %v: %v, HTTP %d", role, err, response.Code)
				}
				if err := json.Unmarshal(response.Body.Bytes(), &records); err != nil {
					t.Fatal(err)
				}

				moderates := role == enum.RoleModerator || role == enum.RoleCollaborator || role == enum.RoleAdministrator
				if !moderates {
					if len(records.Comments) != 0 {
						t.Fatalf("role %v received a hidden comment: %s", role, response.Body)
					}
					continue
				}
				if len(records.Comments) != 1 {
					t.Fatalf("role %v lost its moderation view: %s", role, response.Body)
				}
				comment := records.Comments[0]
				if comment.Permissions.Moderate != moderates || (comment.Content != "") != moderates || comment.ModerationPending != moderates {
					t.Fatalf("role %v received incorrect projection: %s", role, response.Body)
				}
			}

			if _, err := mediaFixtureSQL("UPDATE comments SET deleted_at = NOW() WHERE id = $1", ids[0]); err != nil {
				t.Fatal(err)
			}

			response, err = f.requestWithParams(api.ListDiscussion(), http.MethodGet, path+"?ids="+ids[0], "", params)
			if err != nil || response.Code != http.StatusOK {
				t.Fatalf("deleted record: %v, HTTP %d", err, response.Code)
			}
			if err := json.Unmarshal(response.Body.Bytes(), &records); err != nil {
				t.Fatal(err)
			}
			if len(records.Comments) != 0 {
				t.Fatalf("deleted record remained visible: %s", response.Body)
			}

			if pageID != nil {
				if _, err := mediaFixtureSQL(`UPDATE pages SET visibility = 'private', allowed_roles = '["visitor"]' WHERE id = $1`, *pageID); err != nil {
					t.Fatal(err)
				}

				for _, role := range []enum.Role{0, enum.RoleVisitor, enum.RoleHelper, enum.RoleModerator, enum.RoleCollaborator, enum.RoleAdministrator} {
					f.user = &entity.User{ID: 1, Role: role, Status: enum.UserActive}
					if role == 0 {
						f.user = nil
					}

					want := http.StatusNotFound
					if role == enum.RoleVisitor || role == enum.RoleCollaborator || role == enum.RoleAdministrator {
						want = http.StatusOK
					}

					response, err := f.requestWithParams(api.ListDiscussion(), http.MethodGet, path+"?ids="+ids[0], "", params)
					if err != nil || response.Code != want {
						t.Fatalf("private Page, role %v: %v, HTTP %d, want %d", role, err, response.Code, want)
					}
				}
			}

			otherTenant := *f.tenant
			otherTenant.ID = 2
			f.tenant = &otherTenant
			response, err = f.requestWithParams(api.ListDiscussion(), http.MethodGet, path+"?ids="+ids[0], "", params)
			if err != nil || response.Code != http.StatusNotFound {
				t.Fatalf("another tenant resolved a reading outline: %v, HTTP %d", err, response.Code)
			}
		})
	}
}

func TestDiscussionPromotesVisibleReplies(t *testing.T) {
	for _, kind := range []string{"post", "page"} {
		t.Run(kind, func(t *testing.T) {
			f := newPostWorkflow(t)
			var postID, pageID *int
			params := web.StringMap{}
			if kind == "post" {
				post := &cmd.AddNewPost{Title: "Visible replies", Description: "Discussion"}
				if err := bus.Dispatch(f.ctx, post); err != nil {
					t.Fatal(err)
				}
				postID = &post.Result.ID
				params["number"] = fmt.Sprint(post.Result.Number)
			} else {
				page := &cmd.CreatePage{
					Title:          "Visible replies",
					Slug:           "visible-replies",
					Content:        "Discussion",
					Status:         entity.PageStatusPublished,
					Visibility:     entity.PageVisibilityPublic,
					AllowComments:  true,
					AllowReactions: true,
				}
				if err := bus.Dispatch(f.ctx, page); err != nil {
					t.Fatal(err)
				}
				pageID = &page.Result.ID
				params["id"] = fmt.Sprint(page.Result.ID)
			}

			var ids [5]int
			var parent *int
			for index := range ids {
				err := mediaFixtureScalar(&ids[index], `
                    INSERT INTO comments (tenant_id, post_id, page_id, parent_id, user_id,
                        content, created_at, moderation_pending, deleted_at)
                    VALUES ($1, $2, $3, $4, 2, $5, NOW(), $6,
                        CASE WHEN $7 THEN NOW() END) RETURNING id
                `, f.tenant.ID, postID, pageID, parent, fmt.Sprintf("Comment %d", index), index == 1, index == 2)
				if err != nil {
					t.Fatal(err)
				}
				parent = &ids[index]
			}

			for _, viewer := range []struct {
				role enum.Role
				id   int
			}{
				{0, 0}, {enum.RoleVisitor, 1}, {enum.RoleHelper, 1},
				{enum.RoleModerator, 1}, {enum.RoleCollaborator, 1},
				{enum.RoleAdministrator, 1}, {enum.RoleVisitor, 2},
			} {
				f.user = &entity.User{ID: viewer.id, Role: viewer.role, Status: enum.UserActive}
				if viewer.id == 0 {
					f.user = nil
				}
				staff := viewer.role == enum.RoleModerator || viewer.role == enum.RoleCollaborator || viewer.role == enum.RoleAdministrator
				seesHidden := staff || viewer.id == 2

				for _, order := range []string{"liked", "disliked", "latest", "replies"} {
					path := fmt.Sprintf("/api/discussion?parentId=%d&depth=10&sort=%s", ids[0], order)
					response, err := f.requestWithParams(api.ListDiscussion(), http.MethodGet, path, "", params)
					if err != nil || response.Code != http.StatusOK {
						t.Fatalf("%+v %s: %v HTTP %d %s", viewer, order, err, response.Code, response.Body)
					}
					var result entity.DiscussionPage
					if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
						t.Fatal(err)
					}
					wantIDs := []int{ids[3], ids[4]}
					if seesHidden {
						wantIDs = append([]int{ids[1]}, wantIDs...)
					}
					visible := append(result.Comments, result.Replies...)
					if len(visible) != len(wantIDs) {
						t.Fatalf("%+v received the wrong chain: %s", viewer, response.Body)
					}
					previous := ids[0]
					for index, comment := range visible {
						if comment.ID != wantIDs[index] || comment.ParentID == nil || *comment.ParentID != previous || comment.State != "visible" {
							t.Fatalf("%+v received an unprojected chain: %s", viewer, response.Body)
						}
						if comment.ModerationPending != (staff && comment.ID == ids[1]) || comment.ModerationData != "" {
							t.Fatalf("%+v received incorrect moderation metadata: %s", viewer, response.Body)
						}
						previous = comment.ID
					}
				}

				for _, index := range []int{1, 2, 4} {
					want := http.StatusOK
					if index == 2 || (index == 1 && !seesHidden) {
						want = http.StatusNotFound
					}
					response, err := f.requestWithParams(api.ReadDiscussionComment(), http.MethodGet,
						"/api/comments", "", web.StringMap{"id": fmt.Sprint(ids[index])})
					if err != nil || response.Code != want {
						t.Fatalf("%+v permalink %d: %v HTTP %d, want %d", viewer, index, err, response.Code, want)
					}
					if want == http.StatusOK {
						var result entity.CommentContext
						if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
							t.Fatal(err)
						}
						for _, comment := range result.Comments {
							if comment.ID == ids[2] || (!seesHidden && comment.ID == ids[1]) {
								t.Fatalf("permalink exposed an invisible ancestor: %s", response.Body)
							}
						}
					}
				}
			}
		})
	}
}
