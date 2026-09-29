package postgres_test

import (
	"fmt"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
)

func TestMediaUsageLinksToCommentPermalinks(t *testing.T) {
	for _, owner := range []string{"post", "page"} {
		t.Run(owner, func(t *testing.T) {
			f := newPostWorkflow(t)
			f.user.Role = enum.RoleAdministrator

			var postNumber, pageID int
			var ownerURL string
			if owner == "post" {
				post := &cmd.AddNewPost{Title: "Comment images", Description: "A discussion"}
				if err := bus.Dispatch(f.ctx, post); err != nil {
					t.Fatal(err)
				}

				postNumber = post.Result.Number
				ownerURL = fmt.Sprintf("/posts/%d/%s", post.Result.Number, post.Result.Slug)
			} else {
				page := &cmd.CreatePage{
					Title:              "Comment images",
					Slug:               "comment-images",
					Status:             entity.PageStatusPublished,
					Visibility:         entity.PageVisibilityPublic,
					AllowComments:      true,
					AllowCommentImages: true,
				}
				if err := bus.Dispatch(f.ctx, page); err != nil {
					t.Fatal(err)
				}

				pageID = page.Result.ID
				ownerURL = "/pages/" + page.Result.Slug
			}

			comment := &cmd.CreateComment{
				PostNumber:   postNumber,
				PageID:       pageID,
				Content:      "Comment with image",
				SubmissionID: "comment-image-link",
				Attachments:  []*dto.ImageUpload{pngAttachment(t, 4)},
			}
			if err := bus.Dispatch(f.ctx, comment); err != nil {
				t.Fatal(err)
			}
			key := comment.Result.Attachments[0]
			commentID := comment.Result.ID
			edit := &cmd.UpdateComment{
				CommentID:    commentID,
				Content:      "![inline](/static/images/" + key + ")",
				SubmissionID: "comment-image-link-edit",
			}
			if err := bus.Dispatch(f.ctx, edit); err != nil {
				t.Fatal(err)
			}

			usage := &query.GetFileUsage{BlobKey: key}
			if err := bus.Dispatch(f.ctx, usage); err != nil {
				t.Fatal(err)
			}
			if usage.Total != 2 || len(usage.Result) != 2 {
				t.Fatalf("expected inline and attachment references: %+v", usage)
			}

			wantURL := fmt.Sprintf("%s#comment-%d", ownerURL, commentID)
			for _, reference := range usage.Result {
				if reference.URL != wantURL {
					t.Errorf("%s link=%q, expected %q", reference.Kind, reference.URL, wantURL)
				}
			}
		})
	}
}
