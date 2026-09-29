package dto

import (
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/imagic"
)

const MaxFilePageSize = 100
const MaxFilePage = 1000000000

type FileFilters struct {
	Page           int    `json:"page"`
	PageSize       int    `json:"pageSize"`
	Search         string `json:"search"`
	SortBy         string `json:"sortBy"`
	SortDir        string `json:"sortDir"`
	Type           string `json:"type"`
	Usage          string `json:"usage"`
	IncludeDeleted bool   `json:"includeDeleted"`
	IncludeDrafts  bool   `json:"includeDrafts"`
}

func DefaultFileFilters() FileFilters {
	return FileFilters{Page: 1, PageSize: 20, Type: "all", Usage: "all", SortBy: "createdAt", SortDir: "desc"}
}

type FileOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type FileLibraryOptions struct {
	Defaults      FileFilters  `json:"defaults"`
	Types         []FileOption `json:"types"`
	Usage         []FileOption `json:"usage"`
	Sort          []FileOption `json:"sort"`
	PageSizes     []int        `json:"pageSizes"`
	MaxPage       int          `json:"maxPage"`
	MaxPageSize   int          `json:"maxPageSize"`
	MaxImageBytes int          `json:"maxImageBytes"`
}

func MediaLibraryOptions() FileLibraryOptions {
	return FileLibraryOptions{
		Defaults: DefaultFileFilters(),
		Types: []FileOption{
			{Value: "all", Label: "All types"},
			{Value: "files", Label: "Admin uploads"},
			{Value: "attachments", Label: "Attachments and public images"},
			{Value: "avatars", Label: "Avatars"},
			{Value: "logos", Label: "Logos"},
			{Value: "pages", Label: "Page images"},
			{Value: "oauth", Label: "Authentication images"},
		},
		Usage: []FileOption{
			{Value: "all", Label: "All usage"},
			{Value: "used", Label: "In use"},
			{Value: "unused", Label: "Unused"},
		},
		Sort: []FileOption{
			{Value: "createdAt", Label: "Date uploaded"},
			{Value: "name", Label: "Name"},
			{Value: "size", Label: "Size"},
		},
		PageSizes:     []int{20, 50, MaxFilePageSize},
		MaxPage:       MaxFilePage,
		MaxPageSize:   MaxFilePageSize,
		MaxImageBytes: imagic.MaxImageBytes,
	}
}

type FileInfo struct {
	Name                   string    `json:"name" db:"name"`
	BlobKey                string    `json:"blobKey" db:"key"`
	Size                   int64     `json:"size" db:"size"`
	ContentType            string    `json:"contentType" db:"content_type"`
	CreatedAt              time.Time `json:"createdAt" db:"created_at"`
	IsInUse                bool      `json:"isInUse" db:"is_in_use"`
	HasProtectedReferences bool      `json:"hasProtectedReferences" db:"has_protected_references"`
	Width                  int       `json:"width" db:"width"`
	Height                 int       `json:"height" db:"height"`
	URL                    string    `json:"url"`
	ThumbnailURL           string    `json:"thumbnailURL"`
	State                  string    `json:"state" db:"state"`
	LastError              string    `json:"lastError,omitempty" db:"last_error"`
}

type FileReference struct {
	Kind  string `json:"kind" db:"kind"`
	ID    int    `json:"id" db:"id"`
	Title string `json:"title" db:"title"`
	URL   string `json:"url" db:"url"`
	Field string `json:"-" db:"field"`
	Scope string `json:"scope" db:"scope"`
}

type FileFailure struct {
	BlobKey string `json:"blobKey"`
	Message string `json:"message"`
}

type FileDeletion struct {
	Deleted    []string      `json:"deleted"`
	Pending    []string      `json:"pending"`
	Skipped    []string      `json:"skipped"`
	Errors     []FileFailure `json:"errors"`
	NextCursor string        `json:"nextCursor,omitempty"`
}
