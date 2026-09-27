package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
)

// Keep the original profile value across edits until a check succeeds. Otherwise
// a late rejection could restore another value published during the same outage.
const replaceModerationCheck = `
 ON CONFLICT (tenant_id, content_type, content_id) DO UPDATE
 SET revision = moderation_checks.revision + 1, state = 'pending',
     text_content = EXCLUDED.text_content, blob_keys = EXCLUDED.blob_keys,
     attempts = 0, next_attempt_at = NOW(), last_error = '', result = NULL, updated_at = NOW()`

func scheduleModeration(ctx context.Context, c *cmd.ScheduleModeration) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if !env.IsOpenAIModerationEnabled() {
			return cancelModerationCheck(trx, tenant.ID, c.ContentID, c.ContentType)
		}

		var table, text, attachmentFilter string

		switch c.ContentType {
		case "post":
			table, text, attachmentFilter = "posts", "title || E'\n\n' || description", "post_id = $2 AND comment_id IS NULL"
		case "comment":
			table, text, attachmentFilter = "comments", "content", "comment_id = $2"
		default:
			return fmt.Errorf("invalid moderation content type %q", c.ContentType)
		}

		_, err := trx.Execute(`
         WITH content AS (
             SELECT `+text+` AS text FROM `+table+`
             WHERE tenant_id = $1 AND id = $2 FOR UPDATE
         )
         INSERT INTO moderation_checks (tenant_id, content_type, content_id, state, text_content, blob_keys)
         SELECT $1, $3, $2, 'pending', content.text,
             (SELECT COALESCE(jsonb_agg(DISTINCT attachment_bkey), '[]')
              FROM attachments WHERE tenant_id = $1 AND `+attachmentFilter+`)
         FROM content`+replaceModerationCheck, tenant.ID, c.ContentID, c.ContentType)
		return err
	})
}

func saveModerationCheck(trx *dbx.Trx, tenantID int, kind string, id int, text, keysJSON string) error {
	_, err := trx.Execute(`
     INSERT INTO moderation_checks (tenant_id, content_type, content_id, state, text_content, blob_keys)
     VALUES ($1, $2, $3, 'pending', $4, $5::jsonb)`+replaceModerationCheck,
		tenantID, kind, id, text, keysJSON)
	return err
}

func claimModeration(ctx context.Context, c *cmd.ClaimModeration) error {
	check := &cmd.ModerationCheck{}
	var keys []byte
	var coolingDown bool

	err := dbx.Connection().QueryRowContext(ctx, `
        SELECT available_at > NOW()
        FROM moderation_provider
        WHERE id = 1`).Scan(&coolingDown)

	if err != nil {
		return err
	}

	// The explicit predicate lets PostgreSQL skip the blocked post/comment backlog.
	contentFilter := ""

	if coolingDown {
		contentFilter = "AND content_type IN ('name', 'avatar')"
	}

	// An edit can replace the check while its request is still using a provider slot.
	err = dbx.Connection().QueryRowContext(ctx, `
 WITH candidate AS (
     SELECT tenant_id, content_type, content_id
     FROM moderation_checks
     WHERE state IN ('pending', 'running') AND next_attempt_at <= NOW()
       `+contentFilter+`
     ORDER BY next_attempt_at
     FOR UPDATE SKIP LOCKED LIMIT 1
 ), capacity AS (
     SELECT id FROM moderation_provider_slots
     WHERE id <= $1 AND lease_until <= NOW() AND EXISTS (SELECT 1 FROM candidate)
     ORDER BY lease_until
     FOR UPDATE SKIP LOCKED LIMIT 1
 ), reserved AS (
     UPDATE moderation_provider_slots
     SET claim = claim + 1, lease_until = NOW() + INTERVAL '2 minutes'
     WHERE id IN (SELECT id FROM capacity)
     RETURNING id, claim
 ), claimed AS (
     UPDATE moderation_checks
     SET state = 'running', claim = claim + 1, attempts = attempts + 1,
         next_attempt_at = NOW() + INTERVAL '2 minutes', updated_at = NOW()
     WHERE (tenant_id, content_type, content_id) IN (SELECT * FROM candidate)
       AND EXISTS (SELECT 1 FROM reserved)
     RETURNING tenant_id, content_type, content_id, revision, claim, text_content, blob_keys, attempts
 )
 SELECT claimed.*, reserved.id, reserved.claim,
        (SELECT available_at FROM moderation_provider WHERE id = 1)
 FROM claimed CROSS JOIN reserved`,
		env.Config.OpenAI.Concurrency).Scan(&check.TenantID, &check.ContentType, &check.ContentID,
		&check.Revision, &check.Claim, &check.Text, &keys, &check.Attempts, &check.Slot, &check.SlotClaim, &check.ProviderAvailableAt)

	if err == sql.ErrNoRows {
		return nil
	}

	if err != nil {
		return err
	}

	if err = json.Unmarshal(keys, &check.BlobKeys); err != nil {
		return err
	}

	c.Result = check
	return nil
}

