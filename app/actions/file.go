package actions

import (
	"context"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

type UploadNewFile struct {
	Name       string           `json:"name"`
	File       *dto.ImageUpload `json:"file"`
	UploadType string           `json:"uploadType"`
}

func NewUploadNewFile() *UploadNewFile {
	return &UploadNewFile{
		UploadType: "file",
	}
}

func (action *UploadNewFile) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil && user.IsAdministrator()
}

func (action *UploadNewFile) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	if action.Name == "" {
		result.AddFieldFailure("name", "Name is required")
	}

	if action.File == nil || action.File.Upload == nil {
		result.AddFieldFailure("file", "File is required")
	}

	if action.UploadType != "file" && action.UploadType != "attachment" {
		result.AddFieldFailure("uploadType", "Upload type must be 'file' or 'attachment'")
	}

	if action.File != nil && action.File.Upload != nil {
		fileUploadOpts := validate.ImageUploadOpts{
			IsRequired:   true,
			MaxKilobytes: 50000,
		}

		if messages, err := validate.ImageUpload(ctx, action.File, fileUploadOpts); err != nil {
			return validate.Error(err)
		} else if len(messages) > 0 {
			result.AddFieldFailure("file", messages...)
		}
	}

	return result
}

type RenameFile struct {
	BlobKey string `json:"-"`
	Name    string `json:"name"`
}

func NewRenameFile() *RenameFile {
	return &RenameFile{}
}

func (action *RenameFile) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil && user.IsAdministrator()
}

func (action *RenameFile) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	if action.Name == "" {
		result.AddFieldFailure("name", "Name is required")
	}

	if action.BlobKey == "" {
		result.AddFieldFailure("blobKey", "BlobKey is required")
	}

	return result
}
