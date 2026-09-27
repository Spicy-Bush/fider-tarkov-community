package actions

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/i18n"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/profanity"
	"github.com/gosimple/slug"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

// CreateNewPost is used to create a new post
type CreateNewPost struct {
	Title        string             `json:"title"`
	Description  string             `json:"description"`
	TagSlugs     []string           `json:"tags"`
	Attachments  []*dto.ImageUpload `json:"attachments"`
	SubmissionID string             `json:"submissionId"`

	Tags []*entity.Tag `json:"-"`
}

// OnPreExecute prefetches Tags for later use
func (input *CreateNewPost) OnPreExecute(ctx context.Context) error {
	if env.Config.PostCreationWithTagsEnabled {
		input.Tags = make([]*entity.Tag, 0, len(input.TagSlugs))
		for _, slug := range input.TagSlugs {
			getTag := &query.GetTagBySlug{Slug: slug}
			if err := bus.Dispatch(ctx, getTag); err != nil {
				break
			}

			input.Tags = append(input.Tags, getTag.Result)
		}
	}

	return nil
}

// IsAuthorized returns true if current user is authorized to perform this action
func (action *CreateNewPost) IsAuthorized(ctx context.Context, user *entity.User) bool {
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	if !entity.Can(user, tenant, entity.CreatePosts) {
		return false
	}

	if env.Config.PostCreationWithTagsEnabled {
		for _, tag := range action.Tags {
			if !tag.IsPublic && !tag.AllowedActions(user, tenant).Assign {
				return false
			}
		}
	}

	return true
}

// Validate if current model is valid
func (action *CreateNewPost) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	tenant := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	generalSettings := tenant.GeneralSettings

	if !entity.Can(user, tenant, entity.CreatePosts) {
		result.AddFieldFailure("title", i18n.T(ctx, "validation.custom.postingdisabled"))
		return result
	}

	if !user.IsCollaborator() && !user.IsModerator() && !user.IsAdministrator() {
		if limit, ok := generalSettings.PostLimits[user.Role.String()]; ok && limit.Count > 0 {
			q := &query.GetUserPostCount{
				UserID: user.ID,
				Since:  time.Now().Add(-time.Duration(limit.Hours) * time.Hour),
			}
			if err := bus.Dispatch(ctx, q); err != nil {
				return validate.Error(err)
			}
			if q.Result >= limit.Count {
				result.AddFieldFailure("title", i18n.T(ctx, "validation.custom.toomanyposts"))
				return result
			}
		}
	}

	if len(strings.TrimSpace(action.Title)) == 0 {
		result.AddFieldFailure("title", propertyIsRequired(ctx, "title"))
	} else if len(action.Title) < generalSettings.TitleLengthMin {
		result.AddFieldFailure("title", i18n.T(ctx, "validation.custom.titletooshort", i18n.Params{"min": generalSettings.TitleLengthMin}))
	} else if len(action.Title) > generalSettings.TitleLengthMax {
		result.AddFieldFailure("title", i18n.T(ctx, "validation.custom.titletoolong", i18n.Params{"max": generalSettings.TitleLengthMax}))
	} else if env.Config.PostCreationWithTagsEnabled && len(action.TagSlugs) != len(action.Tags) {
		result.AddFieldFailure("tags", propertyIsInvalid(ctx, "tags"))
	} else if action.Description == "" {
		result.AddFieldFailure("description", propertyIsRequired(ctx, "description"))
	} else if len(action.Description) < generalSettings.DescriptionLengthMin {
		result.AddFieldFailure("description", i18n.T(ctx, "validation.custom.descriptiontooshort", i18n.Params{"min": generalSettings.DescriptionLengthMin}))
	} else if len(action.Description) > generalSettings.DescriptionLengthMax {
		result.AddFieldFailure("description", i18n.T(ctx, "validation.custom.descriptiontoolong", i18n.Params{"max": generalSettings.DescriptionLengthMax}))
	} else if matches, err := profanity.ContainsProfanity(ctx, action.Title); err == nil && len(matches) > 0 {
		result.AddFieldFailure("title", i18n.T(ctx, "validation.custom.containsprofanity"))
	} else if matches, err := profanity.ContainsProfanity(ctx, action.Description); err == nil && len(matches) > 0 {
		result.AddFieldFailure("description", i18n.T(ctx, "validation.custom.containsprofanity"))
	} else {
		err := bus.Dispatch(ctx, &query.GetPostBySlug{Slug: slug.Make(action.Title)})
		if err != nil && errors.Cause(err) != app.ErrNotFound {
			return validate.Error(err)
		} else if err == nil {
			result.AddFieldFailure("title", i18n.T(ctx, "validation.custom.duplicatetitle"))
		}
	}

	messages, err := validate.MultiImageUpload(ctx, nil, action.Attachments, validate.MultiImageUploadOpts{
		MaxUploads:   generalSettings.MaxImagesPerPost,
		MaxKilobytes: 7500,
		ExactRatio:   false,
	})
	if err != nil {
		return validate.Error(err)
	}
	result.AddFieldFailure("attachments", messages...)

	return result
}

