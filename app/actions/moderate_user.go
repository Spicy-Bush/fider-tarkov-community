package actions

import (
	"context"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/utils"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

// MuteUser represents the action to mute a user
type MuteUser struct {
	UserID      int    `json:"userID"`
	Reason      string `json:"reason"`
	Duration    int    `json:"-"`
	DurationStr string `json:"duration"`
}

// IsAuthorized returns true if current user is authorized to perform this action
func (a *MuteUser) IsAuthorized(ctx context.Context, user *entity.User) bool {
	if user == nil {
		return false
	}

	getUser := &query.GetUserByID{UserID: a.UserID}
	if err := bus.Dispatch(ctx, getUser); err != nil {
		return false
	}
	targetUser := getUser.Result

	if user.ID == targetUser.ID {
		return false
	}

	if targetUser.Role == enum.RoleAdministrator {
		return false
	}

	if user.Role == enum.RoleModerator {
		return targetUser.Role == enum.RoleVisitor || targetUser.Role == enum.RoleHelper
	}

	if user.Role == enum.RoleCollaborator {
		return targetUser.Role == enum.RoleVisitor || targetUser.Role == enum.RoleModerator || targetUser.Role == enum.RoleHelper
	}

	return user.Role == enum.RoleAdministrator
}

// Validate if current action is valid
func (a *MuteUser) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	if a.Reason == "" {
		result.AddFieldFailure("reason", "Reason is required")
	}

	// Parse duration string to minutes
	if a.DurationStr == "" {
		result.AddFieldFailure("duration", "Duration is required")
		return result
	}

	minutes, ok := utils.ParseDuration(a.DurationStr)
	if !ok {
		result.AddFieldFailure("duration", "Invalid duration format. Use formats like '30m', '2h', '1d', etc.")
		return result
	}

	// Store parsed minutes in the Duration field
	a.Duration = minutes

	// Validate duration range (1 minute to 10 years)
	if a.Duration < 1 || a.Duration > 525960*10 {
		result.AddFieldFailure("duration", "Duration must be between 1 minute and 10 years")
	}

	return result
}

// WarnUser represents the action to warn a user
type WarnUser struct {
	UserID      int    `json:"userID"`
	Reason      string `json:"reason"`
	Duration    int    `json:"-"`        // Used internally after parsing DurationStr
	DurationStr string `json:"duration"` // String representation (can be "0" for permanent or "30m", "1h", "1d", etc.)
}

// IsAuthorized returns true if current user is authorized to perform this action
func (a *WarnUser) IsAuthorized(ctx context.Context, user *entity.User) bool {
	if user == nil {
		return false
	}

	getUser := &query.GetUserByID{UserID: a.UserID}
	if err := bus.Dispatch(ctx, getUser); err != nil {
		return false
	}
	targetUser := getUser.Result

	if user.ID == targetUser.ID {
		return false
	}

	if targetUser.Role == enum.RoleAdministrator {
		return false
	}

	if user.Role == enum.RoleModerator {
		return targetUser.Role == enum.RoleVisitor || targetUser.Role == enum.RoleHelper
	}

	if user.Role == enum.RoleCollaborator {
		return targetUser.Role == enum.RoleVisitor || targetUser.Role == enum.RoleModerator || targetUser.Role == enum.RoleHelper
	}

	return user.Role == enum.RoleAdministrator
}

// Validate if current action is valid
func (a *WarnUser) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	if a.Reason == "" {
		result.AddFieldFailure("reason", "Reason is required")
	}

	// Special case: "0" means permanent warning
	if a.DurationStr == "0" {
		a.Duration = 0
		return result
	}

	// Parse duration string to minutes for non-zero values
	minutes, ok := utils.ParseDuration(a.DurationStr)
	if !ok {
		result.AddFieldFailure("duration", "Invalid duration format. Use '0' for permanent or formats like '30m', '2h', '1d', etc.")
		return result
	}

	// Store parsed minutes in the Duration field
	a.Duration = minutes

	// Validate duration range (must be less than 1 year)
	if a.Duration < 0 || a.Duration > 525600*10 {
		result.AddFieldFailure("duration", "Duration must be between 0 (permanent) and 5256000 (10 years) minutes")
	}

	return result
}

// DeleteWarning represents the action to delete a warning
type DeleteWarning struct {
	UserID    int `json:"userID"`
	WarningID int `json:"warningID"`
}

// IsAuthorized returns true if current user is authorized to perform this action
func (a *DeleteWarning) IsAuthorized(ctx context.Context, user *entity.User) bool {
	if user == nil {
		return false
	}

	getUser := &query.GetUserByID{UserID: a.UserID}
	if err := bus.Dispatch(ctx, getUser); err != nil {
		return false
	}
	targetUser := getUser.Result

	if user.ID == targetUser.ID {
		return false // cannot moderate yourself
	}

	if targetUser.Role == enum.RoleAdministrator {
		return false // cannot moderate administrators
	}

	if user.Role == enum.RoleCollaborator {
		return true
	}

	return user.Role == enum.RoleAdministrator
}

// Validate if current action is valid
func (a *DeleteWarning) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	if a.UserID <= 0 {
		result.AddFieldFailure("userID", "Invalid user ID")
	}

	if a.WarningID <= 0 {
		result.AddFieldFailure("warningID", "Invalid warning ID")
	}

	return result
}

// DeleteMute represents the action to delete a mute
type DeleteMute struct {
	UserID int `json:"userID"`
	MuteID int `json:"muteID"`
}

// IsAuthorized returns true if current user is authorized to perform this action
func (a *DeleteMute) IsAuthorized(ctx context.Context, user *entity.User) bool {
	if user == nil {
		return false
	}

	getUser := &query.GetUserByID{UserID: a.UserID}
	if err := bus.Dispatch(ctx, getUser); err != nil {
		return false
	}
	targetUser := getUser.Result

	if user.ID == targetUser.ID {
		return false // cannot moderate yourself
	}

	if targetUser.Role == enum.RoleAdministrator {
		return false // cannot moderate administrators
	}

	if user.Role == enum.RoleCollaborator {
		return true
	}

	return user.Role == enum.RoleAdministrator
}

// Validate if current action is valid
func (a *DeleteMute) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	if a.UserID <= 0 {
		result.AddFieldFailure("userID", "Invalid user ID")
	}

	if a.MuteID <= 0 {
		result.AddFieldFailure("muteID", "Invalid mute ID")
	}

	return result
}
