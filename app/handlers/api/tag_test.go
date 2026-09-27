package api_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/api"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	. "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/assert"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/mock"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestCreateTagHandler_ValidRequests(t *testing.T) {
	RegisterT(t)

	bus.AddHandler(func(ctx context.Context, q *query.GetTagBySlug) error {
		return app.ErrNotFound
	})

	var addNewTag *cmd.AddNewTag
	bus.AddHandler(func(ctx context.Context, c *cmd.AddNewTag) error {
		addNewTag = c
		return nil
	})

	server := mock.NewServer()
	status, _ := server.
		OnTenant(mock.DemoTenant).
		AsUser(mock.JonSnow).
		ExecutePost(
			api.CreateEditTag(),
			`{ "name": "Feature Request", "color": "00FF00", "isPublic": true }`,
		)

	Expect(status).Equals(http.StatusOK)

	Expect(addNewTag.Name).Equals("Feature Request")
	Expect(addNewTag.Color).Equals("00FF00")
	Expect(addNewTag.IsPublic).IsTrue()
}

func TestCreateTagHandler_InvalidRequests(t *testing.T) {
	RegisterT(t)

	bus.AddHandler(func(ctx context.Context, q *query.GetTagBySlug) error {
		return app.ErrNotFound
	})

	var testCases = []string{
		`{ }`,
		`{ "name": "" }`,
		`{ "name": "Bug" }`,
		`{ "name": "Bug", "color": "ABC" }`,
		`{ "name": "Bug", "color": "00000X" }`,
		`{ "name": "123456789012345678901234567890A", "color": "000000" }`,
	}

	for _, testCase := range testCases {
		server := mock.NewServer()
		status, _ := server.
			AsUser(mock.JonSnow).
			ExecutePostAsJSON(api.CreateEditTag(), testCase)

		Expect(status).Equals(http.StatusBadRequest)
	}
}

func TestCreateTagHandler_AlreadyInUse(t *testing.T) {
	RegisterT(t)

	tag := &entity.Tag{Name: "Bug", Slug: "bug"}
	bus.AddHandler(func(ctx context.Context, q *query.GetTagBySlug) error {
		q.Result = tag
		return nil
	})

	server := mock.NewServer()

	status, _ := server.
		AsUser(mock.JonSnow).
		ExecutePostAsJSON(
			api.CreateEditTag(),
			`{ "name": "Bug", "color": "0000FF", "isPublic": true }`,
		)

	Expect(status).Equals(http.StatusBadRequest)
}

func TestCreateTagHandler_Collaborator(t *testing.T) {
	RegisterT(t)

	server := mock.NewServer()
	status, _ := server.
		AsUser(mock.AryaStark).
		ExecutePost(
			api.CreateEditTag(),
			`{ "name": "Feature Request", "color": "000000", "isPublic": true }`,
		)

	Expect(status).Equals(http.StatusForbidden)
}

func TestEditInvalidTagHandler(t *testing.T) {
	RegisterT(t)

	bus.AddHandler(func(ctx context.Context, q *query.GetTagBySlug) error {
		return app.ErrNotFound
	})

	server := mock.NewServer()
	status, _ := server.
		AsUser(mock.JonSnow).
		AddParam("slug", "bug").
		ExecutePost(
			api.CreateEditTag(),
			`{ "name": "Feature Request", "color": "000000", "isPublic": true }`,
		)

	Expect(status).Equals(http.StatusNotFound)
}

func TestEditExistingTagHandler(t *testing.T) {
	RegisterT(t)

	tag := &entity.Tag{ID: 5, Name: "Bug", Slug: "bug", Color: "0000FF", IsPublic: true}
	bus.AddHandler(func(ctx context.Context, q *query.GetTagBySlug) error {
		q.Result = tag
		return nil
	})

	var updateTag *cmd.UpdateTag
	bus.AddHandler(func(ctx context.Context, c *cmd.UpdateTag) error {
		updateTag = c
		return nil
	})

	server := mock.NewServer()

	status, _ := server.
		OnTenant(mock.DemoTenant).
		AsUser(mock.JonSnow).
		AddParam("slug", "bug").
		ExecutePost(
			api.CreateEditTag(),
			`{ "name": "Feature Request", "color": "000000", "isPublic": true }`,
		)

	Expect(status).Equals(http.StatusOK)
	Expect(updateTag.TagID).Equals(5)
	Expect(updateTag.Name).Equals("Feature Request")
	Expect(updateTag.Color).Equals("000000")
	Expect(updateTag.IsPublic).IsTrue()
}

func TestDeleteInvalidTagHandler(t *testing.T) {
	RegisterT(t)

	bus.AddHandler(func(ctx context.Context, q *query.GetTagBySlug) error {
		return app.ErrNotFound
	})

	bus.AddHandler(func(ctx context.Context, q *query.GetUserProfileStanding) error {
		return nil
	})

	status, _ := mock.NewServer().
		AsUser(mock.JonSnow).
		AddParam("slug", "bug").
		Execute(api.DeleteTag())

	Expect(status).Equals(http.StatusNotFound)
}

func TestDeleteExistingTagHandler(t *testing.T) {
	RegisterT(t)

	tag := &entity.Tag{ID: 5, Name: "Bug", Slug: "bug", Color: "0000FF", IsPublic: true}
	bus.AddHandler(func(ctx context.Context, q *query.GetTagBySlug) error {
		q.Result = tag
		return nil
	})

	var deleteTag *cmd.DeleteTag
	bus.AddHandler(func(ctx context.Context, c *cmd.DeleteTag) error {
		deleteTag = c
		return nil
	})

	status, _ := mock.NewServer().
		OnTenant(mock.DemoTenant).
		AsUser(mock.JonSnow).
		AddParam("slug", "bug").
		Execute(api.DeleteTag())

	Expect(status).Equals(http.StatusOK)
	Expect(deleteTag.Tag).Equals(tag)
}

