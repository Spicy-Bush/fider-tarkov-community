package postgres_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/api"
	"github.com/Spicy-Bush/fider-tarkov-community/app/middlewares"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"github.com/lib/pq"
	"github.com/reearth/ygo/crdt"
)

func TestSponsorPlacementSettingsBelongToTenant(t *testing.T) {
	f := newPostWorkflow(t)
	otherTenant := &query.GetTenantByDomain{Domain: "avengers"}
	if err := bus.Dispatch(f.ctx, otherTenant); err != nil {
		t.Fatal(err)
	}

	otherCtx := withTenant(f.ctx, otherTenant.Result)
	before := &query.GetSponsorPlacements{}
	if err := bus.Dispatch(otherCtx, before); err != nil {
		t.Fatal(err)
	}

	handler := middlewares.RequirePermission(entity.ManageSponsorship)(api.SaveSponsorPlacement())
	body := `{"submissionId":"placement-save","placement":{"id":"home_desktop","enabled":true,"position":"sidebar","every":0,"empty":"adsense","adsenseSlotId":"1234567890"}}`
	response, err := f.requestWithParams(handler, http.MethodPost, "/api/sponsorship/placements", body, nil)
	if err != nil || response.Code != http.StatusOK {
		t.Fatalf("placement update: status=%d body=%s err=%v", response.Code, response.Body, err)
	}

	after := &query.GetSponsorPlacements{}
	if err := bus.Dispatch(otherCtx, after); err != nil {
		t.Fatal(err)
	}

	oldJSON, _ := json.Marshal(before.Result)
	newJSON, _ := json.Marshal(after.Result)
	if string(oldJSON) != string(newJSON) {
		t.Fatalf("tenant one's edit changed tenant two's settings: before=%s after=%s", oldJSON, newJSON)
	}

	for _, role := range entity.PermissionRoles {
		for _, delegated := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/delegated=%t", role, delegated), func(t *testing.T) {
				viewer := *f.user
				viewer.Role = role
				f.user = &viewer
				f.tenant.RolePermissions = entity.RolePermissions{}
				if delegated {
					f.tenant.RolePermissions[role] = map[entity.Permission]bool{entity.ManageSponsorship: true}
				}

				response, err := f.requestWithParams(handler, http.MethodPost, "/api/sponsorship/placements", body, nil)
				want := http.StatusForbidden
				if delegated || role == enum.RoleAdministrator || role == enum.RoleCollaborator {
					want = http.StatusOK
				}
				if err != nil || response.Code != want {
					t.Fatalf("placement permission: status=%d want=%d body=%s err=%v", response.Code, want, response.Body, err)
				}
			})
		}
	}

	f.user = nil
	response, err = f.requestWithParams(handler, http.MethodPost, "/api/sponsorship/placements", body, nil)
	if err != nil || response.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous placement update: status=%d err=%v", response.Code, err)
	}

	f.user = &entity.User{
		ID:     1,
		Role:   enum.RoleAdministrator,
		Status: enum.UserBlocked,
		Tenant: f.tenant,
	}
	response, err = f.requestWithParams(handler, http.MethodPost, "/api/sponsorship/placements", body, nil)
	if err != nil || response.Code != http.StatusForbidden {
		t.Fatalf("blocked placement update: status=%d err=%v", response.Code, err)
	}

	f.user.Status = enum.UserActive
	response, err = f.requestWithParams(handler, http.MethodPost, "/api/sponsorship/placements", `{"placement":{"id":"unknown"}}`, nil)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unknown placement update: status=%d err=%v", response.Code, err)
	}

	if err := bus.Dispatch(otherCtx, after); err != nil {
		t.Fatal(err)
	}

	newJSON, _ = json.Marshal(after.Result)
	if string(oldJSON) != string(newJSON) {
		t.Fatalf("a later update changed tenant two's settings: %s", newJSON)
	}
}