func finishModeration(ctx context.Context, c *cmd.FinishModeration) error {
	state := "complete"

	switch c.Outcome {
	case cmd.ModerationReviewed:
	case cmd.ModerationRetry:
		state = "pending"
	case cmd.ModerationFailed:
		state = "failed"
	default:
		return fmt.Errorf("invalid moderation outcome %d", c.Outcome)
	}

	tx, err := dbx.Connection().BeginTx(ctx, nil)

	if err != nil {
		return err
	}

	defer tx.Rollback()
	check := c.Check
	table, active := "posts", "status <> 6"

	if check.ContentType == "comment" {
		table, active = "comments", "deleted_at IS NULL"
	} else if check.ContentType == "name" || check.ContentType == "avatar" {
		table, active = "users", "status = 1"
	} else if check.ContentType != "post" {
		return fmt.Errorf("invalid moderation content type")
	}

	// Match the writers' lock order to avoid deadlocking with an edit or file rename.
	var id int
	err = tx.QueryRowContext(ctx, `
        SELECT id
        FROM `+table+`
        WHERE tenant_id = $1 AND id = $2 AND `+active+`
        FOR UPDATE`, check.TenantID, check.ContentID).Scan(&id)
	profile := check.ContentType == "name" || check.ContentType == "avatar"

	if profile && state == "complete" && len(c.Findings) > 0 {
		state = "rejected"
	}

	if err == sql.ErrNoRows {
		state = "canceled"
	} else if err != nil {
		return err
	}

	result := c.Result

	if result == "" {
		result = "null"
	}

	var fallback []byte
	err = tx.QueryRowContext(ctx, `
        UPDATE moderation_checks
        SET state = $1, next_attempt_at = NOW() + $2 * INTERVAL '1 second',
            last_error = $3, result = $4::jsonb, updated_at = NOW(),
            fallback_profile = CASE
                WHEN $1 IN ('pending', 'failed') AND content_type IN ('name', 'avatar')
                THEN COALESCE(fallback_profile, (
                    SELECT CASE
                        WHEN $6 = 'name' THEN jsonb_build_object('name', name)
                        ELSE jsonb_build_object('avatar_type', avatar_type, 'avatar_bkey', avatar_bkey)
                    END
                    FROM users
                    WHERE tenant_id = $5 AND id = $7
                ))
                WHEN $1 = 'canceled' THEN NULL
                ELSE fallback_profile
            END,
            text_content = CASE WHEN $1 IN ('complete', 'canceled') THEN '' ELSE text_content END,
            blob_keys = CASE WHEN $1 IN ('complete', 'canceled') THEN '[]'::jsonb ELSE blob_keys END
        WHERE tenant_id = $5 AND content_type = $6 AND content_id = $7
          AND revision = $8 AND claim = $9 AND state = 'running'
        RETURNING fallback_profile`,
		state, c.RetryAfterSeconds, c.Error, result, check.TenantID, check.ContentType, check.ContentID, check.Revision, check.Claim).Scan(&fallback)
	applied := err != sql.ErrNoRows

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	// A stale completion still owns its slot unless another attempt has reclaimed it.
	released, err := tx.ExecContext(ctx, `
     UPDATE moderation_provider_slots SET lease_until = NOW()
     WHERE id = $1 AND claim = $2`, check.Slot, check.SlotClaim)

	if err != nil {
		return err
	}

	releasedCount, err := released.RowsAffected()

	if err != nil {
		return err
	}

	if releasedCount > 0 && c.CooldownSeconds > 0 {
		if _, err = tx.ExecContext(ctx, `
         UPDATE moderation_provider
         SET available_at = GREATEST(available_at, NOW() + $1 * INTERVAL '1 second')
         WHERE id = 1`, c.CooldownSeconds); err != nil {
			return err
		}
	}

	if !applied {
		return tx.Commit()
	}

	if profile && state != "canceled" {
		if state == "rejected" {
			if fallback != nil {
				if check.ContentType == "name" {
					err = tx.QueryRowContext(ctx, `
                        UPDATE users
                        SET name = $1::jsonb->>'name'
                        WHERE tenant_id = $2 AND id = $3
                        RETURNING name`, string(fallback), check.TenantID, check.ContentID).Scan(&c.PublishedName)
				} else {
					_, err = tx.ExecContext(ctx, `
                        UPDATE users
                        SET avatar_type = ($1::jsonb->>'avatar_type')::integer,
                            avatar_bkey = $1::jsonb->>'avatar_bkey'
                        WHERE tenant_id = $2 AND id = $3`, string(fallback), check.TenantID, check.ContentID)
				}
			}
		} else if check.ContentType == "name" {
			err = tx.QueryRowContext(ctx, `
                UPDATE users
                SET name = $1
                WHERE tenant_id = $2 AND id = $3
                RETURNING name`, check.Text, check.TenantID, check.ContentID).Scan(&c.PublishedName)
		} else {
			_, err = tx.ExecContext(ctx, `
                UPDATE users
                SET avatar_type = $1, avatar_bkey = $2
                WHERE tenant_id = $3 AND id = $4`, enum.AvatarTypeCustom, check.BlobKeys[0], check.TenantID, check.ContentID)
		}

		if err != nil {
			return err
		}

		if state == "complete" || state == "rejected" {
			if _, err = tx.ExecContext(ctx, `
                UPDATE moderation_checks
                SET fallback_profile = NULL
                WHERE tenant_id = $1 AND content_type = $2 AND content_id = $3`,
				check.TenantID, check.ContentType, check.ContentID); err != nil {
				return err
			}
		}
	}

	if state == "complete" && !profile {
		if _, err = tx.ExecContext(ctx, `
            UPDATE `+table+`
            SET moderation_pending = $1, moderation_data = $2::jsonb
            WHERE tenant_id = $3 AND id = $4
              AND moderation_data->>'source' IS DISTINCT FROM 'staff'`,
			len(c.Findings) > 0, result, check.TenantID, check.ContentID); err != nil {
			return err
		}
	}

	if state == "complete" && len(c.Findings) > 0 && !profile {
		categories := make([]string, len(c.Findings))

		for i, finding := range c.Findings {
			categories[i] = fmt.Sprintf("%s (%.2f)", finding.Category, finding.Score)
		}

		// Completing the claimed revision and inserting its report must commit together.

		if _, err = tx.ExecContext(ctx, `
            INSERT INTO reports (tenant_id, reporter_id, reported_type, reported_id, reason, details, status, created_at)
            VALUES ($1, NULL, $2, $3, 'Auto-flagged by AI moderation', $4, 'pending', NOW())`,
			check.TenantID, check.ContentType, check.ContentID, "Flagged categories: "+strings.Join(categories, ", ")); err != nil {
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	c.Applied = state == "complete" || state == "rejected"
	return nil
}

func listModerationFailures(ctx context.Context, c *cmd.ListModerationFailures) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		c.Result = []cmd.ModerationCheck{}
		var counts struct {
			Total  int `db:"total"`
			Failed int `db:"failed"`
		}

		if err := trx.Get(&counts, visibleCommentOwners + `
            SELECT COUNT(*) AS total, COUNT(*) FILTER (WHERE state = 'failed') AS failed
            FROM moderation_checks
            WHERE tenant_id = $1 AND state IN ('failed', 'pending', 'running')
              AND (content_type <> 'comment' OR content_id IN (SELECT id FROM visible_comment_owners))`, tenant.ID, discussionViewerRole(user)); err != nil {
			return err
		}

		c.Total, c.Failed = counts.Total, counts.Failed
		rows, err := trx.Query(visibleCommentOwners + `
            SELECT content_type, content_id, revision, attempts, state, last_error
            FROM moderation_checks
            WHERE tenant_id = $1 AND state IN ('failed', 'pending', 'running')
              AND (content_type <> 'comment' OR content_id IN (SELECT id FROM visible_comment_owners))
            ORDER BY (state = 'failed') DESC, updated_at LIMIT 100`, tenant.ID, discussionViewerRole(user))

		if err != nil {
			return err
		}

		defer rows.Close()

		for rows.Next() {
			var v cmd.ModerationCheck

			if err = rows.Scan(&v.ContentType, &v.ContentID, &v.Revision, &v.Attempts, &v.State, &v.LastError); err != nil {
				return err
			}

			c.Result = append(c.Result, v)
		}

		return rows.Err()
	})
}

