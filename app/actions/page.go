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
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/i18n"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

type CreateUpdatePage struct {
	PageID          int
	Title           string
	Slug            string
	Content         string
	Excerpt         string
	BannerImage     *dto.ImageUpload
	Status          string
	Visibility      string
	AllowedRoles    []string
	ParentPageID    *int
	AllowComments   bool
	AllowReactions  bool
	ShowTOC         bool
	ScheduledFor    *time.Time
	Authors         []int
	Topics          []int
	Tags            []int
	MetaDescription string
	CanonicalURL    string
}

func (input *CreateUpdatePage) Initialize() interface{} {
	input.AllowComments = false
	input.AllowReactions = true
	input.ShowTOC = false
	return input
}

func (action *CreateUpdatePage) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil && (user.IsAdministrator() || user.IsCollaborator())
}

func (action *CreateUpdatePage) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	if len(strings.TrimSpace(action.Title)) == 0 {
		result.AddFieldFailure("title", i18n.T(ctx, "validation.required"))
	} else if len(action.Title) < 1 {
		result.AddFieldFailure("title", i18n.T(ctx, "validation.minlength", i18n.Params{"min": 1}))
	} else if len(action.Title) > 200 {
		result.AddFieldFailure("title", i18n.T(ctx, "validation.maxlength", i18n.Params{"max": 200}))
	}

	if len(action.Slug) > 200 {
		result.AddFieldFailure("slug", i18n.T(ctx, "validation.maxlength", i18n.Params{"max": 200}))
	}

	if len(strings.TrimSpace(action.Content)) == 0 {
		result.AddFieldFailure("content", i18n.T(ctx, "validation.required"))
	} else if len(action.Content) < 1 {
		result.AddFieldFailure("content", i18n.T(ctx, "validation.minlength", i18n.Params{"min": 1}))
	} else if len(action.Content) > 50000 {
		result.AddFieldFailure("content", i18n.T(ctx, "validation.maxlength", i18n.Params{"max": 50000}))
	}

	if len(action.Excerpt) > 500 {
		result.AddFieldFailure("excerpt", i18n.T(ctx, "validation.maxlength", i18n.Params{"max": 500}))
	}

	if len(action.MetaDescription) > 300 {
		result.AddFieldFailure("metaDescription", i18n.T(ctx, "validation.maxlength", i18n.Params{"max": 300}))
	}

	status := entity.PageStatus(action.Status)
	if status != entity.PageStatusDraft && status != entity.PageStatusPublished &&
		status != entity.PageStatusUnpublished && status != entity.PageStatusScheduled {
		result.AddFieldFailure("status", "Invalid status")
	}

	visibility := entity.PageVisibility(action.Visibility)
	if visibility != entity.PageVisibilityPublic && visibility != entity.PageVisibilityPrivate &&
		visibility != entity.PageVisibilityUnlisted {
		result.AddFieldFailure("visibility", "Invalid visibility")
	}

	if status == entity.PageStatusScheduled {
		if action.ScheduledFor == nil {
			result.AddFieldFailure("scheduledFor", "Scheduled date is required for scheduled status")
		} else if action.ScheduledFor.Before(time.Now()) {
			result.AddFieldFailure("scheduledFor", "Scheduled date must be in the future")
		}
	}

	if visibility == entity.PageVisibilityPrivate {
		if len(action.AllowedRoles) == 0 {
			result.AddFieldFailure("allowedRoles", "At least one role must be selected for private pages")
		}
	}

	if action.BannerImage != nil {
		messages, err := validate.ImageUpload(ctx, action.BannerImage, validate.ImageUploadOpts{
			MaxKilobytes: 5000,
		})
		if err != nil {
			return validate.Error(err)
		}
		result.AddFieldFailure("bannerImage", messages...)
	}

	if len(action.Authors) > 0 {
		getUsersByIDs := &query.GetUsersByIDs{UserIDs: action.Authors}
		if err := bus.Dispatch(ctx, getUsersByIDs); err != nil {
			return validate.Error(err)
		}

		foundIDs := make(map[int]bool)
		for _, u := range getUsersByIDs.Result {
			if u.Role != enum.RoleAdministrator && u.Role != enum.RoleCollaborator {
				result.AddFieldFailure("authors", "Authors must be administrators or collaborators")
				break
			}
			foundIDs[u.ID] = true
		}

		if len(foundIDs) != len(action.Authors) {
			result.AddFieldFailure("authors", "One or more authors do not exist")
		}
	}

	return result
}

type DeletePage struct {
	PageID int
}

func (action *DeletePage) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil && (user.IsAdministrator() || user.IsCollaborator())
}

func (action *DeletePage) Validate(ctx context.Context, user *entity.User) *validate.Result {
	return validate.Success()
}