func TestPageRelationsRejectForeignTenant(t *testing.T) {
	for _, field := range []string{"authors", "topics", "tags", "parentPageId"} {
		for _, method := range []string{http.MethodPost, http.MethodPut} {
			t.Run(field+"/"+method, func(t *testing.T) {
				f := newPostWorkflow(t)
				var foreignID, ownID int
				statement := `
					INSERT INTO page_topics (tenant_id, name, slug, description, color)
					VALUES ($1, 'Private topic', 'private-topic', 'Private description', '#123456') RETURNING id
				`
				if field == "authors" {
					statement = "SELECT id FROM users WHERE tenant_id = $1 AND role = 3 LIMIT 1"
				} else if field == "tags" {
					statement = "INSERT INTO page_tags (tenant_id, name, slug) VALUES ($1, 'Private tag', 'private-tag') RETURNING id"
				} else if field == "parentPageId" {
					statement = `INSERT INTO pages (tenant_id, title, slug, content, created_by_id, updated_by_id)
						SELECT $1, 'Private page', 'private-page', 'Private content', id, id
						FROM users WHERE tenant_id = $1 AND role = 3 LIMIT 1 RETURNING id`
				}
				if err := mediaFixtureScalar(&foreignID, statement, 2); err != nil {
					t.Fatal(err)
				}

				if err := mediaFixtureScalar(&ownID, statement, 1); err != nil {
					t.Fatal(err)
				}

				open := &cmd.OpenPageEdit{SubmissionID: "page-associations"}
				if method == http.MethodPut {
					created := &cmd.CreatePage{
						Title:      "Original title",
						Content:    "Original content",
						Slug:       "original-page",
						Status:     entity.PageStatusPublished,
						Visibility: entity.PageVisibilityPublic,
					}
					if err := bus.Dispatch(f.ctx, created); err != nil {
						t.Fatal(err)
					}

					open.PageID = created.Result.ID
				}
				if err := bus.Dispatch(f.ctx, open); err != nil {
					t.Fatal(err)
				}
				change := func(id int) func(*crdt.Transaction) {
					return func(transaction *crdt.Transaction) {
						for name, value := range map[string]string{"title": "Boundary attempt", "content": "New content"} {
							text := transaction.GetText(name)
							text.Delete(transaction, 0, text.Len())
							text.Insert(transaction, 0, value, nil)
						}
						if field == "parentPageId" {
							transaction.GetMap("settings").Set(transaction, field, id)
						} else {
							selected := transaction.GetMap(field)
							selected.Delete(transaction, fmt.Sprint(foreignID))
							selected.Delete(transaction, fmt.Sprint(ownID))
							var value any = true
							if field == "authors" {
								value = int64(0)
								}
							selected.Set(transaction, fmt.Sprint(id), value)
						}
					}
				}
				syncPageEditingHTTP(t, f, open.Result, change(foreignID))
				params := web.StringMap{"id": fmt.Sprint(open.Result.PageID)}
				body, err := json.Marshal(map[string]any{"status": "published", "submissionId": "publish-associations"})
				if err != nil {
					t.Fatal(err)
				}

				response, err := f.requestWithParams(handlers.PublishPageEdit(), http.MethodPost, "/api/pages/publish", string(body), params)
				if err != nil || response.Code != http.StatusBadRequest {
					t.Fatalf("foreign %s accepted: status=%d body=%s err=%v", field, response.Code, response.Body, err)
				}

				if got := workflowCount(t, "SELECT COUNT(*) FROM pages WHERE tenant_id = 1 AND title = 'Boundary attempt'"); got != 0 {
					t.Fatal("rejected relation changed the page")
				}

				if err := bus.Dispatch(f.ctx, open); err != nil {
					t.Fatal(err)
				}
				syncPageEditingHTTP(t, f, open.Result, change(ownID))
				response, err = f.requestWithParams(handlers.PublishPageEdit(), http.MethodPost, "/api/pages/publish", string(body), params)
				if err != nil || response.Code != http.StatusOK {
					t.Fatalf("valid %s did not recover: status=%d body=%s err=%v", field, response.Code, response.Body, err)
				}
			})
		}
	}
}

func TestPageRelationConstraintsRejectForeignTenant(t *testing.T) {
	f := newPostWorkflow(t)
	page := &cmd.CreatePage{
		Title:      "Local page",
		Slug:       "local-page",
		Content:    "Local content",
		Status:     entity.PageStatusPublished,
		Visibility: entity.PageVisibilityPublic,
	}
	if err := bus.Dispatch(f.ctx, page); err != nil {
		t.Fatal(err)
	}

	var topicID, tagID, pageID int
	if err := dbx.Connection().QueryRow(`
		INSERT INTO page_topics (tenant_id, name, slug, description, color)
		VALUES (2, 'Foreign topic', 'foreign-topic', '', '') RETURNING id
	`).Scan(&topicID); err != nil {
		t.Fatal(err)
	}

	if err := dbx.Connection().QueryRow(`
		INSERT INTO page_tags (tenant_id, name, slug)
		VALUES (2, 'Foreign tag', 'foreign-tag') RETURNING id
	`).Scan(&tagID); err != nil {
		t.Fatal(err)
	}

	if err := mediaFixtureScalar(&pageID, `
		INSERT INTO pages (tenant_id, title, slug, content, created_by_id, updated_by_id)
		VALUES (2, 'Foreign page', 'foreign-page', 'Private content', 4, 4) RETURNING id
	`); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name      string
		sql       string
		foreignID int
	}{
		{"author", "INSERT INTO page_authors (tenant_id, page_id, user_id) VALUES (1, $1, $2)", 4},
		{"topic", "INSERT INTO page_topics_map (tenant_id, page_id, topic_id) VALUES (1, $1, $2)", topicID},
		{"tag", "INSERT INTO page_tags_map (tenant_id, page_id, tag_id) VALUES (1, $1, $2)", tagID},
		{"parent", "UPDATE pages SET parent_page_id = $2 WHERE id = $1", pageID},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := dbx.Connection().Exec(test.sql, page.Result.ID, test.foreignID)
			constraint, ok := err.(*pq.Error)
			if !ok || constraint.Code != "23503" {
				t.Fatalf("cross-tenant relation was not rejected by its foreign key: %v", err)
			}
		})
	}
}

