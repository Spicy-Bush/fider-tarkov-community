package query

import "github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"

type ListImageFiles struct {
	Page     int
	PageSize int
	Search   string
	SortBy   string
	SortDir  string

	Result     []*dto.FileInfo
	Total      int
	TotalPages int
}

type IsImageFileInUse struct {
	BlobKey string

	Result bool
	UsedIn []string
}

type GetImageFile struct {
	BlobKey string

	Result *dto.FileInfo
}

type GetNameFromBlobKey struct {
	BlobKey string
	Result  string
}
