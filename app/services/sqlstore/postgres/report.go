package postgres

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/lib/pq"
)

type dbReport struct {
	ID                   int            `db:"id"`
	ReportedType         string         `db:"reported_type"`
	ReportedID           int            `db:"reported_id"`
	Reason               string         `db:"reason"`
	Details              sql.NullString `db:"details"`
	Status               string         `db:"status"`
	CreatedAt            time.Time      `db:"created_at"`
	ReporterID           int            `db:"reporter_id"`
	ReporterName         string         `db:"reporter_name"`
	ReporterAvatarType   sql.NullInt64  `db:"reporter_avatar_type"`
	ReporterAvatarBkey   sql.NullString `db:"reporter_avatar_bkey"`
	AssignedToID         sql.NullInt64  `db:"assigned_to_id"`
	AssignedToName       sql.NullString `db:"assigned_to_name"`
	AssignedToAvatarType sql.NullInt64  `db:"assigned_to_avatar_type"`
	AssignedToAvatarBkey sql.NullString `db:"assigned_to_avatar_bkey"`
	AssignedAt           sql.NullTime   `db:"assigned_at"`
	ResolvedAt           sql.NullTime   `db:"resolved_at"`
	ResolvedByID         sql.NullInt64  `db:"resolved_by_id"`
	ResolvedByName       sql.NullString `db:"resolved_by_name"`
	ResolutionNote       sql.NullString `db:"resolution_note"`
	PostNumber           sql.NullInt64  `db:"post_number"`
	PostSlug             sql.NullString `db:"post_slug"`
}

func (r *dbReport) toModel(ctx context.Context) *entity.Report {
	report := &entity.Report{
		ID:        r.ID,
		Reason:    r.Reason,
		Status:    enum.ReportStatusPending,
		CreatedAt: r.CreatedAt,
		Reporter: &entity.User{
			ID:   r.ReporterID,
			Name: r.ReporterName,
		},
	}

	_ = report.ReportedType.UnmarshalText([]byte(r.ReportedType))
	report.ReportedID = r.ReportedID
	_ = report.Status.UnmarshalText([]byte(r.Status))

	if r.Details.Valid {
		report.Details = r.Details.String
	}

	if r.ReporterAvatarType.Valid {
		report.Reporter.AvatarURL = buildAvatarURL(ctx, enum.AvatarType(r.ReporterAvatarType.Int64), r.ReporterID, r.ReporterName, r.ReporterAvatarBkey.String)
	}

	if r.AssignedToID.Valid {
		report.AssignedTo = &entity.User{
			ID:   int(r.AssignedToID.Int64),
			Name: r.AssignedToName.String,
		}
		if r.AssignedToAvatarType.Valid {
			report.AssignedTo.AvatarURL = buildAvatarURL(ctx, enum.AvatarType(r.AssignedToAvatarType.Int64), int(r.AssignedToID.Int64), r.AssignedToName.String, r.AssignedToAvatarBkey.String)
		}
		if r.AssignedAt.Valid {
			report.AssignedAt = &r.AssignedAt.Time
		}
	}

	if r.ResolvedAt.Valid {
		report.ResolvedAt = &r.ResolvedAt.Time
	}

	if r.ResolvedByID.Valid {
		report.ResolvedBy = &entity.User{
			ID:   int(r.ResolvedByID.Int64),
			Name: r.ResolvedByName.String,
		}
	}

	if r.ResolutionNote.Valid {
		report.ResolutionNote = r.ResolutionNote.String
	}

	if r.PostNumber.Valid {
		report.PostNumber = int(r.PostNumber.Int64)
	}
	if r.PostSlug.Valid {
		report.PostSlug = r.PostSlug.String
	}

	return report
}

type dbReportReason struct {
	ID          int            `db:"id"`
	Slug        string         `db:"slug"`
	Title       string         `db:"title"`
	Description sql.NullString `db:"description"`
	SortOrder   int            `db:"sort_order"`
	IsActive    bool           `db:"is_active"`
}

func (r *dbReportReason) toModel() *entity.ReportReason {
	reason := &entity.ReportReason{
		ID:        r.ID,
		Slug:      r.Slug,
		Title:     r.Title,
		SortOrder: r.SortOrder,
		IsActive:  r.IsActive,
	}
	if r.Description.Valid {
		reason.Description = r.Description.String
	}
	return reason
}

