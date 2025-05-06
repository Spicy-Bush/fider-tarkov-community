package actions

import (
	"context"
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
	Title       string             `json:"title"`
	Description string             `json:"description"`
	TagSlugs    []string           `json:"tags"`
	Attachments []*dto.ImageUpload `json:"attachments"`

	Tags []*entity.Tag
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
	if user == nil {
		return false
	} else if env.Config.PostCreationWithTagsEnabled && (!user.IsCollaborator() || !user.IsModerator()) {
		for _, tag := range action.Tags {
			if !tag.IsPublic {
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

	if generalSettings != nil && generalSettings.PostingGloballyDisabled && !(user.IsCollaborator() || user.IsAdministrator()) {
		result.AddFieldFailure("title", i18n.T(ctx, "validation.custom.postinggloballydisabled"))
		return result
	}

	for _, role := range generalSettings.PostingDisabledFor {
		if user.Role.String() == role {
			result.AddFieldFailure("title", i18n.T(ctx, "validation.custom.postingdisabled"))
			return result
		}
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
	if user == nil {
		return false
	}

	// If user is collaborator or admin, they can edit any post
	if user.IsCollaborator() || user.IsAdministrator() {
		return true
	}

	// If user is moderator, they can only edit posts from regular users
	if user.IsModerator() {
		return input.Post.User.Role == enum.RoleVisitor
	}

	// Regular users can only edit their own posts within 1 hour
	timeAgo := time.Now().UTC().Sub(input.Post.CreatedAt)
	return input.Post.User.ID == user.ID && timeAgo <= 1*time.Hour
}

// Validate if current model is valid
func (action *UpdatePost) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	tenant := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	generalSettings := tenant.GeneralSettings

	if generalSettings != nil && generalSettings.PostingGloballyDisabled && !(user.IsCollaborator() || user.IsAdministrator()) {
		result.AddFieldFailure("title", i18n.T(ctx, "validation.custom.postinggloballydisabled"))
		return result
	}

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
		getAttachments := &query.GetAttachments{Post: action.Post}
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

type ToggleCommentReaction struct {
	Number    int    `route:"number"`
	CommentID int    `route:"id"`
	Reaction  string `route:"reaction"`

	Post    *entity.Post
	Comment *entity.Comment
}

// OnPreExecute prefetches Post and Comment for later use
func (action *ToggleCommentReaction) OnPreExecute(ctx context.Context) error {
	getPost := &query.GetPostByNumber{Number: action.Number}
	commentByID := &query.GetCommentByID{CommentID: action.CommentID}
	if err := bus.Dispatch(ctx, getPost, commentByID); err != nil {
		return err
	}

	action.Post = getPost.Result
	action.Comment = commentByID.Result
	return nil
}

// IsAuthorized returns true if current user is authorized to perform this action
func (action *ToggleCommentReaction) IsAuthorized(ctx context.Context, user *entity.User) bool {
	if user == nil {
		return false
	}

	// If post is locked, only collaborators and administrators can add reactions
	if action.Post.IsLocked() {
		return user.IsCollaborator() || user.IsAdministrator()
	}

	return true
}

// Validate if current model is valid
func (action *ToggleCommentReaction) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	allowedEmojis := []string{"👍", "👎", "❤️", "🤔", "👏", "😂", "😲"}
	isAllowed := false
	for _, emoji := range allowedEmojis {
		if action.Reaction == emoji {
			isAllowed = true
			break
		}
	}

	if !isAllowed {
		result.AddFieldFailure("reaction", i18n.T(ctx, "validation.custom.invalidemoji"))
	}

	return result
}

// AddNewComment represents a new comment to be added
type AddNewComment struct {
	Number      int                `route:"number"`
	Content     string             `json:"content"`
	Attachments []*dto.ImageUpload `json:"attachments"`
}

// IsAuthorized returns true if current user is authorized to perform this action
func (action *AddNewComment) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil
}

// Validate if current model is valid
func (action *AddNewComment) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	tenant := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	generalSettings := tenant.GeneralSettings

	if generalSettings != nil && generalSettings.CommentingGloballyDisabled && !(user.IsCollaborator() || user.IsAdministrator()) {
		result.AddFieldFailure("content", i18n.T(ctx, "validation.custom.commentinggloballydisabled"))
		return result
	}

	for _, role := range generalSettings.CommentingDisabledFor {
		if user.Role.String() == role {
			result.AddFieldFailure("content", i18n.T(ctx, "validation.custom.commentingdisabled"))
			return result
		}
	}

	if !user.IsCollaborator() && !user.IsModerator() && !user.IsAdministrator() {
		if limit, ok := generalSettings.CommentLimits[user.Role.String()]; ok && limit.Count > 0 {
			q := &query.GetUserCommentCount{
				UserID: user.ID,
				Since:  time.Now().Add(-time.Duration(limit.Hours) * time.Hour),
			}
			if err := bus.Dispatch(ctx, q); err != nil {
				return validate.Error(err)
			}
			if q.Result >= limit.Count {
				result.AddFieldFailure("content", i18n.T(ctx, "validation.custom.toomanycomments"))
				return result
			}
		}
	}

	if action.Content == "" {
		result.AddFieldFailure("content", propertyIsRequired(ctx, "comment"))
	} else if matches, err := profanity.ContainsProfanity(ctx, action.Content); err == nil && len(matches) > 0 {
		result.AddFieldFailure("content", i18n.T(ctx, "validation.custom.containsprofanity"))
	}

	messages, err := validate.MultiImageUpload(ctx, nil, action.Attachments, validate.MultiImageUploadOpts{
		MaxUploads:   generalSettings.MaxImagesPerComment,
		MaxKilobytes: 7500,
		ExactRatio:   false,
	})
	if err != nil {
		return validate.Error(err)
	}
	result.AddFieldFailure("attachments", messages...)

	return result
}

// SetResponse represents the action to update an post response
type SetResponse struct {
	Number         int             `route:"number"`
	Status         enum.PostStatus `json:"status"`
	Text           string          `json:"text"`
	OriginalNumber int             `json:"originalNumber"`

	Original *entity.Post
}

// IsAuthorized returns true if current user is authorized to perform this action
func (action *SetResponse) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil && (user.IsAdministrator() || user.IsCollaborator() || user.IsModerator())
}

