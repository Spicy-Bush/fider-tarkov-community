package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

func createComment(ctx context.Context, c *cmd.CreateComment) error {
	c.Created = false
	c.Result = nil

	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if user == nil {
			return validate.Unauthorized()
		}

		if c.SubmissionID == "" || len(c.SubmissionID) > 128 {
			return validate.Failed("Invalid submission identity.")
		}

		owner := &query.GetDiscussion{PostNumber: c.PostNumber, PageID: c.PageID, LockOwner: true}
		if err := getDiscussion(ctx, owner); err != nil {
			return err
		}

		c.Discussion = owner.Result

		fingerprint, err := commentFingerprint(owner.Result.Owner, 0, c.ParentID, c.Content, c.Attachments)
		if err != nil {
			return err
		}

		identity := fmt.Sprintf("comment:%d:%d:%s", tenant.ID, user.ID, c.SubmissionID)
		if _, err := trx.Execute("SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", identity); err != nil {
			return err
		}

		var receipt struct {
			ID   int    `db:"id"`
			Hash string `db:"submission_hash"`
		}

		err = trx.Get(&receipt, `
            SELECT id, submission_hash FROM comments
            WHERE tenant_id = $1 AND user_id = $2 AND submission_id = $3
        `, tenant.ID, user.ID, c.SubmissionID)
		if err == nil {
			if receipt.Hash != fingerprint {
				return app.ErrConflict
			}

			comment := &query.GetCommentByID{CommentID: receipt.ID, IncludeDeleted: true}
			if err := getCommentByID(ctx, comment); err != nil {
				return err
			}

			c.Result = comment.Result
			return nil
		}

		if errors.Cause(err) != app.ErrNotFound {
			return err
		}

		input := actions.CommentInput{
			Discussion:  owner.Result,
			Content:     c.Content,
			Attachments: c.Attachments,
		}
		if validation := input.Validate(ctx, user); !validation.Ok {
			return validation
		}

		parentAuthorID := 0
		if c.ParentID != nil {
			parent := &query.GetCommentByID{CommentID: *c.ParentID, IncludeDeleted: true}
			if err := getCommentByID(ctx, parent); err != nil {
				return err
			}

			matchesPost := owner.Result.Owner.Kind == "post" && parent.Result.PostID == owner.Result.Owner.ID
			matchesPage := owner.Result.Owner.Kind == "page" && parent.Result.PageID == owner.Result.Owner.ID
			if !matchesPost && !matchesPage {
				return validate.Failed("The reply belongs to a different discussion.")
			}

			if !parent.Result.ForViewer(user, owner.Result, tenant, time.Now()).Permissions.Reply {
				return validate.Unauthorized()
			}

			parentAuthorID = parent.Result.User.ID
		}

		var postID, pageID *int
		if owner.Result.Owner.Kind == "post" {
			postID = &owner.Result.Owner.ID
		} else {
			pageID = &owner.Result.Owner.ID
		}

		content := storedCommentContent(c.Content)
		var id int
		if err := trx.Get(&id, `
            INSERT INTO comments (tenant_id, post_id, page_id, parent_id, content, user_id, created_at, submission_id, submission_hash)
            VALUES ($1, $2, $3, $4, $5, $6, clock_timestamp(), $7, $8)
            RETURNING id
        `, tenant.ID, postID, pageID, c.ParentID, content, user.ID, c.SubmissionID, fingerprint); err != nil {
			return err
		}

		attachmentPostID := 0
		if postID != nil {
			attachmentPostID = *postID
		}

		if err := setCommentAttachments(ctx, attachmentPostID, id, c.Attachments); err != nil {
			return err
		}

		if err := scheduleModeration(ctx, &cmd.ScheduleModeration{ContentType: "comment", ContentID: id}); err != nil {
			return err
		}

		if owner.Result.Owner.Kind == "post" && owner.Result.PostStatus == enum.PostArchived {
			post := &entity.Post{ID: owner.Result.Owner.ID}
			if err := unarchivePost(ctx, &cmd.UnarchivePost{Post: post, Reason: "New comment"}); err != nil {
				return err
			}
			owner.Result.PostStatus = post.Status
		}

		loaded := &query.GetCommentByID{CommentID: id}
		if err := getCommentByID(ctx, loaded); err != nil {
			return err
		}

		c.Result = loaded.Result
		c.Created = true
		return scheduleNotification(ctx, &cmd.ScheduleNotification{
			BaseURL: c.BaseURL,
			Comment: &entity.CommentNotification{
				CommentID:      id,
				Owner:          owner.Result.Owner,
				Content:        content,
				MentionIDs:     newCommentMentions(content, ""),
				ParentAuthorID: parentAuthorID,
			},
		})
	})
}

func commentFingerprint(owner entity.DiscussionOwner, commentID int, parentID *int, content string, attachments []*dto.ImageUpload) (string, error) {
	encoded, err := json.Marshal(struct {
		OwnerKind   string
		OwnerID     int
		CommentID   int
		ParentID    *int
		Content     string
		Attachments []*dto.ImageUpload
	}{owner.Kind, owner.ID, commentID, parentID, content, attachments})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", sha256.Sum256(encoded)), nil
}

func newCommentMentions(content, previous string) []int {
	known := make(map[int]bool)
	for _, mention := range entity.CommentString(previous).ParseMentions() {
		known[mention.ID] = true
	}

	ids := make([]int, 0)
	for _, mention := range entity.CommentString(content).ParseMentions() {
		if !known[mention.ID] {
			ids = append(ids, mention.ID)
			known[mention.ID] = true
		}
	}

	return ids
}

func storedCommentContent(content string) string {
	return entity.CommentString(content).FormatMentionJson(func(mention entity.Mention) string {
		name, _ := json.Marshal(mention.Name)
		return fmt.Sprintf(`{"id":%d,"name":%s}`, mention.ID, name)
	})
}
