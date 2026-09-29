package postgres

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/imagic"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/rand"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

func imagesNeedPreparation(images []*dto.ImageUpload) bool {
	for _, image := range images {
		if image != nil && !image.Remove && image.Upload != nil && image.Upload.Prepared == nil {
			return true
		}
	}
	return false
}

func prepareImages(ctx context.Context, images []*dto.ImageUpload, folder string) error {
	return prepareImagesWithKeys(ctx, images, func(index int) string {
		return fmt.Sprintf("%s/%s.webp", folder, rand.String(32))
	})
}

func prepareSubmissionImages(ctx context.Context, images []*dto.ImageUpload, identity string) error {
	return prepareImagesWithKeys(ctx, images, func(index int) string {
		digest := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", identity, index)))
		return fmt.Sprintf("attachments/%x.webp", digest)
	})
}

func prepareImagesWithKeys(ctx context.Context, images []*dto.ImageUpload, keyAt func(int) string) error {
	if !imagesNeedPreparation(images) {
		return nil
	}
	if ctx.Value(app.TransactionCtxKey) != nil {
		return fmt.Errorf("image preparation requires a context without a transaction")
	}

	for index, image := range images {
		if image == nil || image.Remove || image.Upload == nil || image.Upload.Prepared != nil {
			continue
		}
		if err := validate.ImageUploadMetadata(image.Upload); err != nil {
			return err
		}

		prepared, err := prepareInlineImage(ctx, image.Upload.Content, keyAt(index))
		if err != nil {
			return err
		}
		image.Upload.Prepared = prepared
	}
	return nil
}

func prepareInlineImage(ctx context.Context, input []byte, key string) (*dto.PreparedImage, error) {
	source, format, err := imagic.Decode(input)
	if err != nil {
		return nil, validate.Failed(err.Error())
	}
	if imageNeedsResize(source.Bounds().Dx(), source.Bounds().Dy()) {
		source = imagic.Resize(maxImageDimension)(source, format)
	}
	content, err := imagic.EncodeWebP(source)
	if err != nil {
		return nil, err
	}
	prepared, err := prepareMediaImage(source, key, "image/webp", content)
	if err != nil {
		return nil, err
	}
	if err := storePreparedImage(ctx, prepared); err != nil {
		return nil, err
	}
	return prepared, nil
}
