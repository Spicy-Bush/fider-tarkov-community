package postgres_test

import (
	"context"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	. "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/assert"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
)

func TestPrivateTagVisibilityUsesTenantPermissions(t *testing.T) {
	SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	tag := &cmd.AddNewTag{Name: "Private", Color: "123456"}
	post := &cmd.AddNewPost{Title: "Tagged post", Description: "Content"}
	if err := bus.Dispatch(jonSnowCtx, tag, post); err != nil {
		t.Fatal(err)
	}

	if err := bus.Dispatch(jonSnowCtx, &cmd.AssignTag{Tag: tag.Result, Post: post.Result}); err != nil {
		t.Fatal(err)
	}

	for _, candidate := range []struct {
		name    string
		role    enum.Role
		grants  map[entity.Permission]bool
		visible bool
	}{
		{name: "helper", role: enum.RoleHelper},
		{name: "delegated manager", role: enum.RoleHelper, grants: map[entity.Permission]bool{entity.ManageTags: true}, visible: true},
		{name: "delegated reader", role: enum.RoleHelper, grants: map[entity.Permission]bool{entity.ViewPrivateTags: true}, visible: true},
		{name: "moderator", role: enum.RoleModerator, visible: true},
		{name: "revoked moderator", role: enum.RoleModerator, grants: map[entity.Permission]bool{entity.ViewPrivateTags: false}},
		{name: "revoked collaborator", role: enum.RoleCollaborator, grants: map[entity.Permission]bool{entity.ViewPrivateTags: false, entity.ManageTags: false}},
	} {
		t.Run(candidate.name, func(t *testing.T) {
			tenant := *demoTenant
			changes := make([]entity.RolePermissionChange, 0, len(candidate.grants))
			for permission, granted := range candidate.grants {
				changes = append(changes, entity.RolePermissionChange{Role: candidate.role, Permission: permission, Granted: granted})
			}
			permissions, blocked := tenant.RolePermissions.ApplyChanges(jonSnow, &tenant, changes)
			if blocked != "" {
				t.Fatal(blocked)
			}
			tenant.RolePermissions = permissions

			user := *aryaStark
			user.Role = candidate.role
			ctx := context.WithValue(aryaStarkCtx, app.TenantCtxKey, &tenant)
			ctx = context.WithValue(ctx, app.UserCtxKey, &user)

			tags := &query.GetAllTags{}
			loadedPost := &query.GetPostByID{PostID: post.Result.ID}
			if err := bus.Dispatch(ctx, tags, loadedPost); err != nil {
				t.Fatal(err)
			}

			if (len(tags.Result) == 1) != candidate.visible || (len(loadedPost.Result.Tags) == 1) != candidate.visible {
				t.Fatalf("private tag visibility: list=%v post=%v, want %v", tags.Result, loadedPost.Result.Tags, candidate.visible)
			}

			if tag.Result.AllowedActions(&user, &tenant).Assign != candidate.visible {
				t.Fatalf("private tag assignment disagrees with visibility")
			}
		})
	}
}

func TestTagStorage_AddAndGet(t *testing.T) {
	SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	addNewTag := &cmd.AddNewTag{Name: "Feature Request", Color: "FF0000", IsPublic: true}
	err := bus.Dispatch(demoTenantCtx, addNewTag)
	Expect(err).IsNil()
	Expect(addNewTag.Result.Slug).Equals("feature-request")

	getTag := &query.GetTagBySlug{Slug: addNewTag.Result.Slug}
	err = bus.Dispatch(demoTenantCtx, getTag)
	Expect(err).IsNil()
	Expect(getTag.Result.ID).NotEquals(0)
	Expect(getTag.Result.Name).Equals("Feature Request")
	Expect(getTag.Result.Slug).Equals("feature-request")
	Expect(getTag.Result.Color).Equals("FF0000")
	Expect(getTag.Result.IsPublic).IsTrue()
}

func TestTagStorage_AddUpdateAndGet(t *testing.T) {
	SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	addNewTag := &cmd.AddNewTag{Name: "Feature Request", Color: "FF0000", IsPublic: true}
	err := bus.Dispatch(demoTenantCtx, addNewTag)
	Expect(err).IsNil()

	updateTag := &cmd.UpdateTag{TagID: addNewTag.Result.ID, Name: "Bug", Color: "000000", IsPublic: false}
	err = bus.Dispatch(demoTenantCtx, updateTag)
	Expect(err).IsNil()

	getTag := &query.GetTagBySlug{Slug: updateTag.Result.Slug}
	err = bus.Dispatch(demoTenantCtx, getTag)
	Expect(err).IsNil()
	Expect(getTag.Result.ID).Equals(addNewTag.Result.ID)
	Expect(getTag.Result.Name).Equals("Bug")
	Expect(getTag.Result.Slug).Equals("bug")
	Expect(getTag.Result.Color).Equals("000000")
	Expect(getTag.Result.IsPublic).IsFalse()
}