func createReport(ctx context.Context, c *cmd.CreateReport) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var id int
		err := trx.Scalar(&id, `
			INSERT INTO reports (tenant_id, reporter_id, reported_type, reported_id, reason, details, status, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, 'pending', NOW())
			RETURNING id
		`, tenant.ID, user.ID, c.ReportedType.String(), c.ReportedID, c.Reason, nullIfEmpty(c.Details))
		if err != nil {
			return errors.Wrap(err, "failed to create report")
		}
		c.Result = id
		return nil
	})
}

func assignReport(ctx context.Context, c *cmd.AssignReport) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute(`
			UPDATE reports 
			SET assigned_to = $1, assigned_at = NOW(), status = 'in_review'
			WHERE id = $2 AND tenant_id = $3
		`, c.AssignToID, c.ReportID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to assign report")
		}
		return nil
	})
}

func unassignReport(ctx context.Context, c *cmd.UnassignReport) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute(`
			UPDATE reports 
			SET assigned_to = NULL, assigned_at = NULL, status = 'pending'
			WHERE id = $1 AND tenant_id = $2
		`, c.ReportID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to unassign report")
		}
		return nil
	})
}

func resolveReport(ctx context.Context, c *cmd.ResolveReport) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute(`
			UPDATE reports 
			SET status = $1, resolved_at = NOW(), resolved_by = $2, resolution_note = $3
			WHERE id = $4 AND tenant_id = $5
		`, c.Status.String(), user.ID, nullIfEmpty(c.ResolutionNote), c.ReportID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to resolve report")
		}
		return nil
	})
}

func deleteReport(ctx context.Context, c *cmd.DeleteReport) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute(`
			DELETE FROM reports WHERE id = $1 AND tenant_id = $2
		`, c.ReportID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to delete report")
		}
		return nil
	})
}

func getReportByID(ctx context.Context, q *query.GetReportByID) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		report := dbReport{}
		err := trx.Get(&report, `
			SELECT 
				r.id, r.reported_type, r.reported_id, r.reason, r.details, r.status, r.created_at,
				r.reporter_id, ru.name as reporter_name, ru.avatar_type as reporter_avatar_type, ru.avatar_bkey as reporter_avatar_bkey,
				r.assigned_to as assigned_to_id, au.name as assigned_to_name, au.avatar_type as assigned_to_avatar_type, au.avatar_bkey as assigned_to_avatar_bkey, r.assigned_at,
				r.resolved_at, r.resolved_by as resolved_by_id, rbu.name as resolved_by_name,
				r.resolution_note,
				COALESCE(p.number, cp.number) as post_number,
				COALESCE(p.slug, cp.slug) as post_slug
			FROM reports r
			LEFT JOIN users ru ON ru.id = r.reporter_id
			LEFT JOIN users au ON au.id = r.assigned_to
			LEFT JOIN users rbu ON rbu.id = r.resolved_by
			LEFT JOIN posts p ON r.reported_type = 'post' AND p.id = r.reported_id
			LEFT JOIN comments c ON r.reported_type = 'comment' AND c.id = r.reported_id
			LEFT JOIN posts cp ON c.post_id = cp.id
			WHERE r.id = $1 AND r.tenant_id = $2
		`, q.ReportID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to get report by ID")
		}
		q.Result = report.toModel(ctx)
		return nil
	})
}

