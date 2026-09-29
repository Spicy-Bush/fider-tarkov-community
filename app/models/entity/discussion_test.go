package entity_test

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

func TestDiscussionSignInAction(t *testing.T) {
	for _, kind := range []string{"post", "page"} {
		for _, policy := range []string{"open", "disabled", "visitor disabled", "helper disabled", "tenant locked"} {
			t.Run(kind+"/"+policy, func(t *testing.T) {
				discussion := entity.PostDiscussion(&entity.Post{ID: 1, Status: enum.PostOpen})
				if kind == "page" {
					discussion = entity.PageDiscussion(&entity.Page{
						ID:            1,
						Status:        entity.PageStatusPublished,
						Visibility:    entity.PageVisibilityPublic,
						AllowComments: true,
					})
				}

				tenant := &entity.Tenant{Status: enum.TenantActive, GeneralSettings: &entity.GeneralSettings{}}
				wantSignIn := policy == "open" || policy == "helper disabled"
				switch policy {
				case "disabled":
					tenant.GeneralSettings.CommentingGloballyDisabled = true
				case "visitor disabled":
					tenant.GeneralSettings.CommentingDisabledFor = []string{"visitor"}
				case "helper disabled":
					tenant.GeneralSettings.CommentingDisabledFor = []string{"helper"}
				case "tenant locked":
					tenant.Status = enum.TenantLocked
				}

				anonymous := discussion.Permissions(nil, tenant)
				if anonymous.Comment || anonymous.SignInToComment != wantSignIn {
					t.Fatalf("anonymous actions=%+v; want sign-in=%t", anonymous, wantSignIn)
				}

				for _, role := range entity.PermissionRoles {
					viewer := &entity.User{ID: 1, Role: role, Status: enum.UserActive}
					if permissions := discussion.Permissions(viewer, tenant); permissions.SignInToComment {
						t.Fatalf("signed-in %s was asked to sign in", role)
					}
				}

				discussion.AllowComments = false
				if discussion.Permissions(nil, tenant).SignInToComment {
					t.Fatal("disabled discussion invited anonymous comments")
				}
			})
		}
	}

	discussion := entity.PostDiscussion(&entity.Post{ID: 1, Status: enum.PostOpen})
	discussion.Locked = true
	if discussion.Permissions(nil, nil).SignInToComment {
		t.Fatal("locked post invited anonymous comments")
	}

	private := entity.PageDiscussion(&entity.Page{
		ID:            1,
		Status:        entity.PageStatusPublished,
		Visibility:    entity.PageVisibilityPrivate,
		AllowComments: true,
	})
	if private.Permissions(nil, nil).SignInToComment {
		t.Fatal("inaccessible Page invited anonymous comments")
	}
}

func TestHiddenCommentIsIndistinguishableToItsAuthor(t *testing.T) {
	now := time.Now()
	tenant := &entity.Tenant{}
	discussions := []*entity.Discussion{
		entity.PostDiscussion(&entity.Post{ID: 1, Status: enum.PostOpen}),
		entity.PageDiscussion(&entity.Page{
			ID:             1,
			Status:         entity.PageStatusPublished,
			Visibility:     entity.PageVisibilityPublic,
			AllowComments:  true,
			AllowReactions: true,
		}),
	}

	for _, discussion := range discussions {
		for _, role := range []enum.Role{enum.RoleVisitor, enum.RoleHelper} {
			t.Run(discussion.Owner.Kind+"/"+role.String(), func(t *testing.T) {
				author := &entity.User{ID: 2, Role: role, Status: enum.UserActive}
				comment := &entity.Comment{
					ID:             1,
					User:           author,
					Content:        "My comment",
					CreatedAt:      now,
					Attachments:    []string{"image"},
					HasReplies:     true,
					ReactionCounts: []entity.ReactionCounts{{Emoji: "👍", Count: 1, IncludesMe: true}},
				}

				before, err := json.Marshal(comment.ForViewer(author, discussion, tenant, now))
				if err != nil {
					t.Fatal(err)
				}

				comment.ModerationPending = true
				comment.ModerationData = `{"reason":"staff decision"}`
				after, err := json.Marshal(comment.ForViewer(author, discussion, tenant, now))
				if err != nil {
					t.Fatal(err)
				}

				if !bytes.Equal(before, after) {
					t.Fatalf("hiding changed the author's response:\nbefore: %s\nafter: %s", before, after)
				}

				moderator := &entity.User{ID: 3, Role: enum.RoleModerator, Status: enum.UserActive}
				staffView := comment.ForViewer(moderator, discussion, tenant, now)
				if !staffView.ModerationPending || staffView.ModerationData == "" {
					t.Fatal("authorized staff lost the hidden status")
				}

				publicView := comment.ForViewer(nil, discussion, tenant, now)
				if publicView.State != "hidden" || publicView.Content != "" {
					t.Fatal("hidden content became public")
				}

				if !comment.ModerationPending || comment.ModerationData == "" {
					t.Fatal("projection changed persisted moderation information")
				}
			})
		}
	}
}

