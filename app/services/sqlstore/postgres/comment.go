package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

type dbComment struct {
	ID                int            `db:"id"`
	PostID            int            `db:"post_id"`
	PageID            int            `db:"page_id"`
	ParentID          dbx.NullInt    `db:"parent_id"`
	HasReplies        bool           `db:"has_replies"`
	Deleted           bool           `db:"deleted"`
	SortScore         int            `db:"sort_score"`
	Content           string         `db:"content"`
	CreatedAt         time.Time      `db:"created_at"`
	User              *dbUser        `db:"user"`
	Attachments       []string       `db:"attachment_bkeys"`
	EditedAt          dbx.NullTime   `db:"edited_at"`
	EditedBy          *dbUser        `db:"edited_by"`
	ReactionCounts    dbx.NullString `db:"reaction_counts"`
	ModerationPending bool           `db:"moderation_pending"`
	ModerationData    dbx.NullString `db:"moderation_data"`
}

func (c *dbComment) toModel(ctx context.Context) *entity.Comment {
	comment := &entity.Comment{
		ID:                c.ID,
		PostID:            c.PostID,
		PageID:            c.PageID,
		HasReplies:        c.HasReplies,
		Deleted:           c.Deleted,
		State:             "visible",
		SortScore:         c.SortScore,
		ModerationPending: c.ModerationPending,
		Content:           c.Content,
		CreatedAt:         c.CreatedAt,
		User:              c.User.toModel(ctx),
		Attachments:       c.Attachments,
	}

	if c.ParentID.Valid {
		id := int(c.ParentID.Int64)
		comment.ParentID = &id
	}

	if c.EditedAt.Valid {
		comment.EditedBy = c.EditedBy.toModel(ctx)
		comment.EditedAt = &c.EditedAt.Time
	}

	if c.ReactionCounts.Valid {
		_ = json.Unmarshal([]byte(c.ReactionCounts.String), &comment.ReactionCounts)
	}

	if c.ModerationData.Valid {
		comment.ModerationData = c.ModerationData.String
	}

	return comment
}

func setCommentReaction(ctx context.Context, c *cmd.SetCommentReaction) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		owner := &query.GetDiscussion{CommentID: c.CommentID, LockOwner: true}
		if err := getDiscussion(ctx, owner); err != nil {
			return err
		}

		comment := &query.GetCommentByID{CommentID: c.CommentID}
		if err := getCommentByID(ctx, comment); err != nil {
			return err
		}

		if !comment.Result.AllowedActions(user, owner.Result, tenant, time.Now()).React {
			return validate.Unauthorized()
		}

		var err error
		if c.Active {
			_, err = trx.Execute(`
                    INSERT INTO reactions (comment_id, user_id, emoji, created_on)
                    VALUES ($1, $2, $3, $4)
                    ON CONFLICT (comment_id, user_id, emoji) DO NOTHING
                `, c.CommentID, user.ID, c.Emoji, time.Now())
		} else {
			_, err = trx.Execute("DELETE FROM reactions WHERE comment_id = $1 AND user_id = $2 AND emoji = $3", c.CommentID, user.ID, c.Emoji)
		}
		if err != nil {
			return err
		}
		if err := getCommentByID(ctx, comment); err != nil {
			return err
		}
		c.Discussion = owner.Result
		c.Result = comment.Result
		return nil
	})
}