func listReports(ctx context.Context, q *query.ListReports) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if q.Page < 1 {
			q.Page = 1
		}
		if q.PerPage < 1 {
			q.PerPage = 20
		}
		offset := (q.Page - 1) * q.PerPage

		conditions := "r.tenant_id = $1"
		args := []interface{}{tenant.ID}
		argIdx := 2

		if len(q.Status) > 0 {
			statusStrings := make([]string, len(q.Status))
			for i, s := range q.Status {
				statusStrings[i] = s.String()
			}
			conditions += " AND r.status = ANY($" + strconv.Itoa(argIdx) + ")"
			args = append(args, pq.Array(statusStrings))
			argIdx++
		}

		if q.Type != 0 {
			conditions += " AND r.reported_type = $" + strconv.Itoa(argIdx)
			args = append(args, q.Type.String())
			argIdx++
		}

		if q.Reason != "" {
			conditions += " AND r.reason = $" + strconv.Itoa(argIdx)
			args = append(args, q.Reason)
			argIdx++
		}

		err := trx.Scalar(&q.Total, "SELECT COUNT(*) FROM reports r WHERE "+conditions, args...)
		if err != nil {
			return errors.Wrap(err, "failed to count reports")
		}

		var reports []*dbReport
		err = trx.Select(&reports, `
			SELECT 
				r.id, r.reported_type, r.reported_id, r.reason, r.details, r.status, r.created_at,
				r.reporter_id, ru.name as reporter_name, ru.avatar_type as reporter_avatar_type, ru.avatar_bkey as reporter_avatar_bkey,
				r.assigned_to as assigned_to_id, au.name as assigned_to_name, au.avatar_type as assigned_to_avatar_type, au.avatar_bkey as assigned_to_avatar_bkey, r.assigned_at,
				r.resolved_at, r.resolved_by as resolved_by_id, rbu.name as resolved_by_name,
				r.resolution_note,
				COALESCE(p.number, cp.number) as post_number,
				COALESCE(p.slug, cp.slug) as post_slug
			FROM reports r
			LEFT JOIN users ru ON ru.id = r.reporter_id
			LEFT JOIN users au ON au.id = r.assigned_to
			LEFT JOIN users rbu ON rbu.id = r.resolved_by
			LEFT JOIN posts p ON r.reported_type = 'post' AND p.id = r.reported_id
			LEFT JOIN comments c ON r.reported_type = 'comment' AND c.id = r.reported_id
			LEFT JOIN posts cp ON c.post_id = cp.id
			WHERE `+conditions+`
			ORDER BY r.created_at DESC
			LIMIT $`+strconv.Itoa(argIdx)+` OFFSET $`+strconv.Itoa(argIdx+1),
			append(args, q.PerPage, offset)...)
		if err != nil {
			return errors.Wrap(err, "failed to list reports")
		}

		q.Result = make([]*entity.Report, len(reports))
		for i, r := range reports {
			q.Result[i] = r.toModel(ctx)
		}
		return nil
	})
}

func countPendingReports(ctx context.Context, q *query.CountPendingReports) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		err := trx.Scalar(&q.Result, `
			SELECT COUNT(*) FROM reports 
			WHERE tenant_id = $1 AND status IN ('pending', 'in_review')
		`, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to count pending reports")
		}
		return nil
	})
}

func getReportReasons(ctx context.Context, q *query.GetReportReasons) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var reasons []*dbReportReason
		err := trx.Select(&reasons, `
			SELECT id, slug, title, description, sort_order, is_active
			FROM report_reasons
			WHERE tenant_id = $1 AND is_active = TRUE
			ORDER BY sort_order ASC
		`, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to get report reasons")
		}

		q.Result = make([]*entity.ReportReason, len(reasons))
		for i, r := range reasons {
			q.Result[i] = r.toModel()
		}
		return nil
	})
}

func countUserReportsToday(ctx context.Context, q *query.CountUserReportsToday) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		err := trx.Scalar(&q.Result, `
			SELECT COUNT(*) FROM reports 
			WHERE tenant_id = $1 AND reporter_id = $2 AND created_at >= CURRENT_DATE
		`, tenant.ID, q.UserID)
		if err != nil {
			return errors.Wrap(err, "failed to count user reports today")
		}
		return nil
	})
}

func hasUserReportedTarget(ctx context.Context, q *query.HasUserReportedTarget) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var count int
		err := trx.Scalar(&count, `
			SELECT COUNT(*) FROM reports 
			WHERE tenant_id = $1 AND reporter_id = $2 AND reported_type = $3 AND reported_id = $4 AND status = 'pending'
		`, tenant.ID, q.UserID, q.ReportedType.String(), q.ReportedID)
		if err != nil {
			return errors.Wrap(err, "failed to check if user reported target")
		}
		q.Result = count > 0
		return nil
	})
}