// UpdatePost is used to edit an existing new post
type UpdatePost struct {
	Number      int                `route:"number"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Attachments []*dto.ImageUpload `json:"attachments"`

	Post *entity.Post
}

// OnPreExecute prefetches Post for later use
func (input *UpdatePost) OnPreExecute(ctx context.Context) error {
	getPost := &query.GetPostByNumber{Number: input.Number}
	if err := bus.Dispatch(ctx, getPost); err != nil {
		return err
	}

	input.Post = getPost.Result
	return nil
}

// IsAuthorized returns true if current user is authorized to perform this action
func (input *UpdatePost) IsAuthorized(ctx context.Context, user *entity.User) bool {
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	return input.Post.AllowedActions(user, tenant, time.Now()).Edit
}

// Validate if current model is valid
func (action *UpdatePost) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	tenant := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	generalSettings := tenant.GeneralSettings

	// TODO: refactor these if else blocks >.<
	if action.Title == "" {
		result.AddFieldFailure("title", propertyIsRequired(ctx, "title"))
	} else if len(action.Title) < generalSettings.TitleLengthMin {
		result.AddFieldFailure("title", i18n.T(ctx, "validation.custom.titletooshort", i18n.Params{"min": generalSettings.TitleLengthMin}))
	} else if len(action.Title) > generalSettings.TitleLengthMax {
		result.AddFieldFailure("title", i18n.T(ctx, "validation.custom.titletoolong", i18n.Params{"max": generalSettings.TitleLengthMax}))
	} else if matches, err := profanity.ContainsProfanity(ctx, action.Title); err == nil && len(matches) > 0 {
		result.AddFieldFailure("title", i18n.T(ctx, "validation.custom.containsprofanity"))
	} else if matches, err := profanity.ContainsProfanity(ctx, action.Description); err == nil && len(matches) > 0 {
		result.AddFieldFailure("description", i18n.T(ctx, "validation.custom.containsprofanity"))
	}

	postBySlug := &query.GetPostBySlug{Slug: slug.Make(action.Title)}
	err := bus.Dispatch(ctx, postBySlug)
	if err != nil && errors.Cause(err) != app.ErrNotFound {
		return validate.Error(err)
	} else if err == nil && postBySlug.Result.ID != action.Post.ID {
		result.AddFieldFailure("title", i18n.T(ctx, "validation.custom.duplicatetitle"))
	}

	if len(action.Attachments) > 0 {
		getAttachments := &query.GetPostAttachments{PostID: action.Post.ID}
		err = bus.Dispatch(ctx, getAttachments)
		if err != nil {
			return validate.Error(err)
		}

		messages, err := validate.MultiImageUpload(ctx, getAttachments.Result, action.Attachments, validate.MultiImageUploadOpts{
			MaxUploads:   generalSettings.MaxImagesPerPost,
			MaxKilobytes: 7500,
			ExactRatio:   false,
		})
		if err != nil {
			return validate.Error(err)
		}
		result.AddFieldFailure("attachments", messages...)
	}

	return result
}

// SetResponse represents the action to update an post response
type SetResponse struct {
	Number         int              `route:"number"`
	Status         *enum.PostStatus `json:"status"`
	Text           string           `json:"text"`
	OriginalNumber int              `json:"originalNumber"`

	Original *entity.Post
}

// IsAuthorized returns true if current user is authorized to perform this action
func (action *SetResponse) IsAuthorized(ctx context.Context, user *entity.User) bool {
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	return entity.Can(user, tenant, entity.RespondToPosts)
}

// Validate if current model is valid
func (action *SetResponse) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()
	if action.Status == nil {
		result.AddFieldFailure("status", propertyIsRequired(ctx, "status"))
		return result
	}

	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	if !slices.Contains(entity.AllowedPostResponses(user, tenant), *action.Status) {
		result.AddFieldFailure("status", propertyIsInvalid(ctx, "status"))
		return result
	}

	if *action.Status == enum.PostDuplicate {
		if action.OriginalNumber == action.Number {
			result.AddFieldFailure("originalNumber", i18n.T(ctx, "validation.custom.selfduplicate"))
		}

		getOriginaPost := &query.GetPostByNumber{Number: action.OriginalNumber}
		err := bus.Dispatch(ctx, getOriginaPost)
		if err != nil {
			if errors.Cause(err) == app.ErrNotFound {
				result.AddFieldFailure("originalNumber", i18n.T(ctx, "validation.custom.originalpostnotfound"))
			} else {
				return validate.Error(err)
			}
		}

		if getOriginaPost.Result != nil {
			action.Original = getOriginaPost.Result
		}
	}

	return result
}

// DeletePost represents the action of an administrator deleting an existing Post
type DeletePost struct {
	Number int    `route:"number"`
	Text   string `json:"text"`

	Post *entity.Post
}

// OnPreExecute prefetches Post for later use
func (action *DeletePost) OnPreExecute(ctx context.Context) error {
	getPost := &query.GetPostByNumber{Number: action.Number}
	if err := bus.Dispatch(ctx, getPost); err != nil {
		return err
	}

	action.Post = getPost.Result
	return nil
}

// IsAuthorized returns true if current user is authorized to perform this action
func (action *DeletePost) IsAuthorized(ctx context.Context, user *entity.User) bool {
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	return action.Post.AllowedActions(user, tenant, time.Now()).Delete
}

// Validate if current model is valid
func (action *DeletePost) Validate(ctx context.Context, user *entity.User) *validate.Result {
	if action.Post == nil {
		return validate.Failed(i18n.T(ctx, "validation.custom.invalidpost"))
	}

	isReferencedQuery := &query.PostIsReferenced{PostID: action.Post.ID}
	if err := bus.Dispatch(ctx, isReferencedQuery); err != nil {
		return validate.Error(err)
	}

	if isReferencedQuery.Result {
		return validate.Failed(i18n.T(ctx, "validation.custom.cannotdeleteduplicatepost"))
	}

	return validate.Success()
}

type LockPost struct {
	Number      int    `route:"number"`
	LockMessage string `json:"message"`

	Post *entity.Post
}

func (input *LockPost) OnPreExecute(ctx context.Context) error {
	getPost := &query.GetPostByNumber{Number: input.Number}
	if err := bus.Dispatch(ctx, getPost); err != nil {
		return err
	}

	input.Post = getPost.Result
	return nil
}

func (action *LockPost) IsAuthorized(ctx context.Context, user *entity.User) bool {
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	return action.Post.AllowedActions(user, tenant, time.Now()).Lock
}

func (action *LockPost) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	if action.Post == nil {
		result.AddFieldFailure("number", i18n.T(ctx, "validation.custom.invalidpost"))
	}

	return result
}

type UnlockPost struct {
	Number int `route:"number"`

	Post *entity.Post
}

func (input *UnlockPost) OnPreExecute(ctx context.Context) error {
	getPost := &query.GetPostByNumber{Number: input.Number}
	if err := bus.Dispatch(ctx, getPost); err != nil {
		return err
	}

	input.Post = getPost.Result
	return nil
}

func (action *UnlockPost) IsAuthorized(ctx context.Context, user *entity.User) bool {
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	return action.Post.AllowedActions(user, tenant, time.Now()).Lock
}

func (action *UnlockPost) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	if action.Post == nil {
		result.AddFieldFailure("number", i18n.T(ctx, "validation.custom.invalidpost"))
	}

	return result
}
