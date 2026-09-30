package actions

import (
	"context"
	"strings"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

type CreateSponsorshipPackage struct {
	SubmissionID string `json:"submissionId"`
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Slots        string `json:"slots"`
	DurationDays int    `json:"durationDays"`
	Sort         int    `json:"sort"`
}

func (a *CreateSponsorshipPackage) IsAuthorized(ctx context.Context, user *entity.User) bool {
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	return entity.Can(user, tenant, entity.ManageSponsorship)
}

func (a *CreateSponsorshipPackage) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()
	if !validate.ValidSubmissionID(a.SubmissionID) {
		result.AddFieldFailure("submissionId", "A save identity is required.")
	}

	if strings.TrimSpace(a.Slug) == "" {
		result.AddFieldFailure("slug", "Slug is required")
	}

	if strings.TrimSpace(a.Name) == "" {
		result.AddFieldFailure("name", "Name is required")
	}

	if a.DurationDays <= 0 {
		result.AddFieldFailure("durationDays", "Duration must be positive")
	}
	return result
}

type UpdateSponsorshipPackage struct {
	SubmissionID string `json:"submissionId"`
	ID           int    `json:"id"`
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Slots        string `json:"slots"`
	DurationDays int    `json:"durationDays"`
	Sort         int    `json:"sort"`
}

func (a *UpdateSponsorshipPackage) IsAuthorized(ctx context.Context, user *entity.User) bool {
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	return entity.Can(user, tenant, entity.ManageSponsorship)
}

func (a *UpdateSponsorshipPackage) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()
	if !validate.ValidSubmissionID(a.SubmissionID) {
		result.AddFieldFailure("submissionId", "A save identity is required.")
	}

	if a.ID <= 0 {
		result.AddFieldFailure("id", "Invalid ID")
	}
	if strings.TrimSpace(a.Slug) == "" {
		result.AddFieldFailure("slug", "Slug is required")
	}
	if strings.TrimSpace(a.Name) == "" {
		result.AddFieldFailure("name", "Name is required")
	}
	if a.DurationDays <= 0 {
		result.AddFieldFailure("durationDays", "Duration must be positive")
	}
	return result
}

type DeleteSponsorshipPackage struct {
	ID int `json:"id"`
}

func (a *DeleteSponsorshipPackage) IsAuthorized(ctx context.Context, user *entity.User) bool {
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	return entity.Can(user, tenant, entity.ManageSponsorship)
}

func (a *DeleteSponsorshipPackage) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()
	if a.ID <= 0 {
		result.AddFieldFailure("id", "Invalid ID")
	}
	return result
}
