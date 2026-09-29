package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/i18n"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
	"github.com/lib/pq"
)

type dbReport struct {
	ID             int            `db:"id"`
	ReportedType   string         `db:"reported_type"`
	ReportedID     int            `db:"reported_id"`
	Reason         string         `db:"reason"`
	Details        sql.NullString `db:"details"`
	Status         string         `db:"status"`
	CreatedAt      time.Time      `db:"created_at"`
	Reporter       *dbUser        `db:"reporter"`
	AssignedTo     *dbUser        `db:"assigned_to"`
	AssignedAt     sql.NullTime   `db:"assigned_at"`
	ResolvedAt     sql.NullTime   `db:"resolved_at"`
	ResolvedBy     *dbUser        `db:"resolved_by"`
	ResolutionNote sql.NullString `db:"resolution_note"`
	PostNumber     sql.NullInt64  `db:"post_number"`
	PostSlug       sql.NullString `db:"post_slug"`
	PageSlug       sql.NullString `db:"page_slug"`
}

func (r *dbReport) toModel(ctx context.Context) *entity.Report {
	report := &entity.Report{
		ID:         r.ID,
		Reason:     r.Reason,
		Status:     enum.ReportStatusPending,
		CreatedAt:  r.CreatedAt,
		Reporter:   r.Reporter.toModel(ctx),
		AssignedTo: r.AssignedTo.toModel(ctx),
		ResolvedBy: r.ResolvedBy.toModel(ctx),
	}

	_ = report.ReportedType.UnmarshalText([]byte(r.ReportedType))
	report.ReportedID = r.ReportedID
	_ = report.Status.UnmarshalText([]byte(r.Status))

	if r.Details.Valid {
		report.Details = r.Details.String
	}

	if report.AssignedTo != nil && r.AssignedAt.Valid {
		report.AssignedAt = &r.AssignedAt.Time
	}

	if r.ResolvedAt.Valid {
		report.ResolvedAt = &r.ResolvedAt.Time
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
	if r.PageSlug.Valid {
		report.PageSlug = r.PageSlug.String
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
	c.Created = false
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if user == nil || user.Status != enum.UserActive {
			return validate.Unauthorized()
		}

		failure := validate.Success()

		// Reports on every target consume the same daily allowance.
		identity := fmt.Sprintf("reporter:%d:%d", tenant.ID, user.ID)
		if _, err := trx.Execute("SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", identity); err != nil {
			return err
		}

		var targetFailure error
		switch c.ReportedType {
		case enum.ReportTypePost:
			lookup := &query.GetPostByNumber{Number: c.PostNumber}
			if err := getPostByNumber(ctx, lookup); err != nil {
				return err
			}
			c.ReportedID = lookup.Result.ID

			if lookup.Result.User.ID == user.ID {
				failure.AddFieldFailure("reportedId", i18n.T(ctx, "validation.custom.cannotreportown"))
				targetFailure = failure
			} else if !lookup.Result.AllowedActions(user, tenant, time.Now()).Report {
				targetFailure = validate.Unauthorized()
			}

		case enum.ReportTypeComment:
			owner := &query.GetDiscussion{CommentID: c.ReportedID, LockOwner: true}
			if err := getDiscussion(ctx, owner); err != nil {
				return err
			}

			comment := &query.GetCommentByID{CommentID: c.ReportedID}
			if err := getCommentByID(ctx, comment); err != nil {
				return err
			}

			if !comment.Result.AllowedActions(user, owner.Result, tenant, time.Now()).Report {
				targetFailure = validate.Unauthorized()
			}

		default:
			return app.ErrNotFound
		}

		var status struct {
			CountToday int    `db:"count_today"`
			PendingID  int    `db:"pending_id"`
			Reason     string `db:"reason"`
			Details    string `db:"details"`
		}
		if err := trx.Get(&status, `
            SELECT daily.count_today, COALESCE(pending.id, 0) AS pending_id,
                   COALESCE(pending.reason, '') AS reason, COALESCE(pending.details, '') AS details
            FROM (
                SELECT COUNT(*) AS count_today FROM reports
                WHERE tenant_id = $1 AND reporter_id = $2 AND created_at >= CURRENT_DATE
            ) daily
            LEFT JOIN (
                SELECT id, reason, details FROM reports
                WHERE tenant_id = $1 AND reporter_id = $2 AND reported_type = $3
                  AND reported_id = $4 AND status = 'pending'
                ORDER BY (reason = $5 AND COALESCE(details, '') = $6) DESC, id
                LIMIT 1
            ) pending ON TRUE
        `, tenant.ID, user.ID, c.ReportedType.String(), c.ReportedID, c.Reason, c.Details); err != nil {
			return err
		}

		if status.PendingID != 0 {
			if status.Reason == c.Reason && status.Details == c.Details {
				c.Result = status.PendingID
				return nil
			}

			failure.AddFieldFailure("reportedId", i18n.T(ctx, "validation.custom.alreadyreported"))
			return failure
		}

		if tenant.GeneralSettings != nil && tenant.GeneralSettings.ReportingGloballyDisabled {
			failure.AddFieldFailure("reportedId", i18n.T(ctx, "validation.custom.reportingdisabled"))
			return failure
		}

		if targetFailure != nil {
			return targetFailure
		}

		input := actions.CreateReport{Reason: c.Reason, Details: c.Details}
		if result := input.Validate(ctx); !result.Ok {
			return result
		}

		if status.CountToday >= tenant.DailyReportLimit() {
			failure.AddFieldFailure("reportedId", i18n.T(ctx, "validation.custom.reportlimitreached"))
			return failure
		}

		if err := trx.Scalar(&c.Result, `
            INSERT INTO reports (tenant_id, reporter_id, reported_type, reported_id, reason, details, status, created_at)
            VALUES ($1, $2, $3, $4, $5, $6, 'pending', NOW())
            RETURNING id
        `, tenant.ID, user.ID, c.ReportedType.String(), c.ReportedID, c.Reason, nullIfEmpty(c.Details)); err != nil {
			return err
		}

		c.Created = true
		return nil
	})
}