type CreatePageTopic struct {
	Name        string
	Slug        string
	Description string
	Color       string
}

func (action *CreatePageTopic) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil && user.IsAdministrator()
}

func (action *CreatePageTopic) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	if len(strings.TrimSpace(action.Name)) == 0 {
		result.AddFieldFailure("name", i18n.T(ctx, "validation.required"))
	} else if len(action.Name) > 100 {
		result.AddFieldFailure("name", i18n.T(ctx, "validation.maxlength", i18n.Params{"max": 100}))
	}

	if len(action.Description) > 500 {
		result.AddFieldFailure("description", i18n.T(ctx, "validation.maxlength", i18n.Params{"max": 500}))
	}

	if action.Color != "" && len(action.Color) != 7 {
		result.AddFieldFailure("color", "Color must be a valid hex code (#RRGGBB)")
	}

	return result
}

type UpdatePageTopic struct {
	ID          int
	Name        string
	Slug        string
	Description string
	Color       string
}

func (action *UpdatePageTopic) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil && user.IsAdministrator()
}

func (action *UpdatePageTopic) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	if len(strings.TrimSpace(action.Name)) == 0 {
		result.AddFieldFailure("name", i18n.T(ctx, "validation.required"))
	} else if len(action.Name) > 100 {
		result.AddFieldFailure("name", i18n.T(ctx, "validation.maxlength", i18n.Params{"max": 100}))
	}

	if len(action.Description) > 500 {
		result.AddFieldFailure("description", i18n.T(ctx, "validation.maxlength", i18n.Params{"max": 500}))
	}

	if action.Color != "" && len(action.Color) != 7 {
		result.AddFieldFailure("color", "Color must be a valid hex code (#RRGGBB)")
	}

	return result
}

type DeletePageTopic struct {
	ID int
}

func (action *DeletePageTopic) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil && user.IsAdministrator()
}

func (action *DeletePageTopic) Validate(ctx context.Context, user *entity.User) *validate.Result {
	return validate.Success()
}

type CreatePageTag struct {
	Name string
	Slug string
}

func (action *CreatePageTag) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil && user.IsAdministrator()
}

func (action *CreatePageTag) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	if len(strings.TrimSpace(action.Name)) == 0 {
		result.AddFieldFailure("name", i18n.T(ctx, "validation.required"))
	} else if len(action.Name) > 100 {
		result.AddFieldFailure("name", i18n.T(ctx, "validation.maxlength", i18n.Params{"max": 100}))
	}

	return result
}

type UpdatePageTag struct {
	ID   int
	Name string
	Slug string
}

func (action *UpdatePageTag) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil && user.IsAdministrator()
}

func (action *UpdatePageTag) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	if len(strings.TrimSpace(action.Name)) == 0 {
		result.AddFieldFailure("name", i18n.T(ctx, "validation.required"))
	} else if len(action.Name) > 100 {
		result.AddFieldFailure("name", i18n.T(ctx, "validation.maxlength", i18n.Params{"max": 100}))
	}

	return result
}

type DeletePageTag struct {
	ID int
}

func (action *DeletePageTag) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil && user.IsAdministrator()
}

func (action *DeletePageTag) Validate(ctx context.Context, user *entity.User) *validate.Result {
	return validate.Success()
}

type TogglePageReaction struct {
	PageID int
	Emoji  string
}

func (action *TogglePageReaction) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil
}

func (action *TogglePageReaction) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	if len(action.Emoji) == 0 || len(action.Emoji) > 8 {
		result.AddFieldFailure("emoji", "Invalid emoji")
	}

	return result
}

type TogglePageSubscription struct {
	PageID int
}

func (action *TogglePageSubscription) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil
}

func (action *TogglePageSubscription) Validate(ctx context.Context, user *entity.User) *validate.Result {
	return validate.Success()
}

type AddPageComment struct {
	PageID  int
	Content string
}

func (action *AddPageComment) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil
}

func (action *AddPageComment) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	if len(strings.TrimSpace(action.Content)) == 0 {
		result.AddFieldFailure("content", i18n.T(ctx, "validation.required"))
	} else if len(action.Content) > 5000 {
		result.AddFieldFailure("content", i18n.T(ctx, "validation.maxlength", i18n.Params{"max": 5000}))
	}

	return result
}

type SavePageDraft struct {
	PageID          int
	Title           string
	Slug            string
	Content         string
	Excerpt         string
	BannerImageBKey string
	MetaDescription string
	ShowTOC         bool
	DraftData       map[string]interface{}
}

func (action *SavePageDraft) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil && (user.IsAdministrator() || user.IsCollaborator())
}

func (action *SavePageDraft) Validate(ctx context.Context, user *entity.User) *validate.Result {
	return validate.Success()
}