type dbReportedCommentID struct {
	ReportedID int `db:"reported_id"`
}

func getUserReportedItemsOnPost(ctx context.Context, q *query.GetUserReportedItemsOnPost) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var postCount int
		err := trx.Scalar(&postCount, `
			SELECT COUNT(*) FROM reports 
			WHERE tenant_id = $1 AND reporter_id = $2 AND reported_type = 'post' AND reported_id = $3 AND status = 'pending'
		`, tenant.ID, user.ID, q.PostID)
		if err != nil {
			return errors.Wrap(err, "failed to check if user reported post")
		}
		q.HasReportedPost = postCount > 0

		if len(q.CommentIDs) > 0 {
			var reportedRows []*dbReportedCommentID
			err = trx.Select(&reportedRows, `
				SELECT reported_id FROM reports 
				WHERE tenant_id = $1 AND reporter_id = $2 AND reported_type = 'comment' AND reported_id = ANY($3) AND status = 'pending'
			`, tenant.ID, user.ID, pq.Array(q.CommentIDs))
			if err != nil {
				return errors.Wrap(err, "failed to get user reported comments")
			}
			q.ReportedCommentIDs = make([]int, len(reportedRows))
			for i, row := range reportedRows {
				q.ReportedCommentIDs[i] = row.ReportedID
			}
		} else {
			q.ReportedCommentIDs = []int{}
		}

		return nil
	})
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func listAllReportReasons(ctx context.Context, q *query.ListAllReportReasons) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var reasons []*dbReportReason
		err := trx.Select(&reasons, `
			SELECT id, slug, title, description, sort_order, is_active
			FROM report_reasons
			WHERE tenant_id = $1
			ORDER BY sort_order ASC
		`, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to list all report reasons")
		}

		q.Result = make([]*entity.ReportReason, len(reasons))
		for i, r := range reasons {
			q.Result[i] = r.toModel()
		}
		return nil
	})
}

func createReportReason(ctx context.Context, c *cmd.CreateReportReason) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var maxSortOrder int
		err := trx.Scalar(&maxSortOrder, `
			SELECT COALESCE(MAX(sort_order), 0) FROM report_reasons WHERE tenant_id = $1
		`, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to get max sort order")
		}

		slug := generateSlug(c.Title)

		var id int
		err = trx.Get(&id, `
			INSERT INTO report_reasons (tenant_id, slug, title, description, sort_order, is_active)
			VALUES ($1, $2, $3, $4, $5, true)
			RETURNING id
		`, tenant.ID, slug, c.Title, nullIfEmpty(c.Description), maxSortOrder+1)
		if err != nil {
			return errors.Wrap(err, "failed to create report reason")
		}

		c.Result = id
		return nil
	})
}

func updateReportReason(ctx context.Context, c *cmd.UpdateReportReason) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute(`
			UPDATE report_reasons 
			SET title = $1, description = $2, is_active = $3
			WHERE id = $4 AND tenant_id = $5
		`, c.Title, nullIfEmpty(c.Description), c.IsActive, c.ID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to update report reason")
		}
		return nil
	})
}

func deleteReportReason(ctx context.Context, c *cmd.DeleteReportReason) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute(`
			DELETE FROM report_reasons WHERE id = $1 AND tenant_id = $2
		`, c.ID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to delete report reason")
		}
		return nil
	})
}

func generateSlug(title string) string {
	slug := ""
	for _, c := range title {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			slug += string(c)
		} else if c >= 'A' && c <= 'Z' {
			slug += string(c + 32)
		} else if c == ' ' || c == '-' || c == '_' {
			if len(slug) > 0 && slug[len(slug)-1] != '-' {
				slug += "-"
			}
		}
	}
	if len(slug) > 50 {
		slug = slug[:50]
	}
	return slug
}

func reorderReportReasons(ctx context.Context, c *cmd.ReorderReportReasons) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		for i, id := range c.IDs {
			_, err := trx.Execute(`
				UPDATE report_reasons 
				SET sort_order = $1
				WHERE id = $2 AND tenant_id = $3
			`, i+1, id, tenant.ID)
			if err != nil {
				return errors.Wrap(err, "failed to reorder report reason")
			}
		}
		return nil
	})
}
