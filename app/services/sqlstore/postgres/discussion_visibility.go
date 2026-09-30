package postgres

import (
	"context"
	"fmt"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/lib/pq"
)

func discussionSource(ctx context.Context, ownerColumn string, args *[]any) string {
	tenant := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	user, _ := ctx.Value(app.UserCtxKey).(*entity.User)
	*args = append(*args, pq.Array(entity.ModeratedContentRoles(user, tenant)))

	return fmt.Sprintf(`WITH RECURSIVE discussion_comments AS NOT MATERIALIZED (
        SELECT c.id, c.parent_id, c.created_at, c.deleted_at, c.moderation_pending,
            c.deleted_at IS NULL AND (
                NOT c.moderation_pending OR c.user_id = $2
                OR COALESCE(author.role, 0) = ANY($%[1]d::integer[])
            ) AS visible
        FROM comments c
        LEFT JOIN users author ON author.id = c.user_id AND author.tenant_id = c.tenant_id
        WHERE c.tenant_id = $1 AND c.%[2]s = $3
    ), concealed_comments AS MATERIALIZED (
        SELECT id, parent_id FROM discussion_comments
        WHERE (deleted_at IS NOT NULL OR moderation_pending) AND NOT visible
    ), promoted_parents AS (
        SELECT c.id, c.parent_id FROM concealed_comments c
        WHERE c.parent_id IS NULL OR c.parent_id NOT IN (SELECT id FROM concealed_comments)
        UNION ALL
        SELECT child.id, parent.parent_id
        FROM promoted_parents parent
        JOIN LATERAL (
            SELECT id FROM discussion_comments
            WHERE parent_id = parent.id AND (deleted_at IS NOT NULL OR moderation_pending)
              AND NOT visible
            -- Keep each recursive step on the parent index instead of rescanning the discussion.
            OFFSET 0
        ) child ON TRUE
    )`, len(*args), pq.QuoteIdentifier(ownerColumn))
}

func visibleDiscussionChildren(parent string) string {
	return fmt.Sprintf(`SELECT id, created_at FROM discussion_comments
    WHERE (parent_id = %[1]s OR (%[1]s::integer IS NULL AND parent_id IS NULL)) AND visible
    UNION ALL
    SELECT child.id, child.created_at
    FROM discussion_comments child JOIN promoted_parents promoted ON child.parent_id = promoted.id
    WHERE (promoted.parent_id = %[1]s OR (%[1]s::integer IS NULL AND promoted.parent_id IS NULL))
      AND child.visible`, parent)
}

func visibleDiscussionParent(child string) string {
	return fmt.Sprintf(`SELECT CASE WHEN promoted.id IS NULL THEN record.parent_id ELSE promoted.parent_id END AS parent_id
        FROM discussion_comments record LEFT JOIN promoted_parents promoted ON promoted.id = record.parent_id
        WHERE record.id = %s`, child)
}

func readVisibleComments(ctx context.Context, trx *dbx.Trx, ownerColumn, selection string, args ...any) ([]*entity.Comment, error) {
	source := discussionSource(ctx, ownerColumn, &args)
	details := fmt.Sprintf(commentDetails,
		"("+visibleDiscussionParent("c.id")+")",
		"EXISTS ("+visibleDiscussionChildren("c.id")+")",
		"EXISTS (SELECT 1 FROM discussion_comments visible WHERE visible.id = c.id AND visible.visible)",
	)

	var records []*dbComment
	if err := trx.Select(&records, source+", selected AS ("+selection+") "+details, args...); err != nil {
		return nil, err
	}

	comments := make([]*entity.Comment, 0, len(records))
	for _, record := range records {
		comments = append(comments, record.toModel(ctx))
	}
	return comments, nil
}
