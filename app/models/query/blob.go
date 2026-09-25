package query

import "github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"

type ListBlobs struct {
	Prefix string

	Result []string
}

type GetBlobByKey struct {
	Key                    string
	AllowUnpublishedAvatar bool

	Result *dto.Blob
}

type IsAvatarPublished struct {
	Key    string
	Result bool
}