func updateComment(ctx context.Context, c *cmd.UpdateComment) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if user == nil {
			return validate.Unauthorized()
		}

		if c.SubmissionID == "" || len(c.SubmissionID) > 128 {
			return validate.Failed("Invalid submission identity.")
		}

		owner := &query.GetDiscussion{CommentID: c.CommentID, LockOwner: true}
		if err := getDiscussion(ctx, owner); err != nil {
			return err
		}
		c.Discussion = owner.Result

		if _, err := trx.Execute("SELECT id FROM comments WHERE tenant_id = $1 AND id = $2 FOR UPDATE", tenant.ID, c.CommentID); err != nil {
			return err
		}

		comment := &query.GetCommentByID{CommentID: c.CommentID, IncludeDeleted: true}
		if err := getCommentByID(ctx, comment); err != nil {
			return err
		}

		fingerprint, err := commentFingerprint(owner.Result.Owner, c.CommentID, comment.Result.ParentID, c.Content, c.Attachments)
		if err != nil {
			return err
		}

		identity := fmt.Sprintf("comment-edit:%d:%d:%s", tenant.ID, user.ID, c.SubmissionID)
		if _, err := trx.Execute("SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", identity); err != nil {
			return err
		}

		var savedHash string
		err = trx.Scalar(&savedHash, `
            SELECT submission_hash FROM comment_edit_receipts
            WHERE tenant_id = $1 AND user_id = $2 AND submission_id = $3
        `, tenant.ID, user.ID, c.SubmissionID)
		if err == nil {
			if savedHash != fingerprint {
				return app.ErrConflict
			}

			c.Result = comment.Result
			return nil
		}

		if errors.Cause(err) != app.ErrNotFound {
			return err
		}

		input := actions.CommentInput{
			Discussion:  owner.Result,
			Comment:     comment.Result,
			Content:     c.Content,
			Attachments: c.Attachments,
		}
		if validation := input.Validate(ctx, user); !validation.Ok {
			return validation
		}

		previousContent := comment.Result.Content
		content := storedCommentContent(c.Content)
		_, err = trx.Execute(`
			UPDATE comments SET content = $1, edited_by_id = $2,
                edited_at = GREATEST(CURRENT_TIMESTAMP,
                    COALESCE(edited_at, created_at) + INTERVAL '1 microsecond')
			WHERE id = $3 AND tenant_id = $4`, content, user.ID, c.CommentID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed update comment")
		}

		if err := setCommentAttachments(ctx, comment.Result.PostID, c.CommentID, c.Attachments); err != nil {
			return err
		}

		if err := scheduleModeration(ctx, &cmd.ScheduleModeration{ContentType: "comment", ContentID: c.CommentID}); err != nil {
			return err
		}

		if err := getCommentByID(ctx, comment); err != nil {
			return err
		}

		c.Result = comment.Result

		mentions := newCommentMentions(content, previousContent)
		if len(mentions) > 0 {
			if err := scheduleNotification(ctx, &cmd.ScheduleNotification{
				BaseURL: c.BaseURL,
				Comment: &entity.CommentNotification{
					CommentID:  c.CommentID,
					Owner:      owner.Result.Owner,
					Content:    content,
					MentionIDs: mentions,
					Edited:     true,
				},
			}); err != nil {
				return err
			}
		}

		_, err = trx.Execute(`
            INSERT INTO comment_edit_receipts (tenant_id, user_id, submission_id, comment_id, submission_hash)
            VALUES ($1, $2, $3, $4, $5)
        `, tenant.ID, user.ID, c.SubmissionID, c.CommentID, fingerprint)
		return err
	})
}

func deleteComment(ctx context.Context, c *cmd.DeleteComment) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		owner := &query.GetDiscussion{CommentID: c.CommentID, LockOwner: true}
		if err := getDiscussion(ctx, owner); err != nil {
			return err
		}

		comment := &query.GetCommentByID{CommentID: c.CommentID, IncludeDeleted: true}
		if err := getCommentByID(ctx, comment); err != nil {
			return err
		}

		if !comment.Result.AllowedActions(user, owner.Result, tenant, time.Now()).Delete {
			return validate.Unauthorized()
		}

		c.Discussion = owner.Result
		c.Result = comment.Result
		if comment.Result.Deleted {
			return nil
		}

		if _, err := trx.Execute(
			"UPDATE comments SET deleted_at = $1, deleted_by_id = $2 WHERE id = $3 AND tenant_id = $4",
			time.Now(), user.ID, c.CommentID, tenant.ID,
		); err != nil {
			return errors.Wrap(err, "failed delete comment")
		}
		c.Result.Deleted = true
		return nil
	})
}

func getCommentByID(ctx context.Context, q *query.GetCommentByID) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		q.Result = nil
		viewerID := 0
		if user != nil {
			viewerID = user.ID
		}

		comments, err := readComments(ctx, trx, `
            SELECT id, 0 AS position FROM comments
            WHERE tenant_id = $1 AND id = $3 AND (deleted_at IS NULL OR $4)
        `, tenant.ID, viewerID, q.CommentID, q.IncludeDeleted)
		if err != nil {
			return err
		}

		if len(comments) == 0 {
			return app.ErrNotFound
		}

		q.Result = comments[0]
		return nil
	})
}