// Validate if current model is valid
func (action *SetResponse) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	if action.Status < enum.PostOpen || action.Status > enum.PostDuplicate {
		result.AddFieldFailure("status", propertyIsInvalid(ctx, "status"))
	}

	if action.Status == enum.PostDuplicate {
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

// IsAuthorized returns true if current user is authorized to perform this action
func (action *DeletePost) IsAuthorized(ctx context.Context, user *entity.User) bool {
	if user == nil {
		return false
	}

	// If user is collaborator or admin, they can delete any post
	if user.IsCollaborator() || user.IsAdministrator() {
		return true
	}

	// If user is moderator, they can only delete posts from regular users
	if user.IsModerator() {
		return action.Post.User.Role == enum.RoleVisitor
	}

	return false
}

// Validate if current model is valid
func (action *DeletePost) Validate(ctx context.Context, user *entity.User) *validate.Result {
	getPost := &query.GetPostByNumber{Number: action.Number}
	if err := bus.Dispatch(ctx, getPost); err != nil {
		return validate.Error(err)
	}

	action.Post = getPost.Result

	isReferencedQuery := &query.PostIsReferenced{PostID: action.Post.ID}
	if err := bus.Dispatch(ctx, isReferencedQuery); err != nil {
		return validate.Error(err)
	}

	if isReferencedQuery.Result {
		return validate.Failed(i18n.T(ctx, "validation.custom.cannotdeleteduplicatepost"))
	}

	return validate.Success()
}

// EditComment represents the action to update an existing comment
type EditComment struct {
	PostNumber  int                `route:"number"`
	ID          int                `route:"id"`
	Content     string             `json:"content"`
	Attachments []*dto.ImageUpload `json:"attachments"`

	Post    *entity.Post
	Comment *entity.Comment
}

// IsAuthorized returns true if current user is authorized to perform this action
func (action *EditComment) IsAuthorized(ctx context.Context, user *entity.User) bool {
	postByNumber := &query.GetPostByNumber{Number: action.PostNumber}
	commentByID := &query.GetCommentByID{CommentID: action.ID}
	if err := bus.Dispatch(ctx, postByNumber, commentByID); err != nil {
		return false
	}

	action.Post = postByNumber.Result
	action.Comment = commentByID.Result

	// If user is collaborator or admin, they can edit any comment
	if user.IsCollaborator() || user.IsAdministrator() {
		return true
	}

	// If user is moderator, they can only edit comments from regular users
	if user.IsModerator() {
		return action.Comment.User.Role == enum.RoleVisitor
	}

	// Regular users can only edit their own comments
	return user.ID == action.Comment.User.ID
}

// Validate if current model is valid
func (action *EditComment) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	tenant := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	generalSettings := tenant.GeneralSettings

	if generalSettings != nil && generalSettings.CommentingGloballyDisabled && !(user.IsCollaborator() || user.IsAdministrator()) {
		result.AddFieldFailure("content", i18n.T(ctx, "validation.custom.commentinggloballydisabled"))
		return result
	}

	if action.Content == "" {
		result.AddFieldFailure("content", propertyIsRequired(ctx, "comment"))
	} else if matches, err := profanity.ContainsProfanity(ctx, action.Content); err == nil && len(matches) > 0 {
		result.AddFieldFailure("content", i18n.T(ctx, "validation.custom.containsprofanity"))
	}

	if len(action.Attachments) > 0 {
		getAttachments := &query.GetAttachments{Post: action.Post, Comment: action.Comment}
		err := bus.Dispatch(ctx, getAttachments)
		if err != nil {
			return validate.Error(err)
		}

		messages, err := validate.MultiImageUpload(ctx, getAttachments.Result, action.Attachments, validate.MultiImageUploadOpts{
			MaxUploads:   generalSettings.MaxImagesPerComment,
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

// DeleteComment represents the action of deleting an existing comment
type DeleteComment struct {
	PostNumber int `route:"number"`
	CommentID  int `route:"id"`
}

// IsAuthorized returns true if current user is authorized to perform this action
func (action *DeleteComment) IsAuthorized(ctx context.Context, user *entity.User) bool {
	commentByID := &query.GetCommentByID{CommentID: action.CommentID}
	if err := bus.Dispatch(ctx, commentByID); err != nil {
		return false
	}

	// If user is collaborator or admin, they can delete any comment
	if user.IsCollaborator() || user.IsAdministrator() {
		return true
	}

	// If user is moderator, they can only delete comments from regular users
	if user.IsModerator() {
		return commentByID.Result.User.Role == enum.RoleVisitor
	}

	// Regular users can only delete their own comments
	return user.ID == commentByID.Result.User.ID
}

// Validate if current model is valid
func (action *DeleteComment) Validate(ctx context.Context, user *entity.User) *validate.Result {
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
	return user != nil && (user.IsAdministrator() || user.IsCollaborator())
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
	return user != nil && (user.IsAdministrator() || user.IsCollaborator() || user.IsModerator())
}

func (action *UnlockPost) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	if action.Post == nil {
		result.AddFieldFailure("number", i18n.T(ctx, "validation.custom.invalidpost"))
	}

	return result
}
