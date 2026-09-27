package api

import (
	"encoding/base64"
	"encoding/json"
	"slices"
	"strconv"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/metrics"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/postcache"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

type commentCursor struct {
	CreatedAt time.Time `json:"createdAt"`
	ID        int       `json:"id"`
	Score     int       `json:"score"`
	Sort      string    `json:"sort"`
}

func projectDiscussionComments(c *web.Context, discussion *entity.Discussion, comments []*entity.Comment) error {
	status := &query.GetUserReportStatus{}
	now := time.Now()

	for index, comment := range comments {
		visible := comment.ForViewer(c.User(), discussion, c.Tenant(), now)
		comments[index] = visible

		if visible.Permissions.Report {
			status.CommentIDs = append(status.CommentIDs, visible.ID)
		}
	}

	if len(status.CommentIDs) == 0 {
		return nil
	}

	if err := bus.Dispatch(c, status); err != nil {
		return err
	}

	limitReached := status.CountToday >= c.Tenant().DailyReportLimit()
	for _, comment := range comments {
		if limitReached || slices.Contains(status.ReportedCommentIDs, int64(comment.ID)) {
			comment.Permissions.Report = false
		}
	}

	return nil
}

func CommentResponse(c *web.Context, discussion *entity.Discussion, comment *entity.Comment) error {
	comments := []*entity.Comment{comment}
	if err := projectDiscussionComments(c, discussion, comments); err != nil {
		return c.Failure(err)
	}
	return c.Ok(comments[0])
}

func discussionRequest(c *web.Context) (*query.GetDiscussion, error) {
	if c.Param("number") != "" {
		number, err := c.ParamAsInt("number")
		return &query.GetDiscussion{PostNumber: number}, err
	}

	pageID, err := c.ParamAsInt("id")
	return &query.GetDiscussion{PageID: pageID}, err
}

func ListDiscussion() web.HandlerFunc {
	return func(c *web.Context) error {
		owner, err := discussionRequest(c)
		if err != nil {
			return c.NotFound()
		}

		if err := bus.Dispatch(c, owner); err != nil {
			return c.Failure(err)
		}

		comments := &query.GetDiscussionComments{Discussion: owner.Result}
		depth := 1
		if requested := c.QueryParam("depth"); requested != "" {
			depth, err = strconv.Atoi(requested)
			if err != nil || depth < 1 || depth > 10 {
				return c.BadRequest(web.Map{"message": "Invalid discussion depth."})
			}
		}

		comments.Sort = c.QueryParam("sort")
		if comments.Sort == "" {
			comments.Sort = "liked"
		}

		switch comments.Sort {
		case "liked", "disliked", "replies", "latest":
		default:
			return c.BadRequest(web.Map{"message": "Invalid discussion order."})
		}

		if parent := c.QueryParam("parentId"); parent != "" {
			id, err := strconv.Atoi(parent)
			if err != nil || id <= 0 {
				return c.BadRequest(web.Map{"message": "Invalid reply parent."})
			}

			comments.ParentID = &id
		}

		if after := c.QueryParam("after"); after != "" {
			var cursor commentCursor
			decoded, err := base64.RawURLEncoding.DecodeString(after)
			if err != nil || json.Unmarshal(decoded, &cursor) != nil || cursor.ID <= 0 || cursor.CreatedAt.IsZero() || cursor.Sort != comments.Sort {
				return c.BadRequest(web.Map{"message": "Invalid comment cursor."})
			}

			comments.After = cursor.CreatedAt
			comments.AfterID = cursor.ID
			comments.AfterScore = cursor.Score
		}

		if err := bus.Dispatch(c, comments); err != nil {
			return c.Failure(err)
		}

		response := entity.DiscussionPage{
			Owner:       owner.Result.Owner,
			Permissions: owner.Result.Permissions(c.User(), c.Tenant()),
			Comments:    comments.Result,
		}

		if len(response.Comments) > 25 {
			response.Comments = response.Comments[:25]
			last := response.Comments[24]
			cursor, err := json.Marshal(commentCursor{
				CreatedAt: last.CreatedAt,
				ID:        last.ID,
				Score:     last.SortScore,
				Sort:      comments.Sort,
			})
			if err != nil {
				return err
			}

			response.Next = base64.RawURLEncoding.EncodeToString(cursor)
		}

		if comments.ParentID != nil && depth > 1 {
			chain := &query.GetDiscussionChainReplies{Depth: depth - 1}
			for _, comment := range response.Comments {
				if comment.HasReplies {
					chain.ParentIDs = append(chain.ParentIDs, comment.ID)
				}
			}

			if len(chain.ParentIDs) > 0 {
				if err := bus.Dispatch(c, chain); err != nil {
					return c.Failure(err)
				}

				response.Replies = chain.Result
			}
		}

		projected := make([]*entity.Comment, 0, len(response.Comments)+len(response.Replies))
		projected = append(projected, response.Comments...)
		projected = append(projected, response.Replies...)
		if err := projectDiscussionComments(c, owner.Result, projected); err != nil {
			return c.Failure(err)
		}

		response.Replies = projected[len(response.Comments):]
		response.Comments = projected[:len(response.Comments)]

		c.Response.Header().Set("Cache-Control", "private, no-store")
		return c.Ok(response)
	}
}

func ReadDiscussionComment() web.HandlerFunc {
	return func(c *web.Context) error {
		id, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		owner := &query.GetDiscussion{CommentID: id}
		if err := bus.Dispatch(c, owner); err != nil {
			return c.Failure(err)
		}
		ancestors := &query.GetCommentAncestors{CommentID: id, Discussion: owner.Result}
		if err := bus.Dispatch(c, ancestors); err != nil {
			return c.Failure(err)
		}

		comments := ancestors.Result
		response := entity.CommentContext{
			CommentID: id,
			DiscussionPage: entity.DiscussionPage{
				Owner:       owner.Result.Owner,
				Permissions: owner.Result.Permissions(c.User(), c.Tenant()),
				Comments:    comments,
			},
		}

		if len(comments) > 0 {
			response.NextAncestorID = comments[0].ParentID
		}

		if err := projectDiscussionComments(c, owner.Result, response.Comments); err != nil {
			return c.Failure(err)
		}

		c.Response.Header().Set("Cache-Control", "private, no-store")
		return c.Ok(response)
	}
}

func CreateDiscussionComment() web.HandlerFunc {
	return func(c *web.Context) error {
		owner, err := discussionRequest(c)
		if err != nil {
			return c.NotFound()
		}

		var input struct {
			Content      string             `json:"content"`
			ParentID     *int               `json:"parentId"`
			Attachments  []*dto.ImageUpload `json:"attachments"`
			SubmissionID string             `json:"submissionId"`
		}
		if err := c.Bind(&input); err != nil {
			return c.BadRequest(web.Map{"message": "Invalid comment."})
		}

		create := &cmd.CreateComment{
			PostNumber:   owner.PostNumber,
			PageID:       owner.PageID,
			ParentID:     input.ParentID,
			Content:      input.Content,
			Attachments:  input.Attachments,
			SubmissionID: input.SubmissionID,
			BaseURL:      web.BaseURL(c),
		}
		if err := bus.Dispatch(c, create); err != nil {
			return c.Failure(err)
		}

		if create.Created {
			metrics.TotalComments.Inc()
			if create.Discussion.Owner.Kind == "post" {
				postcache.InvalidateTenantRankings(c.Tenant().ID)
				postcache.InvalidateCountPerStatus(c.Tenant().ID)
			}
		}

		return CommentResponse(c, create.Discussion, create.Result)
	}
}

func EditDiscussionComment() web.HandlerFunc {
	return func(c *web.Context) error {
		id, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		var input struct {
			Content      string             `json:"content"`
			Attachments  []*dto.ImageUpload `json:"attachments"`
			SubmissionID string             `json:"submissionId"`
		}
		if err := c.Bind(&input); err != nil {
			return c.BadRequest(web.Map{"message": "Invalid comment."})
		}

		update := &cmd.UpdateComment{
			CommentID:    id,
			Content:      input.Content,
			Attachments:  input.Attachments,
			BaseURL:      web.BaseURL(c),
			SubmissionID: input.SubmissionID,
		}
		if err := bus.Dispatch(c, update); err != nil {
			return c.Failure(err)
		}

		return CommentResponse(c, update.Discussion, update.Result)
	}
}

func DeleteDiscussionComment() web.HandlerFunc {
	return func(c *web.Context) error {
		id, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		operation := &cmd.DeleteComment{CommentID: id}
		if err := bus.Dispatch(c, operation); err != nil {
			return c.Failure(err)
		}

		return CommentResponse(c, operation.Discussion, operation.Result)
	}
}

func ReactToDiscussionComment() web.HandlerFunc {
	return func(c *web.Context) error {
		id, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		emoji := c.Param("reaction")
		switch emoji {
		case "👍", "👎", "❤️", "🤔", "👏", "😂", "😲":
		default:
			return c.BadRequest(web.Map{"message": "Invalid reaction."})
		}

		var input struct {
			Active *bool `json:"active"`
		}
		if err := c.Bind(&input); err != nil || input.Active == nil {
			return c.BadRequest(web.Map{"message": "Choose whether to add or remove the reaction."})
		}

		operation := &cmd.SetCommentReaction{
			CommentID: id,
			Emoji:     emoji,
			Active:    *input.Active,
		}
		if err := bus.Dispatch(c, operation); err != nil {
			return c.Failure(err)
		}

		comment := operation.Result.ForViewer(c.User(), operation.Discussion, c.Tenant(), time.Now())
		return c.Ok(web.Map{"reactionCounts": comment.ReactionCounts})
	}
}
