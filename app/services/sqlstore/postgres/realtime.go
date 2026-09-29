package postgres

import (
	"context"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/lib/pq"
)

func getRealtimeAccess(ctx context.Context, q *query.GetRealtimeAccess) error {
	q.Result = make([]query.RealtimeAccess, len(q.Viewers))
	if len(q.Viewers) == 0 {
		return nil
	}

	tenantIDs := make([]int, len(q.Viewers))
	userIDs := make([]int, len(q.Viewers))
	pageIDs := make([]int, len(q.Viewers))
	for index, viewer := range q.Viewers {
		tenantIDs[index] = viewer.TenantID
		userIDs[index] = viewer.UserID
		pageIDs[index] = viewer.PageID
	}

	return dbx.InTransaction(ctx, func(ctx context.Context, trx *dbx.Trx) error {
		var rows []*struct {
			Position          int               `db:"position"`
			TenantID          int               `db:"tenant_id"`
			UserID            int               `db:"user_id"`
			Role              enum.Role         `db:"role"`
			UserStatus        enum.UserStatus   `db:"user_status"`
			TenantStatus      enum.TenantStatus `db:"tenant_status"`
			RolePermissions   string            `db:"role_permissions"`
			RolePostResponses string            `db:"role_post_responses"`
			PageExists        bool              `db:"page_exists"`
		}
		err := trx.Select(&rows, `
			SELECT viewer.position, u.tenant_id, u.id AS user_id, u.role, u.status AS user_status,
			       t.status AS tenant_status, t.role_permissions, t.role_post_responses,
			       EXISTS (SELECT 1 FROM pages p WHERE p.id=viewer.page_id AND p.tenant_id=viewer.tenant_id) AS page_exists
			FROM unnest($1::int[], $2::int[], $3::int[]) WITH ORDINALITY AS viewer(tenant_id, user_id, page_id, position)
			JOIN users u ON u.id=viewer.user_id AND u.tenant_id=viewer.tenant_id
			JOIN tenants t ON t.id = u.tenant_id
		`, pq.Array(tenantIDs), pq.Array(userIDs), pq.Array(pageIDs))
		if err != nil {
			return err
		}

		tenants := make(map[int]*entity.Tenant)
		for _, stored := range rows {
			if stored.TenantStatus == enum.TenantPending {
				continue
			}

			tenant := tenants[stored.TenantID]
			if tenant == nil {
				permissions, err := parseRolePermissions(stored.RolePermissions)
				if err != nil {
					return err
				}
				tenant = &entity.Tenant{ID: stored.TenantID, Status: stored.TenantStatus, RolePermissions: permissions}
				if tenant.RolePostResponses, err = parseRolePostResponses(stored.RolePostResponses); err != nil {
					return err
				}
				tenants[stored.TenantID] = tenant
			}

			user := &entity.User{ID: stored.UserID, Role: stored.Role, Status: stored.UserStatus}
			q.Result[stored.Position-1] = query.RealtimeAccess{
				Reports: entity.Can(user, tenant, entity.ManageReports),
				Queue:   entity.Can(user, tenant, entity.ManageQueue),
				Pages:   stored.PageExists && entity.Can(user, tenant, entity.ManagePages),
			}
		}
		return nil
	})
}
