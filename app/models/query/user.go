package query

import (
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
)

type CountUsers struct {
	Result int
}

type UserSubscribedTo struct {
	PostID int

	Result bool
}

type GetUserByAPIKey struct {
	APIKey string

	Result *entity.User
}

type GetCurrentUserSettings struct {
	Result map[string]string
}

type GetUserByID struct {
	UserID int

	Result *entity.User
}

type GetUserByEmail struct {
	Email string

	Result *entity.User
}

type GetUserByProvider struct {
	Provider string
	UID      string

	Result *entity.User
}

type GetAllUsers struct {
	Result []*entity.User
}

type GetAllUserProviders struct {
	Result []*entity.UserProvider
}

type GetAllUsersNames struct {
	Query  string
	Limit  int
	Result []*dto.UserNames
}
