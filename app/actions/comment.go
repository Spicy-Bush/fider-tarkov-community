package actions

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/i18n"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/profanity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

type CommentInput struct {
	Discussion  *entity.Discussion
	Comment     *entity.Comment
	Content     string
	Attachments []*dto.ImageUpload
}

func (input *CommentInput) Validate(ctx context.Context, user *entity.User) *validate.Result {
	tenant := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	permissions := input.Discussion.Permissions(user, tenant)
	canWrite := permissions.Comment
	if input.Comment != nil {
		canWrite = input.Comment.AllowedActions(user, input.Discussion, tenant, time.Now()).Edit
	}

	if !canWrite {
		return validate.Unauthorized()
	}

	result := validate.Success()
	if strings.TrimSpace(input.Content) == "" {
		result.AddFieldFailure("content", propertyIsRequired(ctx, "comment"))
	} else if matches, err := profanity.ContainsProfanity(ctx, input.Content); err == nil && len(matches) > 0 {
		result.AddFieldFailure("content", i18n.T(ctx, "validation.custom.containsprofanity"))
	}

	if input.Discussion.Owner.Kind == "page" && utf8.RuneCountInString(input.Content) > 5000 {
		result.AddFieldFailure("content", "Comments must contain at most 5000 characters.")
	}

	settings := tenant.GeneralSettings
	if input.Comment == nil && settings != nil && !entity.CanBypassPostingRateLimits(user, tenant) {
		if limit, ok := settings.CommentLimits[user.Role.String()]; ok && limit.Count > 0 {
			count := &query.GetUserCommentCount{
				UserID: user.ID,
				Since:  time.Now().Add(-time.Duration(limit.Hours) * time.Hour),
			}

			if err := bus.Dispatch(ctx, count); err != nil {
				return validate.Error(err)
			}

			if count.Result >= limit.Count {
				result.AddFieldFailure("content", i18n.T(ctx, "validation.custom.toomanycomments"))
			}
		}
	}

	var existing []string
	if input.Comment != nil {
		existing = input.Comment.Attachments
	}

	for _, attachment := range input.Attachments {
		if attachment == nil {
			result.AddFieldFailure("attachments", "Invalid attachment.")
			return result
		}

		if attachment.Upload != nil {
			if attachment.Remove || attachment.BlobKey != "" {
				result.AddFieldFailure("attachments", "An upload cannot also replace or remove a stored image.")
				return result
			}

			if !permissions.Images {
				result.AddFieldFailure("attachments", "Image uploads are disabled for this Page.")
			}

			if len(attachment.Upload.Content) == 0 {
				result.AddFieldFailure("attachments", "The image is empty.")
			}
			continue
		}

		owned := false
		for _, key := range existing {
			if key == attachment.BlobKey {
				owned = true
				break
			}
		}

		if !owned && (attachment.Remove || !permissions.Images) {
			result.AddFieldFailure("attachments", "The attachment is unavailable or cannot be added to this comment.")
		}
	}

	maxUploads := 2
	if settings != nil {
		maxUploads = settings.MaxImagesPerComment
	}

	messages, err := validate.MultiImageUpload(ctx, existing, input.Attachments, validate.MultiImageUploadOpts{
		MaxUploads:   maxUploads,
		MaxKilobytes: 7500,
	})
	if err != nil {
		return validate.Error(err)
	}

	result.AddFieldFailure("attachments", messages...)
	return result
}