func TestDiscussionCommentAuthority(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	discussion := entity.PostDiscussion(&entity.Post{ID: 1, Status: enum.PostOpen})
	tenant := &entity.Tenant{GeneralSettings: &entity.GeneralSettings{}}
	roles := []enum.Role{
		enum.RoleVisitor,
		enum.RoleHelper,
		enum.RoleModerator,
		enum.RoleCollaborator,
		enum.RoleAdministrator,
	}
	wantStaffAuthority := map[enum.Role][]bool{
		enum.RoleVisitor:       {false, false, false, false, false},
		enum.RoleHelper:        {false, false, false, false, false},
		enum.RoleModerator:     {true, true, false, false, false},
		enum.RoleCollaborator:  {true, true, true, true, true},
		enum.RoleAdministrator: {true, true, true, true, true},
	}

	for _, role := range roles {
		for authorIndex, authorRole := range roles {
			t.Run(role.String()+" acting on "+authorRole.String(), func(t *testing.T) {
				viewer := &entity.User{ID: 1, Role: role, Status: enum.UserActive}
				comment := &entity.Comment{
					User:      &entity.User{ID: 2, Role: authorRole},
					CreatedAt: now.Add(-24 * time.Hour),
				}

				permissions := comment.AllowedActions(viewer, discussion, tenant, now)
				want := wantStaffAuthority[role][authorIndex]
				if permissions.Edit != want || permissions.Delete != want || permissions.Moderate != want {
					t.Fatalf("unexpected authority: %+v; want edit/delete/moderate=%v", permissions, want)
				}
			})
		}
	}
}

func TestDiscussionAuthorEditDeadlineAndMute(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	discussion := entity.PostDiscussion(&entity.Post{ID: 1, Status: enum.PostOpen})
	tenant := &entity.Tenant{GeneralSettings: &entity.GeneralSettings{}}
	viewer := &entity.User{ID: 1, Role: enum.RoleVisitor, Status: enum.UserActive}
	comment := &entity.Comment{User: viewer, CreatedAt: now.Add(-time.Hour)}

	atDeadline := comment.AllowedActions(viewer, discussion, tenant, now)
	if !atDeadline.Edit || !atDeadline.Delete || atDeadline.Moderate {
		t.Fatalf("unexpected permissions at the deadline: %+v", atDeadline)
	}

	afterDeadline := comment.AllowedActions(viewer, discussion, tenant, now.Add(time.Microsecond))
	if afterDeadline.Edit || !afterDeadline.Delete {
		t.Fatalf("edit deadline was not enforced: %+v", afterDeadline)
	}

	viewer.Muted = true
	comment.CreatedAt = now
	muted := comment.AllowedActions(viewer, discussion, tenant, now)
	if muted.Edit || muted.Reply || muted.React || !muted.Delete {
		t.Fatalf("muted author must retain deletion only: %+v", muted)
	}

	viewer.Muted = false
	viewer.Role = enum.RoleModerator
	comment.CreatedAt = now.Add(-24 * time.Hour)
	moderator := comment.AllowedActions(viewer, discussion, tenant, now)
	if !moderator.Edit || !moderator.Delete || !moderator.Moderate {
		t.Fatalf("moderator could not manage their own comment: %+v", moderator)
	}
}

