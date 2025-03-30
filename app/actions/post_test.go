package actions_test

import (
	"context"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"

	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	. "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/assert"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
)

// createTestContext creates a context with a mock tenant for testing
func createTestContext() context.Context {
	tenant := &entity.Tenant{
		GeneralSettings: &entity.GeneralSettings{
			TitleLengthMin:             5,
			TitleLengthMax:             100,
			DescriptionLengthMin:       10,
			DescriptionLengthMax:       500,
			PostingGloballyDisabled:    false,
			CommentingGloballyDisabled: false,
			PostingDisabledFor:         []string{},
			CommentingDisabledFor:      []string{},
			PostLimits:                 map[string]entity.PostLimit{},
			CommentLimits:              map[string]entity.CommentLimit{},
			MaxImagesPerPost:           5,
			MaxImagesPerComment:        3,
		},
	}
	return context.WithValue(context.Background(), app.TenantCtxKey, tenant)
}

func TestCreateNewPost_ValidPostTitles(t *testing.T) {
	RegisterT(t)

	bus.AddHandler(func(ctx context.Context, q *query.GetPostBySlug) error {
		return app.ErrNotFound
	})

	ctx := createTestContext()
	user := &entity.User{
		ID:   1,
		Role: enum.RoleVisitor,
	}

	for _, title := range []string{
		"this is my new post",
		"this post is very descriptive",
	} {
		action := &actions.CreateNewPost{
			Title:       title,
			Description: "This is a description with more than 10 characters", // Add valid description
		}
		result := action.Validate(ctx, user)
		ExpectSuccess(result)
	}
}

func TestSetResponse_InvalidStatus(t *testing.T) {
	RegisterT(t)

	ctx := createTestContext()
	action := &actions.SetResponse{
		Status: enum.PostDeleted,
		Text:   "Spam!",
	}
	result := action.Validate(ctx, nil)
	ExpectFailed(result, "status")
}

func TestDeletePost_WhenIsBeingReferenced(t *testing.T) {
	RegisterT(t)

	post1 := &entity.Post{ID: 1, Number: 1, Title: "Post 1"}
	post2 := &entity.Post{ID: 2, Number: 2, Title: "Post 2"}

	bus.AddHandler(func(ctx context.Context, q *query.GetPostByNumber) error {
		if q.Number == post1.Number {
			q.Result = post1
			return nil
		}

		if q.Number == post2.Number {
			q.Result = post2
			return nil
		}

		return app.ErrNotFound
	})

	bus.AddHandler(func(ctx context.Context, q *query.PostIsReferenced) error {
		q.Result = q.PostID == post2.ID
		return nil
	})

	ctx := createTestContext()
	action := &actions.DeletePost{}
	action.Number = post1.Number
	ExpectSuccess(action.Validate(ctx, nil))

	action.Number = post2.Number
	ExpectFailed(action.Validate(ctx, nil))
}

func TestDeleteComment(t *testing.T) {
	RegisterT(t)

	author := &entity.User{ID: 1, Role: enum.RoleVisitor}
	notAuthor := &entity.User{ID: 2, Role: enum.RoleVisitor}
	administrator := &entity.User{ID: 3, Role: enum.RoleAdministrator}
	comment := &entity.Comment{
		ID:      1,
		User:    author,
		Content: "Comment #1",
	}

	bus.AddHandler(func(ctx context.Context, q *query.GetCommentByID) error {
		if q.CommentID == comment.ID {
			q.Result = comment
			return nil
		}
		return app.ErrNotFound
	})

	ctx := createTestContext()
	action := &actions.DeleteComment{
		CommentID: comment.ID,
	}

	authorized := action.IsAuthorized(ctx, notAuthor)
	Expect(authorized).IsFalse()

	authorized = action.IsAuthorized(ctx, author)
	Expect(authorized).IsTrue()

	authorized = action.IsAuthorized(ctx, administrator)
	Expect(authorized).IsTrue()
}