func TestProfileContentDoesNotRevealHiddenVotedPosts(t *testing.T) {
	f := newPostWorkflow(t)
	post := &cmd.AddNewPost{Title: "Hidden title", Description: "Hidden description"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}

	visitor := &query.GetUserByID{UserID: 2}
	if err := bus.Dispatch(f.ctx, visitor); err != nil {
		t.Fatal(err)
	}

	if err := bus.Dispatch(f.ctx, &cmd.AddVote{Post: post.Result, User: visitor.Result, VoteType: enum.VoteTypeUp}); err != nil {
		t.Fatal(err)
	}

	if _, err := mediaFixtureSQL("UPDATE posts SET moderation_pending = true WHERE id = $1", post.Result.ID); err != nil {
		t.Fatal(err)
	}

	f.user = visitor.Result

	response, err := f.requestWithParams(api.SearchUserContent(), http.MethodGet,
		"/api/user/profile/2/content?contentType=voted", "", web.StringMap{"userID": "2"})
	if err != nil || response.Code != http.StatusOK {
		t.Fatalf("content search: status=%d body=%s err=%v", response.Code, response.Body, err)
	}

	var result struct {
		Posts []query.UserPostResult `json:"posts"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}

	if len(result.Posts) != 0 {
		t.Fatalf("ordinary voter learned a hidden post: %s", response.Body)
	}
}

func TestProfileContentUsesDiscussionVisibility(t *testing.T) {
	f := newPostWorkflow(t)
	author := &query.GetUserByID{UserID: 2}
	if err := bus.Dispatch(f.ctx, author); err != nil {
		t.Fatal(err)
	}

	publicPost := &cmd.AddNewPost{Title: "Public authored post", Description: "Visible content"}
	hiddenPost := &cmd.AddNewPost{Title: "Hidden authored post", Description: "Hidden content"}
	if err := bus.Dispatch(withUser(f.ctx, author.Result), publicPost, hiddenPost); err != nil {
		t.Fatal(err)
	}

	if _, err := mediaFixtureSQL("UPDATE posts SET moderation_pending = true WHERE id = $1", hiddenPost.Result.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := mediaFixtureSQL(`
		INSERT INTO comments (tenant_id, post_id, user_id, content, moderation_pending, created_at)
		VALUES (1, $1, 2, 'Public authored comment', false, NOW()),
		       (1, $1, 2, 'Hidden authored comment', true, NOW()),
		       (1, $2, 2, 'Comment on hidden post', false, NOW())
	`, publicPost.Result.ID, hiddenPost.Result.ID); err != nil {
		t.Fatal(err)
	}

	for _, role := range entity.PermissionRoles {
		t.Run(role.String(), func(t *testing.T) {
			f.user = &entity.User{
				ID:     3,
				Role:   role,
				Status: enum.UserActive,
				Tenant: f.tenant,
			}
			f.tenant.RolePermissions = entity.RolePermissions{
				role: {entity.ReadProfiles: true},
			}
			response, err := f.requestWithParams(api.SearchUserContent(), http.MethodGet,
				"/api/user/profile/2/content?contentType=all&q=authored&sortBy=title&sortOrder=asc",
				"", web.StringMap{"userID": "2"})
			if err != nil || response.Code != http.StatusOK {
				t.Fatalf("content search: status=%d body=%s err=%v", response.Code, response.Body, err)
			}

			var result struct {
				Posts    []query.UserPostResult    `json:"posts"`
				Comments []query.UserCommentResult `json:"comments"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}

			want := 1
			if role == enum.RoleModerator || role == enum.RoleCollaborator || role == enum.RoleAdministrator {
				want = 2
			}

			if len(result.Posts) != want || len(result.Comments) != want {
				t.Fatalf("profile content differs from discussion visibility: %s", response.Body)
			}
		})
	}

	f.user = author.Result
	response, err := f.requestWithParams(api.SearchUserContent(), http.MethodGet,
		"/api/user/profile/2/content?contentType=all", "", web.StringMap{"userID": "2"})
	if err != nil || response.Code != http.StatusOK {
		t.Fatalf("own content search: status=%d body=%s err=%v", response.Code, response.Body, err)
	}

	var own struct {
		Posts    []query.UserPostResult    `json:"posts"`
		Comments []query.UserCommentResult `json:"comments"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &own); err != nil {
		t.Fatal(err)
	}

	if len(own.Posts) != 2 || len(own.Comments) != 3 {
		t.Fatalf("author lost access to their own content: %s", response.Body)
	}
}
