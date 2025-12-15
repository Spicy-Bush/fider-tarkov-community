package actions_test

import (
	"context"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	. "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/assert"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
)

func TestInvalidUserNames(t *testing.T) {
	RegisterT(t)

	bus.AddHandler(func(ctx context.Context, q *query.GetTenantProfanityWords) error {
		q.Result = ""
		return nil
	})

	for _, name := range []string{
		"",
		"123456789012345678901234567890123456789012345678901", // 51 chars
	} {

		action := actions.NewUpdateUserName()
		action.Name = name
		result := action.Validate(context.Background(), &entity.User{})
		ExpectFailed(result, "name")
	}
}

func TestValidUserNames(t *testing.T) {
	RegisterT(t)

	bus.AddHandler(func(ctx context.Context, q *query.GetTenantProfanityWords) error {
		q.Result = ""
		return nil
	})

	for _, name := range []string{
		"Jon Snow",
		"Arya",
	} {
		action := actions.NewUpdateUserName()
		action.Name = name
		result := action.Validate(context.Background(), &entity.User{})
		ExpectSuccess(result)
	}
}

func TestInvalidSettings(t *testing.T) {
	RegisterT(t)

	for _, settings := range []map[string]string{
		{
			"bad_name": "3",
		},
		{
			enum.NotificationEventNewComment.UserSettingsKeyName: "8",
		},
	} {
		action := actions.NewUpdateUserSettings()
		action.Settings = settings
		result := action.Validate(context.Background(), &entity.User{})
		ExpectFailed(result, "settings")
	}
}

func TestValidSettings(t *testing.T) {
	RegisterT(t)

	for _, settings := range []map[string]string{
		nil,
		{
			enum.NotificationEventNewPost.UserSettingsKeyName:      enum.NotificationEventNewPost.DefaultSettingValue,
			enum.NotificationEventNewComment.UserSettingsKeyName:   enum.NotificationEventNewComment.DefaultSettingValue,
			enum.NotificationEventChangeStatus.UserSettingsKeyName: enum.NotificationEventChangeStatus.DefaultSettingValue,
		},
		{
			enum.NotificationEventNewComment.UserSettingsKeyName: enum.NotificationEventNewComment.DefaultSettingValue,
		},
	} {
		action := actions.NewUpdateUserSettings()
		action.Settings = settings

		result := action.Validate(context.Background(), &entity.User{
			AvatarBlobKey: "jon.png",
		})

		ExpectSuccess(result)
	}
}
