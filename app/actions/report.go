package actions

import (
	"context"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

type CreateReport struct {
	Reason  string `json:"reason"`
	Details string `json:"details"`
}

func (a *CreateReport) Validate(ctx context.Context) *validate.Result {
	result := validate.Success()

	if a.Reason == "" {
		result.AddFieldFailure("reason", propertyIsRequired(ctx, "reason"))
		return result
	}

	if len(a.Details) > 2000 {
		result.AddFieldFailure("details", propertyMaxStringLen(ctx, "details", 2000))
		return result
	}

	reasons := &query.GetReportReasons{}
	if err := bus.Dispatch(ctx, reasons); err != nil {
		return validate.Error(err)
	}

	for _, reason := range reasons.Result {
		if reason.Title == a.Reason {
			return result
		}
	}

	result.AddFieldFailure("reason", propertyIsInvalid(ctx, "reason"))
	return result
}

type AssignReport struct {
	ReportID int `json:"reportId"`
}

func (a *AssignReport) IsAuthorized(ctx context.Context, user *entity.User) bool {
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	return entity.Can(user, tenant, entity.ManageReports)
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
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	return entity.Can(user, tenant, entity.ManageReports)
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

type CreateReportReason struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

func (a *CreateReportReason) IsAuthorized(ctx context.Context, user *entity.User) bool {
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	return entity.Can(user, tenant, entity.ManageReportReasons)
}

func (a *CreateReportReason) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	if a.Title == "" {
		result.AddFieldFailure("title", propertyIsRequired(ctx, "title"))
	} else if len(a.Title) > 100 {
		result.AddFieldFailure("title", propertyMaxStringLen(ctx, "title", 100))
	}

	if len(a.Description) > 500 {
		result.AddFieldFailure("description", propertyMaxStringLen(ctx, "description", 500))
	}

	return result
}

type UpdateReportReason struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	IsActive    bool   `json:"isActive"`
}

func (a *UpdateReportReason) IsAuthorized(ctx context.Context, user *entity.User) bool {
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	return entity.Can(user, tenant, entity.ManageReportReasons)
}

func (a *UpdateReportReason) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	if a.ID <= 0 {
		result.AddFieldFailure("id", propertyIsInvalid(ctx, "id"))
	}

	if a.Title == "" {
		result.AddFieldFailure("title", propertyIsRequired(ctx, "title"))
	} else if len(a.Title) > 100 {
		result.AddFieldFailure("title", propertyMaxStringLen(ctx, "title", 100))
	}

	if len(a.Description) > 500 {
		result.AddFieldFailure("description", propertyMaxStringLen(ctx, "description", 500))
	}

	return result
}

type DeleteReportReason struct {
	ID int `json:"id"`
}

func (a *DeleteReportReason) IsAuthorized(ctx context.Context, user *entity.User) bool {
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	return entity.Can(user, tenant, entity.ManageReportReasons)
}

func (a *DeleteReportReason) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	if a.ID <= 0 {
		result.AddFieldFailure("id", propertyIsInvalid(ctx, "id"))
	}

	return result
}

type ReorderReportReasons struct {
	IDs []int `json:"ids"`
}

func (a *ReorderReportReasons) IsAuthorized(ctx context.Context, user *entity.User) bool {
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	return entity.Can(user, tenant, entity.ManageReportReasons)
}

func (a *ReorderReportReasons) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	if len(a.IDs) == 0 {
		result.AddFieldFailure("ids", propertyIsRequired(ctx, "ids"))
	}

	return result
}
