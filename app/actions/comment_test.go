package actions_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
)

func TestCommentInputRejectsContradictoryAttachments(t *testing.T) {
	image, err := os.ReadFile(env.Path("/app/pkg/web/testdata/logo1.png"))
	if err != nil {
		t.Fatal(err)
	}
	bus.AddHandler(func(context.Context, *query.GetTenantProfanityWords) error {
		return nil
	})

	ctx := createTestContext()
	user := &entity.User{ID: 1, Role: enum.RoleVisitor, Status: enum.UserActive}
	discussion := entity.PostDiscussion(&entity.Post{ID: 1, Status: enum.PostOpen})
	comment := &entity.Comment{
		ID: 1,
		User: user,
		CreatedAt: time.Now(),
		Attachments: []string{"attachments/owned.webp"},
	}
	cases := []struct {
		name string
		attachment *dto.ImageUpload
	}{
		{
			name: "upload and removal",
			attachment: &dto.ImageUpload{
				Remove: true,
				Upload: &dto.ImageUploadData{Content: image},
			},
		},
		{
			name: "upload and existing key",
			attachment: &dto.ImageUpload{
				BlobKey: comment.Attachments[0],
				Upload: &dto.ImageUploadData{Content: image},
			},
		},
		{
			name: "upload and foreign key removal",
			attachment: &dto.ImageUpload{
				BlobKey: "attachments/foreign.webp",
				Remove: true,
				Upload: &dto.ImageUploadData{Content: image},
			},
		},
	}

	for _, candidate := range cases {
		t.Run(candidate.name, func(t *testing.T) {
			input := &actions.CommentInput{
				Discussion: discussion,
				Comment: comment,
				Content: "Edited comment",
				Attachments: []*dto.ImageUpload{candidate.attachment},
			}
			result := input.Validate(ctx, user)
			if result.Ok || result.Err != nil || !result.Authorized {
				t.Errorf("expected attachment validation failure: ok=%v authorized=%v err=%v errors=%v", result.Ok, result.Authorized, result.Err, result.Errors)
			}

			input.Attachments = []*dto.ImageUpload{
				{BlobKey: comment.Attachments[0], Remove: true},
				{Upload: &dto.ImageUploadData{Content: image}},
			}
			if recovered := input.Validate(ctx, user); !recovered.Ok {
				t.Fatalf("valid replacement failed: err=%v errors=%v", recovered.Err, recovered.Errors)
			}
		})
	}
}
