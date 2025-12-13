package entity

import (
	"encoding/json"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

// User represents an user inside our application
type User struct {
	ID            int             `json:"id"`
	Name          string          `json:"name"`
	Tenant        *Tenant         `json:"-"`
	Email         string          `json:"-"`
	Role          enum.Role       `json:"role"`
	VisualRole    enum.VisualRole `json:"visualRole"`
	Providers     []*UserProvider `json:"-"`
	AvatarBlobKey string          `json:"-"`
	AvatarType    enum.AvatarType `json:"-"`
	AvatarURL     string          `json:"avatarURL,omitempty"`
	Status        enum.UserStatus `json:"status"`
	warningCheck  func(int) bool
	muteCheck     func(int) bool
}

// Map permission role to equivalent visual role
func (u *User) GetVisualRole() enum.VisualRole {
	if u.VisualRole != enum.VisualRoleNone {
		return u.VisualRole
	}

	switch u.Role {
	case enum.RoleVisitor:
		return enum.VisualRoleVisitor
	case enum.RoleCollaborator:
		return enum.VisualRoleBSGCrew
	case enum.RoleAdministrator:
		return enum.VisualRoleAdministrator
	case enum.RoleModerator:
		return enum.VisualRoleModerator
	case enum.RoleHelper:
		return enum.VisualRoleHelper
	default:
		return enum.VisualRoleVisitor
	}
}

// HasProvider returns true if current user has registered with given provider
func (u *User) HasProvider(provider string) bool {
	for _, p := range u.Providers {
		if p.Name == provider {
			return true
		}
	}
	return false
}

// IsCollaborator returns true if user has special permissions
func (u *User) IsCollaborator() bool {
	return u.Role == enum.RoleCollaborator || u.Role == enum.RoleAdministrator
}

// IsAdministrator returns true if user is administrator
func (u *User) IsAdministrator() bool {
	return u.Role == enum.RoleAdministrator
}

// IsModerator returns true if user is moderator
func (u *User) IsModerator() bool {
	return u.Role == enum.RoleModerator
}

func (u *User) IsHelper() bool {
	return u.Role == enum.RoleHelper
}

// HasWarning returns true if user has an active warning
func (u *User) HasWarning() bool {
	if u.warningCheck != nil {
		return u.warningCheck(u.ID)
	}
	return false
}

// IsMuted returns true if user is currently muted
func (u *User) IsMuted() bool {
	if u.muteCheck != nil {
		return u.muteCheck(u.ID)
	}
	return false
}

// SetWarningCheck sets the callback function to check if a user has a warning
func (u *User) SetWarningCheck(check func(int) bool) {
	u.warningCheck = check
}

// SetMuteCheck sets the callback function to check if a user is muted
func (u *User) SetMuteCheck(check func(int) bool) {
	u.muteCheck = check
}

// UserProvider represents the relationship between an User and an Authentication provide
type UserProvider struct {
	UserID int    `json:"user_id" db:"user_id"`
	Name   string `json:"provider" db:"provider"`
	UID    string `json:"provider_uid" db:"provider_uid"`
}

// UserWithEmail is a wrapper around User that includes the email field when marshaling to JSON
type UserWithEmail struct {
	*User
}

func (umc UserWithEmail) MarshalJSON() ([]byte, error) {
	type Alias User // Prevent recursion

	providerInfo := make([]map[string]string, len(umc.User.Providers))
	for i, provider := range umc.User.Providers {
		providerInfo[i] = map[string]string{
			"name": provider.Name,
			"uid":  provider.UID,
		}
	}

	return json.Marshal(&struct {
		*Alias
		Email      string              `json:"email"`
		VisualRole enum.VisualRole     `json:"visualRole"`
		Providers  []map[string]string `json:"providers"`
	}{
		Alias:      (*Alias)(umc.User),
		Email:      umc.User.Email,
		VisualRole: umc.User.VisualRole,
		Providers:  providerInfo,
	})
}