func retryModerationFailures(ctx context.Context, c *cmd.RetryModerationFailures) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var err error
		c.Count, err = trx.Execute(visibleCommentOwners + `
            UPDATE moderation_checks
            SET state = 'pending', attempts = 0, next_attempt_at = NOW(), last_error = '', updated_at = NOW()
            WHERE tenant_id = $1 AND state = 'failed'
              AND (content_type <> 'comment' OR content_id IN (SELECT id FROM visible_comment_owners))`, tenant.ID, discussionViewerRole(user))
		return err
	})
}

func saveProfileName(ctx context.Context, c *cmd.SaveProfileName) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var current string

		if err := trx.Scalar(&current, `
            SELECT name
            FROM users
            WHERE id = $1 AND tenant_id = $2 AND status <> $3
            FOR UPDATE`, c.UserID, tenant.ID, enum.UserDeleted); err != nil {
			return err
		}

		c.Pending = c.Review && c.Name != current

		if c.Review && c.Name == current {
			// Resaving a value published during an outage must not bypass its check.
			if err := trx.Scalar(&c.Pending, `
                SELECT EXISTS (
                    SELECT 1 FROM moderation_checks
                    WHERE tenant_id = $1 AND content_type = 'name' AND content_id = $2
                      AND state IN ('pending', 'running', 'failed')
                      AND (fallback_profile IS NOT NULL OR text_content = $3)
                )`, tenant.ID, c.UserID, c.Name); err != nil {
				return err
			}
		}

		if c.Pending {
			return saveModerationCheck(trx, tenant.ID, "name", c.UserID, c.Name, "[]")
		}

		if _, err := trx.Execute(`UPDATE users SET name = $1 WHERE tenant_id = $2 AND id = $3`, c.Name, tenant.ID, c.UserID); err != nil {
			return err
		}

		return cancelModerationCheck(trx, tenant.ID, c.UserID, "name")
	})
}

