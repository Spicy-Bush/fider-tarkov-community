package cmd

import "github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"

type UploadImageFile struct {
	Name        string
	Content     []byte
	ContentType string
	FileName    string
	Prefix      string

	Result *dto.FileInfo
}

type RenameImageFile struct {
	BlobKey string
	Name    string

	Result *dto.FileInfo
}

type DeleteImageFile struct {
	BlobKey string
}

type UpdateImageFileReferences struct {
	OldBlobKey string
	NewBlobKey string
}

type DeleteImageFileReferences struct {
	BlobKey string
}
