package actions

import (
	"context"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/i18n"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/profanity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

// UpdateUserSettings happens when users updates their settings
type UpdateUserSettings struct {
	Settings map[string]string `json:"settings"`
}

func NewUpdateUserSettings() *UpdateUserSettings {
	return &UpdateUserSettings{}
}

// IsAuthorized returns true if current user is authorized to perform this action
func (action *UpdateUserSettings) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil
}

// Validate if current model is valid
func (action *UpdateUserSettings) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	if action.Settings != nil {
		if len(action.Settings) > 100 {
			result.AddFieldFailure("settings", "Too many settings provided")
			return result
		}

		for k, v := range action.Settings {
			if len(k) > 100 || len(v) > 100 {
				result.AddFieldFailure("settings", "Setting key or value too long")
				continue
			}

			ok := false
			for _, e := range enum.AllNotificationEvents {
				if e.UserSettingsKeyName == k {
					ok = true
					if !e.Validate(v) {
						result.AddFieldFailure("settings", i18n.T(ctx, "validation.invalidvalue", i18n.Params{"name": k}, i18n.Params{"value": v}))
					}
					break
				}
			}
			if !ok {
				result.AddFieldFailure("settings", i18n.T(ctx, "validation.custom.unknownsettings", i18n.Params{"name": k}))
			}
		}
	}

	return result
}

// UpdateUserName happens when users updates their name
type UpdateUserName struct {
	Name string `json:"name"`
}

func NewUpdateUserName() *UpdateUserName {
	return &UpdateUserName{}
}

// IsAuthorized returns true if current user is authorized to perform this action
func (action *UpdateUserName) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil
}

// Validate if current model is valid
func (action *UpdateUserName) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	if action.Name == "" {
		result.AddFieldFailure("name", propertyIsRequired(ctx, "name"))
	} else if matches, err := profanity.ContainsProfanity(ctx, action.Name); err == nil && len(matches) > 0 {
		result.AddFieldFailure("content", i18n.T(ctx, "validation.custom.containsprofanity"))
	}

	if len(action.Name) > 50 {
		result.AddFieldFailure("name", propertyMaxStringLen(ctx, "name", 50))
	}

	return result
}

// UpdateUserAvatar happens when users updates their avatar
type UpdateUserAvatar struct {
	AvatarType enum.AvatarType  `json:"avatarType"`
	Avatar     *dto.ImageUpload `json:"avatar"`
}

func NewUpdateUserAvatar() *UpdateUserAvatar {
	return &UpdateUserAvatar{
		Avatar: &dto.ImageUpload{},
	}
}

// IsAuthorized returns true if current user is authorized to perform this action
func (action *UpdateUserAvatar) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil
}

// Validate if current model is valid
func (action *UpdateUserAvatar) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	if action.AvatarType == 0 {
		action.AvatarType = user.AvatarType
	} else if action.AvatarType < 1 || action.AvatarType > 3 {
		result.AddFieldFailure("avatarType", propertyIsInvalid(ctx, "avatarType"))
	}

	if action.Avatar == nil {
		action.Avatar = &dto.ImageUpload{}
	}
	action.Avatar.BlobKey = user.AvatarBlobKey

	if action.AvatarType == enum.AvatarTypeCustom && action.Avatar.Upload != nil {
		messages, err := validate.ImageUpload(ctx, action.Avatar, validate.ImageUploadOpts{
			IsRequired:   true,
			MinHeight:    50,
			MinWidth:     50,
			ExactRatio:   true,
			MaxKilobytes: 5000,
		})
		if err != nil {
			return validate.Error(err)
		}
		result.AddFieldFailure("avatar", messages...)
	}

	return result
}
