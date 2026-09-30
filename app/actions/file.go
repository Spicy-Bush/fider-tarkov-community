package actions

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/imagic"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/rand"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/blob"
)

func ValidFileType(value string) bool {
	return fileOptionExists(dto.MediaLibraryOptions().Types, value)
}

func fileOptionExists(options []dto.FileOption, value string) bool {
	return slices.ContainsFunc(options, func(option dto.FileOption) bool { return option.Value == value })
}

func ParseFileListQuery(values url.Values) (*query.ListImageFiles, error) {
	q := query.NewListImageFiles()
	options := dto.MediaLibraryOptions()
	for _, field := range []struct {
		name    string
		value   *int
		maximum int
	}{
		{"page", &q.Page, options.MaxPage},
		{"pageSize", &q.PageSize, options.MaxPageSize},
	} {
		if raw := values.Get(field.name); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 1 || value > field.maximum {
				return nil, validate.Failed("Choose a valid " + field.name + ".")
			}
			*field.value = value
		}
	}

	for _, field := range []struct {
		name    string
		value   *string
		options []dto.FileOption
	}{
		{"type", &q.Type, options.Types},
		{"usage", &q.Usage, options.Usage},
		{"sortBy", &q.SortBy, options.Sort},
	} {
		if value := values.Get(field.name); value != "" {
			if !fileOptionExists(field.options, value) {
				return nil, validate.Failed("Choose a valid " + field.name + ".")
			}
			*field.value = value
		}
	}

	if direction := values.Get("sortDir"); direction != "" {
		if direction != "asc" && direction != "desc" {
			return nil, validate.Failed("Choose a valid file order.")
		}
		q.SortDir = direction
	}
	q.Search = values.Get("search")
	q.IncludeDeleted = values.Get("includeDeleted") == "true"
	q.IncludeDrafts = values.Get("includeDrafts") == "true"
	return q, nil
}

type UploadNewFile struct {
	SubmissionID string           `json:"submissionId"`
	Name         string           `json:"name"`
	File         *dto.ImageUpload `json:"file"`
	UploadType   string           `json:"uploadType"`
}

func NewUploadNewFile() *UploadNewFile {
	return &UploadNewFile{
		UploadType: string(enum.FileUploadPrivate),
	}
}

func NewFileUploadID(tenantID, userID int) string {
	nonce := rand.String(32)
	return nonce + "." + base64.RawURLEncoding.EncodeToString(fileUploadSignature(tenantID, userID, nonce))
}

func validFileUploadID(value string, tenantID, userID int) bool {
	if len(value) != 76 || value[32] != '.' {
		return false
	}

	signature, err := base64.RawURLEncoding.DecodeString(value[33:])
	return err == nil && hmac.Equal(signature, fileUploadSignature(tenantID, userID, value[:32]))
}

func fileUploadSignature(tenantID, userID int, nonce string) []byte {
	mac := hmac.New(sha256.New, []byte(env.Config.JWTSecret))
	fmt.Fprintf(mac, "file-upload:%d:%d:%s", tenantID, userID, nonce)
	return mac.Sum(nil)
}

func (action *UploadNewFile) IsAuthorized(ctx context.Context, user *entity.User) bool {
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	return entity.Can(user, tenant, entity.ManageFiles)
}

func (action *UploadNewFile) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	action.Name = strings.TrimSpace(action.Name)
	if action.Name == "" || len(action.Name) > 255 {
		result.AddFieldFailure("name", "Use a name between 1 and 255 characters")
	}

	tenant := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	if !validFileUploadID(action.SubmissionID, tenant.ID, user.ID) {
		result.AddFieldFailure("submissionId", "Start a new image upload")
	}

	if action.File == nil || action.File.Upload == nil {
		result.AddFieldFailure("file", "File is required")
	}

	if !enum.FileUploadType(action.UploadType).IsValid() {
		result.AddFieldFailure("uploadType", "Upload type must be 'file' or 'attachment'")
	}

	if action.File != nil && action.File.Upload != nil {
		fileUploadOpts := validate.ImageUploadOpts{
			IsRequired:   true,
			MaxKilobytes: imagic.MaxImageKilobytes,
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
	BlobKey string `json:"blobKey"`
	Name    string `json:"name"`
}

func NewRenameFile() *RenameFile {
	return &RenameFile{}
}

func (action *RenameFile) IsAuthorized(ctx context.Context, user *entity.User) bool {
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	return entity.Can(user, tenant, entity.ManageFiles)
}

func (action *RenameFile) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	action.Name = strings.TrimSpace(action.Name)
	if action.Name == "" || len(action.Name) > 255 {
		result.AddFieldFailure("name", "Use a name between 1 and 255 characters")
	}

	if err := blob.ValidateKey(action.BlobKey); err != nil {
		result.AddFieldFailure("blobKey", "Choose a valid file")
	}

	return result
}

type BulkDeleteFiles struct {
	BlobKeys       []string `json:"blobKeys"`
	Force          bool     `json:"force"`
	IncludeDeleted bool     `json:"includeDeleted"`
	IncludeDrafts  bool     `json:"includeDrafts"`
}

func NewBulkDeleteFiles() *BulkDeleteFiles {
	return &BulkDeleteFiles{}
}

func (action *BulkDeleteFiles) IsAuthorized(ctx context.Context, user *entity.User) bool {
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	return entity.Can(user, tenant, entity.ManageFiles)
}

func (action *BulkDeleteFiles) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	if len(action.BlobKeys) == 0 {
		result.AddFieldFailure("blobKeys", "At least one blob key is required")
	}

	if len(action.BlobKeys) > 100 {
		result.AddFieldFailure("blobKeys", "Cannot delete more than 100 files at once")
	}

	for _, key := range action.BlobKeys {
		if err := blob.ValidateKey(key); err != nil {
			result.AddFieldFailure("blobKeys", "Choose valid files")
			break
		}
	}

	return result
}
