package handlers

import (
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/imagic"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/blob"
)

func FileManagementPage() web.HandlerFunc {
	return func(c *web.Context) error {
		return c.Page(http.StatusOK, web.Props{
			Page:  "Administration/pages/FileManagement.page",
			Title: "Files · Site Settings",
			Data:  web.Map{"options": dto.MediaLibraryOptions()},
		})
	}
}

func filePage(c *web.Context, name string, defaultValue, maximum int) (int, error) {
	value := c.QueryParam(name)
	if value == "" {
		return defaultValue, nil
	}

	page, err := strconv.Atoi(value)
	if err != nil || page < 1 || page > maximum {
		return 0, validate.Failed("Choose a valid " + name + ".")
	}
	return page, nil
}

func ListFiles() web.HandlerFunc {
	return func(c *web.Context) error {
		q, err := actions.ParseFileListQuery(c.Request.URL.Query())
		if err != nil {
			return c.Failure(err)
		}

		if err := bus.Dispatch(c, q); err != nil {
			return c.Failure(err)
		}

		inventory := &query.GetMediaInventory{}
		if err := bus.Dispatch(c, inventory); err != nil {
			return c.Failure(err)
		}

		return c.Ok(web.Map{
			"files":      q.Result,
			"total":      q.Total,
			"totalBytes": q.TotalBytes,
			"page":       q.Page,
			"pageSize":   q.PageSize,
			"totalPages": q.TotalPages,
			"inventory":  inventory.Result,
			"listedAt":   q.ListedAt,
		})
	}
}

func RefreshFileInventory() web.HandlerFunc {
	return func(c *web.Context) error {
		if err := bus.Dispatch(c, &cmd.RefreshMediaInventory{}); err != nil {
			return c.Failure(err)
		}
		return c.Ok(web.Map{})
	}
}

func UploadFile() web.HandlerFunc {
	return func(c *web.Context) error {
		action := actions.NewUploadNewFile()
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		upload := &cmd.UploadImageFile{
			SubmissionID: action.SubmissionID,
			Name:         action.Name,
			Type:         enum.FileUploadType(action.UploadType),
			Content:      action.File.Upload.Content,
		}
		if err := bus.Dispatch(c, upload); err != nil {
			return c.Failure(err)
		}
		return c.Ok(upload.Result)
	}
}

func NewFileUploadID() web.HandlerFunc {
	return func(c *web.Context) error {
		if !entity.Can(c.User(), c.Tenant(), entity.ManageFiles) && !entity.Can(c.User(), c.Tenant(), entity.ManageSponsorship) {
			return c.Forbidden()
		}

		return c.Ok(actions.NewFileUploadID(c.Tenant().ID, c.User().ID))
	}
}

func RenameFile() web.HandlerFunc {
	return func(c *web.Context) error {
		action := actions.NewRenameFile()
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		rename := &cmd.RenameImageFile{BlobKey: action.BlobKey, Name: action.Name}
		if err := bus.Dispatch(c, rename); err != nil {
			return c.Failure(err)
		}
		return c.Ok(rename.Result)
	}
}

func GetFileUsage() web.HandlerFunc {
	return func(c *web.Context) error {
		key := c.QueryParam("key")
		if err := blob.ValidateKey(key); err != nil {
			return c.Failure(validate.Failed("Choose a valid file."))
		}
		page, err := filePage(c, "page", 1, 1000000000)
		if err != nil {
			return c.Failure(err)
		}

		usage := &query.GetFileUsage{BlobKey: key, Page: page}
		if err := bus.Dispatch(c, usage); err != nil {
			return c.Failure(err)
		}
		return c.Ok(web.Map{
			"items":      usage.Result,
			"total":      usage.Total,
			"page":       usage.Page,
			"pageSize":   usage.PageSize,
			"totalPages": usage.TotalPages,
		})
	}
}

func BulkDeleteFiles() web.HandlerFunc {
	return func(c *web.Context) error {
		action := actions.NewBulkDeleteFiles()
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		deletion := &cmd.DeleteFiles{
			BlobKeys: action.BlobKeys, Force: action.Force,
			IncludeDeleted: action.IncludeDeleted, IncludeDrafts: action.IncludeDrafts,
		}
		if err := bus.Dispatch(c, deletion); err != nil {
			return c.Failure(err)
		}
		return c.Ok(deletion.Result)
	}
}

func PruneUnusedFiles() web.HandlerFunc {
	return func(c *web.Context) error {
		var body struct {
			Search         string    `json:"search"`
			Type           string    `json:"type"`
			Before         time.Time `json:"before"`
			Cursor         string    `json:"cursor"`
			IncludeDeleted bool      `json:"includeDeleted"`
			IncludeDrafts  bool      `json:"includeDrafts"`
		}
		if err := c.Bind(&body); err != nil {
			return c.Failure(validate.Failed("Choose the files to clean up."))
		}
		if !actions.ValidFileType(body.Type) || body.Before.IsZero() || body.Before.After(time.Now().Add(time.Minute)) {
			return c.Failure(validate.Failed("Choose a valid cleanup filter and cutoff."))
		}
		if body.Cursor != "" && blob.ValidateKey(body.Cursor) != nil {
			return c.Failure(validate.Failed("Choose a valid cleanup cursor."))
		}

		prune := &cmd.PruneFiles{
			Search:         body.Search,
			Type:           body.Type,
			Before:         body.Before,
			Cursor:         body.Cursor,
			IncludeDeleted: body.IncludeDeleted,
			IncludeDrafts:  body.IncludeDrafts,
		}
		if err := bus.Dispatch(c, prune); err != nil {
			return c.Failure(err)
		}
		return c.Ok(prune.Result)
	}
}

func DownloadFile() web.HandlerFunc {
	return func(c *web.Context) error {
		key := c.QueryParam("key")
		if err := blob.ValidateKey(key); err != nil {
			return c.Failure(validate.Failed("Choose a valid file."))
		}

		file := &query.GetMediaFile{BlobKey: key}
		if err := bus.Dispatch(c, file); err != nil {
			return c.Failure(err)
		}
		if file.Result.State != "ready" {
			return c.Failure(blob.ErrNotFound)
		}

		content := &query.GetBlobByKey{Key: key, AllowUnpublishedAvatar: true, MaxBytes: imagic.MaxImageBytes}
		if err := bus.Dispatch(c, content); err != nil {
			return c.Failure(err)
		}
		disposition := "attachment"
		if c.QueryParam("inline") == "true" {
			disposition = "inline"
		}
		c.Response.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": file.Result.Name}))
		c.Response.Header().Set("Cache-Control", "private, no-store")
		c.Response.Header().Set("X-Content-Type-Options", "nosniff")
		return c.Blob(http.StatusOK, content.Result.ContentType, content.Result.Content)
	}
}
