package actions

import (
	"context"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/i18n"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

type CreateReport struct {
	ReportedType string `json:"reportedType"`
	ReportedID   int    `json:"reportedId"`
	Reason       string `json:"reason"`
	Details      string `json:"details"`
}

func (a *CreateReport) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil && user.Status == enum.UserActive
}

func (a *CreateReport) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	if tenant != nil && tenant.GeneralSettings != nil && tenant.GeneralSettings.ReportingGloballyDisabled {
		result.AddFieldFailure("reportedId", i18n.T(ctx, "validation.custom.reportingdisabled"))
		return result
	}

	if a.ReportedType == "" {
		result.AddFieldFailure("reportedType", propertyIsRequired(ctx, "reportedType"))
		return result
	}

	var reportType enum.ReportType
	_ = reportType.UnmarshalText([]byte(a.ReportedType))
	if reportType == 0 {
		result.AddFieldFailure("reportedType", propertyIsInvalid(ctx, "reportedType"))
		return result
	}

	if a.ReportedID <= 0 {
		result.AddFieldFailure("reportedId", propertyIsInvalid(ctx, "reportedId"))
		return result
	}

	if a.Reason == "" {
		result.AddFieldFailure("reason", propertyIsRequired(ctx, "reason"))
		return result
	}

	getReasons := &query.GetReportReasons{}
	if err := bus.Dispatch(ctx, getReasons); err != nil {
		result.AddFieldFailure("reason", i18n.T(ctx, "validation.custom.unabletovalidate"))
		return result
	}

	validReason := false
	for _, r := range getReasons.Result {
		if r.Title == a.Reason {
			validReason = true
			break
		}
	}
	if !validReason {
		result.AddFieldFailure("reason", propertyIsInvalid(ctx, "reason"))
		return result
	}

	if len(a.Details) > 2000 {
		result.AddFieldFailure("details", propertyMaxStringLen(ctx, "details", 2000))
		return result
	}

	if reportType == enum.ReportTypePost {
		getPost := &query.GetPostByID{PostID: a.ReportedID}
		if err := bus.Dispatch(ctx, getPost); err != nil {
			result.AddFieldFailure("reportedId", i18n.T(ctx, "validation.custom.postnotfound"))
			return result
		}
		if getPost.Result.User.ID == user.ID {
			result.AddFieldFailure("reportedId", i18n.T(ctx, "validation.custom.cannotreportown"))
			return result
		}
	}

	if reportType == enum.ReportTypeComment {
		getComment := &query.GetCommentByID{CommentID: a.ReportedID}
		if err := bus.Dispatch(ctx, getComment); err != nil {
			result.AddFieldFailure("reportedId", i18n.T(ctx, "validation.custom.commentnotfound"))
			return result
		}
		if getComment.Result.User.ID == user.ID {
			result.AddFieldFailure("reportedId", i18n.T(ctx, "validation.custom.cannotreportown"))
			return result
		}
	}

	hasReported := &query.HasUserReportedTarget{
		UserID:       user.ID,
		ReportedType: reportType,
		ReportedID:   a.ReportedID,
	}
	if err := bus.Dispatch(ctx, hasReported); err == nil && hasReported.Result {
		result.AddFieldFailure("reportedId", i18n.T(ctx, "validation.custom.alreadyreported"))
		return result
	}

	maxReportsPerDay := 10
	if tenant != nil && tenant.GeneralSettings != nil && tenant.GeneralSettings.ReportLimitsPerDay > 0 {
		maxReportsPerDay = tenant.GeneralSettings.ReportLimitsPerDay
	}

	countToday := &query.CountUserReportsToday{UserID: user.ID}
	if err := bus.Dispatch(ctx, countToday); err == nil && countToday.Result >= maxReportsPerDay {
		result.AddFieldFailure("reportedId", i18n.T(ctx, "validation.custom.reportlimitreached"))
		return result
	}

	return result
}

type AssignReport struct {
	ReportID int `json:"reportId"`
}

func (a *AssignReport) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil && (user.Role == enum.RoleModerator || user.Role == enum.RoleCollaborator || user.Role == enum.RoleAdministrator)
}

func (a *AssignReport) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	if a.ReportID <= 0 {
		result.AddFieldFailure("reportId", propertyIsInvalid(ctx, "reportId"))
	}

	return result
}

type ResolveReport struct {
	ReportID       int    `json:"reportId"`
	Status         string `json:"status"`
	ResolutionNote string `json:"resolutionNote"`
}

func (a *ResolveReport) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil && (user.Role == enum.RoleModerator || user.Role == enum.RoleCollaborator || user.Role == enum.RoleAdministrator)
}

func (a *ResolveReport) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	if a.ReportID <= 0 {
		result.AddFieldFailure("reportId", propertyIsInvalid(ctx, "reportId"))
		return result
	}

	if a.Status != "resolved" && a.Status != "dismissed" {
		result.AddFieldFailure("status", propertyIsInvalid(ctx, "status"))
	}

	if len(a.ResolutionNote) > 2000 {
		result.AddFieldFailure("resolutionNote", propertyMaxStringLen(ctx, "resolutionNote", 2000))
	}

	return result
}
