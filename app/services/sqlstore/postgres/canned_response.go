package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
)

// Create a new canned response
func createCannedResponse(ctx context.Context, c *cmd.CreateCannedResponse) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var createdByID *int
		if user != nil {
			userID := user.ID
			createdByID = &userID
		}

		var id int
		now := time.Now()
		err := trx.Get(&id, `
			INSERT INTO canned_responses (tenant_id, type, title, content, duration, is_active, created_at, created_by_id) 
			VALUES ($1, $2, $3, $4, $5, true, $6, $7)
			RETURNING id`, tenant.ID, c.Type, c.Title, c.Content, c.Duration, now, createdByID)
		if err != nil {
			return errors.Wrap(err, "failed to create canned response")
		}

		c.Result = &entity.CannedResponse{
			ID:          id,
			Type:        c.Type,
			Title:       c.Title,
			Content:     c.Content,
			Duration:    c.Duration,
			IsActive:    true,
			CreatedAt:   now,
			CreatedByID: createdByID,
		}

		return nil
	})
}

// Update an existing canned response
func updateCannedResponse(ctx context.Context, c *cmd.UpdateCannedResponse) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute(`
			UPDATE canned_responses 
			SET type = $1, title = $2, content = $3, duration = $4, is_active = $5
			WHERE id = $6 AND tenant_id = $7
		`, c.Type, c.Title, c.Content, c.Duration, c.IsActive, c.ID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to update canned response")
		}

		// Get creator ID if available
		var createdByID *int
		if user != nil {
			userID := user.ID
			createdByID = &userID
		}

		c.Result = &entity.CannedResponse{
			ID:          c.ID,
			Type:        c.Type,
			Title:       c.Title,
			Content:     c.Content,
			Duration:    c.Duration,
			IsActive:    c.IsActive,
			CreatedByID: createdByID,
		}

		return nil
	})
}

// Delete a canned response
func deleteCannedResponse(ctx context.Context, c *cmd.DeleteCannedResponse) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute(`
			DELETE FROM canned_responses 
			WHERE id = $1 AND tenant_id = $2
		`, c.ID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to delete canned response")
		}
		return nil
	})
}

// Get a canned response by ID
func getCannedResponseByID(ctx context.Context, q *query.GetCannedResponseByID) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		response := &entity.CannedResponse{}

		err := trx.Get(response, `
            SELECT id, type, title, content, duration, is_active, created_at, created_by_id 
            FROM canned_responses 
            WHERE id = $1 AND tenant_id = $2
        `, q.ID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to get canned response")
		}

		q.Result = response
		return nil
	})
}

// List all canned responses of a specific type
func listCannedResponses(ctx context.Context, q *query.ListCannedResponses) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		rows, err := trx.Query(`
            SELECT id, type, title, content, duration, is_active, created_at, created_by_id 
            FROM canned_responses 
            WHERE type = $1 AND tenant_id = $2 AND is_active = true
            ORDER BY title ASC
        `, q.Type, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to query canned responses")
		}
		defer rows.Close()

		// using an empty slice so [] is returned when empty
		responses := []*entity.CannedResponse{}
		for rows.Next() {
			response := &entity.CannedResponse{}
			var createdByID sql.NullInt64
			err := rows.Scan(
				&response.ID,
				&response.Type,
				&response.Title,
				&response.Content,
				&response.Duration,
				&response.IsActive,
				&response.CreatedAt,
				&createdByID,
			)
			if err != nil {
				return errors.Wrap(err, "failed to scan canned response")
			}

			if createdByID.Valid {
				userID := int(createdByID.Int64)
				response.CreatedByID = &userID
			}

			responses = append(responses, response)
		}

		q.Result = responses
		return nil
	})
}
