package query

import "github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"

type GetMediaThumbnail struct {
	Key                    string
	Size                   int
	AllowUnpublishedAvatar bool

	Result *dto.Blob
}
