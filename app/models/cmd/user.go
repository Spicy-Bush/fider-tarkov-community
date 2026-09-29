package cmd

import (
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

type BlockUser struct {
	UserID int
}

type UnblockUser struct {
	UserID int
}

type RegenerateAPIKey struct {
	Result string
}

type DeleteCurrentUser struct {
}

type ChangeUserRole struct {
	UserID int
	Role   enum.Role
	Result *entity.User
}

type ChangeUserEmail struct {
	UserID int
	Email  string
}

type UpdateCurrentUserSettings struct {
	Settings map[string]string
}

type RegisterUser struct {
	User *entity.User
}

type RegisterUserProvider struct {
	UserID       int
	ProviderName string
	ProviderUID  string
}

type ChangeUserVisualRole struct {
	UserID     int
	VisualRole enum.VisualRole
}
