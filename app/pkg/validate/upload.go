package validate

import (
	"context"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/i18n"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/imagic"
)

// MultiImageUploadOpts arguments to validate mulitple image upload process
type MultiImageUploadOpts struct {
	MaxUploads   int
	IsRequired   bool
	MinWidth     int
	MinHeight    int
	ExactRatio   bool
	MaxKilobytes int
}

// ImageUploadOpts arguments to validate given upload
type ImageUploadOpts struct {
	IsRequired   bool
	MinWidth     int
	MinHeight    int
	ExactRatio   bool
	MaxKilobytes int
}

// MultiImageUpload validates multiple image uploads
func MultiImageUpload(ctx context.Context, currentAttachments []string, uploads []*dto.ImageUpload, opts MultiImageUploadOpts) ([]string, error) {
	remaining := make(map[string]bool, len(currentAttachments))
	for _, key := range currentAttachments {
		remaining[key] = true
	}

	newImages := 0

	for _, upload := range uploads {
		if upload == nil {
			return []string{"Invalid attachment."}, nil
		}

		if upload.Remove {
			delete(remaining, upload.BlobKey)
		} else if upload.Upload != nil {
			newImages++
		}

		messages, err := ImageUpload(ctx, upload, ImageUploadOpts{
			IsRequired:   opts.IsRequired,
			MinWidth:     opts.MinWidth,
			MinHeight:    opts.MinHeight,
			ExactRatio:   opts.ExactRatio,
			MaxKilobytes: opts.MaxKilobytes,
		})
		if err != nil {
			return nil, err
		}
		if len(messages) > 0 {
			return messages, nil
		}
	}

	if len(remaining)+newImages > opts.MaxUploads {
		return []string{i18n.T(ctx, "validation.custom.maxattachments", i18n.Params{"number": opts.MaxUploads})}, nil
	}

	return []string{}, nil
}

// ImageUpload validates given image upload
func ImageUpload(ctx context.Context, upload *dto.ImageUpload, opts ImageUploadOpts) ([]string, error) {
	messages := []string{}

	if opts.IsRequired {
		if upload == nil || (upload.BlobKey == "" && upload.Upload == nil) || upload.Remove {
			messages = append(messages, i18n.T(ctx, "validation.required",
				i18n.Params{"name": i18n.T(ctx, "property.image")},
			))
		}
	}

	if upload != nil && upload.Upload != nil && len(upload.Upload.Content) > 0 {
		if len(upload.Upload.Content) > opts.MaxKilobytes*1024 {
			return []string{i18n.T(ctx, "validation.custom.maximagesize",
				i18n.Params{"kilobytes": opts.MaxKilobytes},
			)}, nil
		}

		logo, err := imagic.Parse(upload.Upload.Content)
		if err != nil {
			if err == imagic.ErrNotSupported {
				messages = append(messages, i18n.T(ctx, "validation.custom.unsupportedfileformat"))
			} else if err == imagic.ErrTooLarge || err == imagic.ErrTooManyBytes {
				messages = append(messages, err.Error())
			} else {
				return nil, err
			}
		} else {

			if logo.Width < opts.MinWidth || logo.Height < opts.MinHeight {
				messages = append(messages, i18n.T(ctx, "validation.custom.minimagedimensions",
					i18n.Params{"width": opts.MinWidth, "height": opts.MinHeight},
				))
			}

			if opts.ExactRatio && logo.Width != logo.Height {
				messages = append(messages, i18n.T(ctx, "validation.custom.imagesquareratio"))
			}
		}
	}

	return messages, nil
}
