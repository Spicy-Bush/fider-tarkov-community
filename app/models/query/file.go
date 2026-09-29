package query

import (
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
)

type ListImageFiles struct {
	dto.FileFilters

	Result     []*dto.FileInfo
	Total      int
	TotalPages int
	TotalBytes int64
	ListedAt   time.Time
}

func NewListImageFiles() *ListImageFiles {
	return &ListImageFiles{FileFilters: dto.DefaultFileFilters()}
}

type GetFileUsage struct {
	BlobKey  string
	Page     int
	PageSize int

	Result     []*dto.FileReference
	Total      int
	TotalPages int
}

type GetMediaFile struct {
	BlobKey string
	Result  *dto.FileInfo
}