func assignReport(ctx context.Context, c *cmd.AssignReport) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if err := authorizeReportChange(ctx, user, c.ReportID); err != nil {
			return err
		}

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
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if err := authorizeReportChange(ctx, user, c.ReportID); err != nil {
			return err
		}

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
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if err := authorizeReportChange(ctx, user, c.ReportID); err != nil {
			return err
		}

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
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if err := authorizeReportChange(ctx, user, c.ReportID); err != nil {
			return err
		}

		_, err := trx.Execute(`
			DELETE FROM reports WHERE id = $1 AND tenant_id = $2
		`, c.ReportID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to delete report")
		}
		return nil
	})
}

func authorizeReportChange(ctx context.Context, user *entity.User, reportID int) error {
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	if !entity.Can(user, tenant, entity.ManageReports) {
		return validate.Unauthorized()
	}

	report := &query.GetReportByID{ReportID: reportID}
	if err := getReportByID(ctx, report); err != nil {
		return err
	}

	if report.Result.ReportedType == enum.ReportTypeComment {
		return getDiscussion(ctx, &query.GetDiscussion{CommentID: report.Result.ReportedID, LockOwner: true})
	}

	return nil
}

func getReportByID(ctx context.Context, q *query.GetReportByID) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		report := dbReport{}
		err := trx.Get(&report, visibleCommentOwners+`
			SELECT 
				r.id, r.reported_type, r.reported_id, r.reason, r.details, r.status, r.created_at,
				ru.id as reporter_id, ru.name as reporter_name,
				ru.role as reporter_role, ru.visual_role as reporter_visual_role, ru.status as reporter_status,
				ru.avatar_type as reporter_avatar_type, ru.avatar_bkey as reporter_avatar_bkey,
				au.id as assigned_to_id, au.name as assigned_to_name,
				au.role as assigned_to_role, au.visual_role as assigned_to_visual_role, au.status as assigned_to_status,
				au.avatar_type as assigned_to_avatar_type, au.avatar_bkey as assigned_to_avatar_bkey, r.assigned_at,
				r.resolved_at, rbu.id as resolved_by_id, rbu.name as resolved_by_name,
				rbu.role as resolved_by_role, rbu.visual_role as resolved_by_visual_role, rbu.status as resolved_by_status,
				rbu.avatar_type as resolved_by_avatar_type, rbu.avatar_bkey as resolved_by_avatar_bkey,
				r.resolution_note,
				COALESCE(p.number, cp.number) as post_number,
				COALESCE(p.slug, cp.slug) as post_slug, pg.slug AS page_slug
			FROM reports r
			LEFT JOIN users ru ON ru.id = r.reporter_id AND ru.tenant_id = r.tenant_id
			LEFT JOIN users au ON au.id = r.assigned_to AND au.tenant_id = r.tenant_id
			LEFT JOIN users rbu ON rbu.id = r.resolved_by AND rbu.tenant_id = r.tenant_id
			LEFT JOIN posts p ON r.reported_type = 'post' AND p.id = r.reported_id
			LEFT JOIN comments c ON r.reported_type = 'comment' AND c.id = r.reported_id
			LEFT JOIN posts cp ON c.post_id = cp.id
			LEFT JOIN pages pg ON c.page_id = pg.id AND pg.tenant_id = r.tenant_id
			WHERE r.tenant_id = $1 AND r.id = $6
			AND (r.reported_type <> 'comment' OR r.reported_id IN (SELECT id FROM visible_comment_owners))
		`, append(commentOwnerParams(tenant, user), q.ReportID)...)
		if err != nil {
			return errors.Wrap(err, "failed to get report by ID")
		}
		q.Result = report.toModel(ctx)
		return nil
	})
}