func TestDiscussionLockedPostEditAndRecovery(t *testing.T) {
	now := time.Now()
	post := &entity.Post{
		ID:             1,
		Status:         enum.PostOpen,
		LockedSettings: &entity.PostLockedSettings{Locked: true},
	}
	discussion := entity.PostDiscussion(post)
	tenant := &entity.Tenant{}
	roles := []enum.Role{enum.RoleVisitor, enum.RoleHelper, enum.RoleModerator, enum.RoleCollaborator, enum.RoleAdministrator}

	for _, role := range roles {
		t.Run(role.String(), func(t *testing.T) {
			post.LockedSettings.Locked = true
			discussion = entity.PostDiscussion(post)
			viewer := &entity.User{ID: 1, Role: role, Status: enum.UserActive}
			comment := &entity.Comment{User: viewer, CreatedAt: now}
			permissions := comment.AllowedActions(viewer, discussion, tenant, now)
			wantEdit := role == enum.RoleCollaborator || role == enum.RoleAdministrator

			if permissions.Edit != wantEdit {
				t.Errorf("locked post edit=%v, want %v", permissions.Edit, wantEdit)
			}
			if !permissions.Delete {
				t.Error("locking a post removed the author's deletion permission")
			}

			post.LockedSettings.Locked = false
			discussion = entity.PostDiscussion(post)
			if !comment.AllowedActions(viewer, discussion, tenant, now).Edit {
				t.Error("unlocking did not restore editing")
			}
		})
	}
}

func TestPrivatePageDiscussionVisibility(t *testing.T) {
	page := &entity.Page{
		ID:           1,
		Status:       entity.PageStatusPublished,
		Visibility:   entity.PageVisibilityPrivate,
		AllowedRoles: []string{"visitor"},
	}
	discussion := entity.PageDiscussion(page)
	tenant := &entity.Tenant{GeneralSettings: &entity.GeneralSettings{}}
	comment := &entity.Comment{User: &entity.User{ID: 2, Role: enum.RoleVisitor}}
	roles := []struct {
		role enum.Role
		view bool
	}{
		{role: enum.RoleVisitor, view: true},
		{role: enum.RoleHelper, view: false},
		{role: enum.RoleModerator, view: false},
		{role: enum.RoleCollaborator, view: true},
		{role: enum.RoleAdministrator, view: true},
	}

	if discussion.CanView(nil, tenant) {
		t.Fatal("anonymous reader could view private Page")
	}

	for _, candidate := range roles {
		viewer := &entity.User{ID: 1, Role: candidate.role, Status: enum.UserActive}
		if discussion.CanView(viewer, tenant) != candidate.view {
			t.Errorf("wrong private Page access for %s", candidate.role)
		}

		if !candidate.view {
			permissions := comment.AllowedActions(viewer, discussion, tenant, time.Now())
			if permissions != (entity.CommentPermissions{}) {
				t.Errorf("inaccessible Page granted comment actions to %s: %+v", candidate.role, permissions)
			}
		}
	}
}

func TestRemovedCommentProjection(t *testing.T) {
	discussion := entity.PostDiscussion(&entity.Post{ID: 1, Status: enum.PostOpen})
	tenant := &entity.Tenant{GeneralSettings: &entity.GeneralSettings{}}
	comment := &entity.Comment{
		ID:                1,
		User:              &entity.User{ID: 2, Role: enum.RoleVisitor},
		Content:           "private content",
		Attachments:       []string{"private-image"},
		ModerationPending: true,
		HasReplies:        true,
	}

	for _, deleted := range []bool{false, true} {
		comment.Deleted = deleted
		visible := comment.ForViewer(nil, discussion, tenant, time.Now())
		if visible.Content != "" || visible.User != nil || len(visible.Attachments) != 0 {
			t.Fatalf("removed comment exposed its fields: %+v", visible)
		}

		if !visible.HasReplies || visible.ID != 1 {
			t.Fatal("removed comment lost its thread structure")
		}
	}

	if comment.Content != "private content" || comment.User == nil {
		t.Fatal("projection mutated the stored comment")
	}
}
