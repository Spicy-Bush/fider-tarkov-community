package query

import "github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"

type CanUseStoredImage struct {
	Key          string
	MaxKilobytes int
	Result       bool
}

type ListBlobs struct {
	Prefix string

	Result  []string
	Skipped int64
}

type GetBlobByKey struct {
	Key                    string
	AllowUnpublishedAvatar bool
	ForBackup              bool
	MaxBytes               int64

	Result *dto.Blob
}

type IsAvatarPublished struct {
	Key    string
	Result bool
}