func TestDeleteExistingTagHandler_Collaborator(t *testing.T) {
	RegisterT(t)

	tag := &entity.Tag{ID: 5, Name: "Bug", Slug: "bug", Color: "0000FF", IsPublic: true}
	bus.AddHandler(func(ctx context.Context, q *query.GetTagBySlug) error {
		q.Result = tag
		return nil
	})

	bus.AddHandler(func(ctx context.Context, q *query.GetUserProfileStanding) error {
		return nil
	})

	status, _ := mock.NewServer().
		AsUser(mock.AryaStark).
		AddParam("slug", "bug").
		Execute(api.DeleteTag())

	Expect(status).Equals(http.StatusForbidden)
}

func TestAssignTagHandler_Success(t *testing.T) {
	RegisterT(t)

	post := &entity.Post{ID: 2, Number: 2}
	tag := &entity.Tag{ID: 5, Slug: "bug"}
	bus.AddHandler(func(ctx context.Context, q *query.GetPostByNumber) error {
		q.Result = post
		return nil
	})

	bus.AddHandler(func(ctx context.Context, q *query.GetTagBySlug) error {
		q.Result = tag
		return nil
	})

	var assignTag *cmd.AssignTag
	bus.AddHandler(func(ctx context.Context, c *cmd.AssignTag) error {
		assignTag = c
		return nil
	})

	status, _ := mock.NewServer().
		OnTenant(mock.DemoTenant).
		AsUser(mock.JonSnow).
		AddParam("slug", tag.Slug).
		AddParam("number", post.Number).
		Execute(api.AssignTag())

	Expect(status).Equals(http.StatusOK)
	Expect(assignTag.Post).Equals(post)
	Expect(assignTag.Tag).Equals(tag)
}

func TestAssignTagHandler_UnknownTag(t *testing.T) {
	RegisterT(t)

	post := &entity.Post{ID: 2, Number: 2}
	bus.AddHandler(func(ctx context.Context, q *query.GetPostByNumber) error {
		q.Result = post
		return nil
	})

	bus.AddHandler(func(ctx context.Context, q *query.GetTagBySlug) error {
		return app.ErrNotFound
	})

	bus.AddHandler(func(ctx context.Context, q *query.GetUserProfileStanding) error {
		return nil
	})

	status, _ := mock.NewServer().
		OnTenant(mock.DemoTenant).
		AsUser(mock.JonSnow).
		AddParam("slug", "bug").
		AddParam("number", post.Number).
		Execute(api.AssignTag())

	Expect(status).Equals(http.StatusNotFound)
}

func TestAssignOrUnassignTagHandler_Unauthorized(t *testing.T) {
	RegisterT(t)

	bus.AddHandler(func(ctx context.Context, q *query.GetUserProfileStanding) error {
		return nil
	})

	var testCases = []web.HandlerFunc{
		api.AssignTag(),
		api.UnassignTag(),
	}

	for _, handler := range testCases {
		status, _ := mock.NewServer().
			OnTenant(mock.DemoTenant).
			AsUser(mock.AryaStark).
			AddParam("slug", "feature-request").
			AddParam("number", "500").
			Execute(handler)

		Expect(status).Equals(http.StatusForbidden)
	}
}

func TestUnassignTagHandler_Success(t *testing.T) {
	RegisterT(t)

	post := &entity.Post{ID: 2, Number: 2}
	tag := &entity.Tag{ID: 5, Slug: "bug"}
	bus.AddHandler(func(ctx context.Context, q *query.GetPostByNumber) error {
		q.Result = post
		return nil
	})

	bus.AddHandler(func(ctx context.Context, q *query.GetTagBySlug) error {
		q.Result = tag
		return nil
	})

	var unassignTag *cmd.UnassignTag
	bus.AddHandler(func(ctx context.Context, c *cmd.UnassignTag) error {
		unassignTag = c
		return nil
	})

	status, _ := mock.NewServer().
		OnTenant(mock.DemoTenant).
		AsUser(mock.JonSnow).
		AddParam("slug", tag.Slug).
		AddParam("number", post.Number).
		Execute(api.UnassignTag())

	Expect(status).Equals(http.StatusOK)
	Expect(unassignTag.Post).Equals(post)
	Expect(unassignTag.Tag).Equals(tag)
}

func TestListTagsHandler(t *testing.T) {
	RegisterT(t)

	tag1 := &entity.Tag{ID: 2, Name: "Bug", Slug: "bug", Color: "0000FF", IsPublic: true}
	tag2 := &entity.Tag{ID: 5, Name: "Feature Request", Slug: "feature-request", Color: "0000FF", IsPublic: true}
	bus.AddHandler(func(ctx context.Context, q *query.GetAllTags) error {
		q.Result = []*entity.Tag{tag1, tag2}
		return nil
	})

	server := mock.NewServer()

	status, query := server.
		AsUser(mock.JonSnow).
		ExecuteAsJSON(api.ListTags())

	Expect(status).Equals(http.StatusOK)
	Expect(query.IsArray()).IsTrue()
	Expect(query.ArrayLength()).Equals(2)
}