func listReports(ctx context.Context, q *query.ListReports) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if q.Page < 1 {
			q.Page = 1
		}
		if q.PerPage < 1 {
			q.PerPage = 20
		}
		offset := (q.Page - 1) * q.PerPage

		conditions := "r.tenant_id = $1 AND (r.reported_type <> 'comment' OR r.reported_id IN (SELECT id FROM visible_comment_owners))"
		args := commentOwnerParams(tenant, user)
		argIdx := len(args) + 1

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

		err := trx.Scalar(&q.Total, visibleCommentOwners+"SELECT COUNT(*) FROM reports r WHERE "+conditions, args...)
		if err != nil {
			return errors.Wrap(err, "failed to count reports")
		}

		var reports []*dbReport
		err = trx.Select(&reports, visibleCommentOwners+`
			SELECT 
				r.id, r.reported_type, r.reported_id, r.reason, r.details, r.status, r.created_at,
				ru.id as reporter_id, ru.name as reporter_name,
				ru.role as reporter_role, ru.visual_role as reporter_visual_role, ru.status as reporter_status,
				ru.avatar_type as reporter_avatar_type, ru.avatar_bkey as reporter_avatar_bkey,
				au.id as assigned_to_id, au.name as assigned_to_name,
				au.role as assigned_to_role, au.visual_role as assigned_to_visual_role, au.status as assigned_to_status,
				au.avatar_type as assigned_to_avatar_type, au.avatar_bkey as assigned_to_avatar_bkey, r.assigned_at,
				r.resolved_at, rbu.id as resolved_by_id, rbu.name as resolved_by_name,
				rbu.role as resolved_by_role, rbu.visual_role as resolved_by_visual_role, rbu.status as resolved_by_status,
				rbu.avatar_type as resolved_by_avatar_type, rbu.avatar_bkey as resolved_by_avatar_bkey,
				r.resolution_note,
				COALESCE(p.number, cp.number) as post_number,
				COALESCE(p.slug, cp.slug) as post_slug, pg.slug AS page_slug
			FROM reports r
			LEFT JOIN users ru ON ru.id = r.reporter_id AND ru.tenant_id = r.tenant_id
			LEFT JOIN users au ON au.id = r.assigned_to AND au.tenant_id = r.tenant_id
			LEFT JOIN users rbu ON rbu.id = r.resolved_by AND rbu.tenant_id = r.tenant_id
			LEFT JOIN posts p ON r.reported_type = 'post' AND p.id = r.reported_id
			LEFT JOIN comments c ON r.reported_type = 'comment' AND c.id = r.reported_id
			LEFT JOIN posts cp ON c.post_id = cp.id
			LEFT JOIN pages pg ON c.page_id = pg.id AND pg.tenant_id = r.tenant_id
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
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		err := trx.Scalar(&q.Result, visibleCommentOwners+`
			SELECT COUNT(*) FROM reports r
			WHERE r.tenant_id = $1 AND r.status IN ('pending', 'in_review')
			AND (r.reported_type <> 'comment' OR r.reported_id IN (SELECT id FROM visible_comment_owners))
		`, commentOwnerParams(tenant, user)...)
		if err != nil {
			return errors.Wrap(err, "failed to count pending reports")
		}
		return nil
	})
}

func getReportReasons(ctx context.Context, q *query.GetReportReasons) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
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

func getUserReportStatus(ctx context.Context, q *query.GetUserReportStatus) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		return trx.Get(q, `
            SELECT
                (SELECT COUNT(*) FROM reports
                 WHERE tenant_id = $1 AND reporter_id = $2 AND created_at >= CURRENT_DATE
                ) AS count_today,
                EXISTS (
                    SELECT 1 FROM reports
                    WHERE tenant_id = $1 AND reporter_id = $2 AND reported_type = 'post'
                      AND reported_id = $3 AND status = 'pending'
                ) AS has_reported_post,
                ARRAY (
                    SELECT reported_id FROM reports
                    WHERE tenant_id = $1 AND reporter_id = $2 AND reported_type = 'comment'
                      AND reported_id = ANY($4) AND status = 'pending'
                ) AS reported_comment_ids
        `, tenant.ID, user.ID, q.PostID, pq.Array(q.CommentIDs))
	})
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func listAllReportReasons(ctx context.Context, q *query.ListAllReportReasons) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
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
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
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
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
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
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
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
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
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
