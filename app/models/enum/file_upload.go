package enum

import "fmt"

type FileUploadType string

const (
	FileUploadPrivate FileUploadType = "file"
	FileUploadPublic  FileUploadType = "attachment"
)

func (kind FileUploadType) IsValid() bool {
	return kind == FileUploadPrivate || kind == FileUploadPublic
}

func (kind FileUploadType) Prefix() string {
	switch kind {
	case FileUploadPrivate:
		return "files/"
	case FileUploadPublic:
		return "attachments/"
	default:
		panic(fmt.Sprintf("File upload type %q was not validated", kind))
	}
}