func TestTagStorage_AddDeleteAndGet(t *testing.T) {
	SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	addNewTag := &cmd.AddNewTag{Name: "Bug", Color: "FFFFFF", IsPublic: true}
	err := bus.Dispatch(demoTenantCtx, addNewTag)
	Expect(err).IsNil()

	err = bus.Dispatch(demoTenantCtx, &cmd.DeleteTag{Tag: addNewTag.Result})
	Expect(err).IsNil()

	getTag := &query.GetTagBySlug{Slug: addNewTag.Result.Slug}
	err = bus.Dispatch(demoTenantCtx, getTag)
	Expect(errors.Cause(err)).Equals(app.ErrNotFound)
	Expect(getTag.Result).IsNil()
}

func TestTagStorage_Assign_Unassign(t *testing.T) {
	SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	newPost := &cmd.AddNewPost{Title: "My new post", Description: "with this description"}
	err := bus.Dispatch(aryaStarkCtx, newPost)
	Expect(err).IsNil()

	addNewTag := &cmd.AddNewTag{Name: "Bug", Color: "FFFFFF", IsPublic: true}
	err = bus.Dispatch(jonSnowCtx, addNewTag)
	Expect(err).IsNil()

	err = bus.Dispatch(jonSnowCtx, &cmd.AssignTag{Tag: addNewTag.Result, Post: newPost.Result})
	Expect(err).IsNil()

	assignedTags := &query.GetAssignedTags{Post: newPost.Result}
	err = bus.Dispatch(demoTenantCtx, assignedTags)
	Expect(err).IsNil()
	Expect(assignedTags.Result).HasLen(1)
	Expect(assignedTags.Result[0].ID).Equals(addNewTag.Result.ID)
	Expect(assignedTags.Result[0].Name).Equals("Bug")
	Expect(assignedTags.Result[0].Slug).Equals("bug")
	Expect(assignedTags.Result[0].Color).Equals("FFFFFF")
	Expect(assignedTags.Result[0].IsPublic).IsTrue()

	err = bus.Dispatch(jonSnowCtx, &cmd.UnassignTag{Tag: addNewTag.Result, Post: newPost.Result})
	Expect(err).IsNil()

	assignedTags = &query.GetAssignedTags{Post: newPost.Result}
	err = bus.Dispatch(jonSnowCtx, assignedTags)
	Expect(err).IsNil()
	Expect(assignedTags.Result).HasLen(0)
}

func TestTagStorage_Assign_DeleteTag(t *testing.T) {
	SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	newPost := &cmd.AddNewPost{Title: "My new post", Description: "with this description"}
	err := bus.Dispatch(aryaStarkCtx, newPost)
	Expect(err).IsNil()

	addNewTag := &cmd.AddNewTag{Name: "Bug", Color: "FFFFFF", IsPublic: true}
	err = bus.Dispatch(jonSnowCtx, addNewTag)
	Expect(err).IsNil()

	err = bus.Dispatch(jonSnowCtx, &cmd.AssignTag{Tag: addNewTag.Result, Post: newPost.Result})
	Expect(err).IsNil()

	err = bus.Dispatch(jonSnowCtx, &cmd.DeleteTag{Tag: addNewTag.Result})
	Expect(err).IsNil()

	assignedTags := &query.GetAssignedTags{Post: newPost.Result}
	err = bus.Dispatch(jonSnowCtx, assignedTags)
	Expect(err).IsNil()
	Expect(assignedTags.Result).HasLen(0)
}

func TestTagStorage_GetAll(t *testing.T) {
	SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	err := bus.Dispatch(demoTenantCtx, &cmd.AddNewTag{Name: "Feature Request", Color: "FF0000", IsPublic: true})
	Expect(err).IsNil()
	err = bus.Dispatch(demoTenantCtx, &cmd.AddNewTag{Name: "Bug", Color: "0F0F0F", IsPublic: false})
	Expect(err).IsNil()

	getAllTags := &query.GetAllTags{}
	err = bus.Dispatch(jonSnowCtx, getAllTags)
	Expect(err).IsNil()
	Expect(getAllTags.Result).HasLen(2)

	Expect(getAllTags.Result[0].ID).NotEquals(0)
	Expect(getAllTags.Result[0].Name).Equals("Bug")
	Expect(getAllTags.Result[0].Slug).Equals("bug")
	Expect(getAllTags.Result[0].Color).Equals("0F0F0F")
	Expect(getAllTags.Result[0].IsPublic).IsFalse()

	Expect(getAllTags.Result[1].ID).NotEquals(0)
	Expect(getAllTags.Result[1].Name).Equals("Feature Request")
	Expect(getAllTags.Result[1].Slug).Equals("feature-request")
	Expect(getAllTags.Result[1].Color).Equals("FF0000")
	Expect(getAllTags.Result[1].IsPublic).IsTrue()

	err = bus.Dispatch(aryaStarkCtx, getAllTags)
	Expect(err).IsNil()
	Expect(getAllTags.Result).HasLen(1)
	Expect(getAllTags.Result[0].Name).Equals("Feature Request")
}
