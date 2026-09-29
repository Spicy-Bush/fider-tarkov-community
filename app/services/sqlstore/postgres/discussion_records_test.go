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
			if records.Comments[0].State != "deleted" || records.Comments[0].Content != "" || records.Comments[0].User != nil {
				t.Fatalf("deleted record leaked content: %s", response.Body)
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
