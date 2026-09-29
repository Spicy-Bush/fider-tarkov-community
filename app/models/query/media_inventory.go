package query

import "github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"

type ScanBlobMetadata struct {
	Cursor    string
	BatchSize int
	Skipped   int64
	Accept    func(files []dto.BlobMetadata, nextCursor string, complete bool) error
}

type GetMediaInventory struct {
	Result *dto.MediaInventory
}
