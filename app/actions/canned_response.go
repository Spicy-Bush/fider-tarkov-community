package actions

import (
	"context"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

// CreateCannedResponse is the input model used to create a new canned response
type CreateCannedResponse struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Content  string `json:"content"`
	Duration string `json:"duration"`
}

// IsAuthorized returns true if current user is authorized to perform this action
func (action *CreateCannedResponse) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil && (user.IsAdministrator() || user.IsCollaborator() || user.IsModerator())
}

// Validate if current model is valid
func (action *CreateCannedResponse) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	if action.Type == "" {
		result.AddFieldFailure("type", "Type is required")
	}

	if action.Title == "" {
		result.AddFieldFailure("title", "Title is required")
	}

	if action.Content == "" {
		result.AddFieldFailure("content", "Content is required")
	}

	// Duration is optional for some types but required for others
	if action.Type == "mute" && action.Duration == "" {
		result.AddFieldFailure("duration", "Duration is required for mute responses")
	}

	return result
}

// UpdateCannedResponse is the input model used to update an existing canned response
type UpdateCannedResponse struct {
	ID       int    `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	Content  string `json:"content"`
	Duration string `json:"duration"`
	IsActive bool   `json:"isActive"`
}

// IsAuthorized returns true if current user is authorized to perform this action
func (action *UpdateCannedResponse) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil && (user.IsAdministrator() || user.IsCollaborator() || user.IsModerator())
}

// Validate if current model is valid
func (action *UpdateCannedResponse) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	if action.ID <= 0 {
		result.AddFieldFailure("id", "Invalid ID")
	}

	if action.Type == "" {
		result.AddFieldFailure("type", "Type is required")
	}

	if action.Title == "" {
		result.AddFieldFailure("title", "Title is required")
	}

	if action.Content == "" {
		result.AddFieldFailure("content", "Content is required")
	}

	// Duration is optional for some types but required for others
	if action.Type == "mute" && action.Duration == "" {
		result.AddFieldFailure("duration", "Duration is required for mute responses")
	}

	return result
}

// DeleteCannedResponse is the input model used to delete an existing canned response
type DeleteCannedResponse struct {
	ID int `json:"id"`
}

// IsAuthorized returns true if current user is authorized to perform this action
func (action *DeleteCannedResponse) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil && (user.IsAdministrator() || user.IsCollaborator() || user.IsModerator())
}

// Validate if current model is valid
func (action *DeleteCannedResponse) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	if action.ID <= 0 {
		result.AddFieldFailure("id", "Invalid ID")
	}

	return result
}