func saveProfileAvatar(ctx context.Context, c *cmd.SaveProfileAvatar) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var current struct {
			Key  string          `db:"avatar_bkey"`
			Type enum.AvatarType `db:"avatar_type"`
		}

		if err := trx.Get(&current, `
            SELECT avatar_bkey, avatar_type
            FROM users
            WHERE id = $1 AND tenant_id = $2 AND status <> $3
            FOR UPDATE`, c.UserID, tenant.ID, enum.UserDeleted); err != nil {
			return err
		}

		if c.AvatarType != enum.AvatarTypeCustom {
			c.BlobKey = ""
		}

		// Older rows can retain a custom key after switching to a generated avatar.
		published := current.Type == enum.AvatarTypeCustom && c.BlobKey == current.Key
		c.Pending = c.Review && c.BlobKey != "" && !published

		if c.Review && c.AvatarType == enum.AvatarTypeCustom && published {
			if err := trx.Scalar(&c.Pending, `
                SELECT EXISTS (
                    SELECT 1 FROM moderation_checks
                    WHERE tenant_id = $1 AND content_type = 'avatar' AND content_id = $2
                      AND state IN ('pending', 'running', 'failed')
                      AND (fallback_profile IS NOT NULL OR blob_keys ? $3)
                )`, tenant.ID, c.UserID, c.BlobKey); err != nil {
				return err
			}
		}

		if c.Pending {
			keys, err := json.Marshal([]string{c.BlobKey})

			if err != nil {
				return err
			}

			return saveModerationCheck(trx, tenant.ID, "avatar", c.UserID, "", string(keys))
		}

		if _, err := trx.Execute(`
            UPDATE users
            SET avatar_type = $1, avatar_bkey = $2
            WHERE tenant_id = $3 AND id = $4`, c.AvatarType, c.BlobKey, tenant.ID, c.UserID); err != nil {
			return err
		}

		return cancelModerationCheck(trx, tenant.ID, c.UserID, "avatar")
	})
}

func cancelModerationCheck(trx *dbx.Trx, tenantID, contentID int, kind string) error {
	_, err := trx.Execute(`
     UPDATE moderation_checks
     SET revision = revision + 1, state = 'canceled', text_content = '',
         blob_keys = '[]', result = NULL, fallback_profile=NULL, updated_at = NOW()
     WHERE tenant_id = $1 AND content_type = $2 AND content_id = $3`, tenantID, kind, contentID)
	return err
}

func getProfileModeration(ctx context.Context, c *cmd.GetProfileModeration) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		c.Result = []cmd.ModerationCheck{}
		rows, err := trx.Query(`
            SELECT content_type, revision, state, text_content, blob_keys
            FROM moderation_checks
            WHERE tenant_id = $1 AND content_id = $2
              AND content_type IN ('name', 'avatar') AND state <> 'canceled'`, tenant.ID, c.UserID)

		if err != nil {
			return err
		}

		defer rows.Close()

		for rows.Next() {
			var check cmd.ModerationCheck
			var keys []byte

			if err = rows.Scan(&check.ContentType, &check.Revision, &check.State, &check.Text, &keys); err != nil {
				return err
			}

			if err = json.Unmarshal(keys, &check.BlobKeys); err != nil {
				return err
			}

			c.Result = append(c.Result, check)
		}

		return rows.Err()
	})
}
