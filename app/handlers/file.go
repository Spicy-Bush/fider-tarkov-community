package handlers

import (
	"fmt"
	"net/http"

	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func FileManagementPage() web.HandlerFunc {
	return func(c *web.Context) error {
		return c.Page(http.StatusOK, web.Props{
			Page:  "Administration/pages/FileManagement.page",
			Title: "Files · Site Settings",
			Data:  web.Map{},
		})
	}
}

func ListFiles() web.HandlerFunc {
	return func(c *web.Context) error {
		page, err := c.QueryParamAsInt("page")
		if err != nil {
			page = 1
		}
		pageSize, err := c.QueryParamAsInt("pageSize")
		if err != nil {
			pageSize = 20
		}
		if pageSize > 100 {
			pageSize = 100
		}

		search := c.QueryParam("search")
		sortBy := c.QueryParam("sortBy")
		sortDir := c.QueryParam("sortDir")

		if sortBy == "" {
			sortBy = "createdAt"
		}
		if sortDir == "" {
			sortDir = "desc"
		}

		listFilesQuery := &query.ListImageFiles{
			Page:     page,
			PageSize: pageSize,
			Search:   search,
			SortBy:   sortBy,
			SortDir:  sortDir,
		}
		if err := bus.Dispatch(c, listFilesQuery); err != nil {
			return c.Failure(err)
		}

		return c.Ok(map[string]interface{}{
			"files":      listFilesQuery.Result,
			"total":      listFilesQuery.Total,
			"page":       listFilesQuery.Page,
			"pageSize":   listFilesQuery.PageSize,
			"totalPages": listFilesQuery.TotalPages,
		})
	}
}

func UploadFile() web.HandlerFunc {
	return func(c *web.Context) error {
		action := actions.NewUploadNewFile()
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		prefix := "files/"
		if action.UploadType == "attachment" {
			prefix = "attachments/"
		}

		return c.WithTransaction(func() error {
			uploadCmd := &cmd.UploadImageFile{
				Name:        action.Name,
				Content:     action.File.Upload.Content,
				ContentType: action.File.Upload.ContentType,
				FileName:    action.File.Upload.FileName,
				Prefix:      prefix,
			}

			if err := bus.Dispatch(c, uploadCmd); err != nil {
				return c.Failure(err)
			}

			return c.Ok(uploadCmd.Result)
		})
	}
}

func RenameFile() web.HandlerFunc {
	return func(c *web.Context) error {
		blobKey := c.Param("blobKey")
		if extraPath := c.Param("path"); extraPath != "" {
			blobKey = fmt.Sprintf("%s/%s", blobKey, extraPath)
		}

		action := actions.NewRenameFile()
		action.BlobKey = blobKey

		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		return c.WithTransaction(func() error {
			renameCmd := &cmd.RenameImageFile{
				BlobKey: blobKey,
				Name:    action.Name,
			}

			if err := bus.Dispatch(c, renameCmd); err != nil {
				return c.Failure(err)
			}

			return c.Ok(renameCmd.Result)
		})
	}
}

func DeleteFile() web.HandlerFunc {
	return func(c *web.Context) error {
		blobKey := c.Param("blobKey")

		if extraPath := c.Param("path"); extraPath != "" {
			blobKey = fmt.Sprintf("%s/%s", blobKey, extraPath)
		}

		usageQuery := &query.IsImageFileInUse{BlobKey: blobKey}
		if err := bus.Dispatch(c, usageQuery); err != nil {
			return c.Failure(err)
		}

		forceDelete := c.QueryParam("force") == "true"

		if usageQuery.Result && !forceDelete {
			return c.BadRequest(web.Map{
				"message": "Cannot delete an image that is in use. Use force delete to remove it and all references.",
			})
		}

		return c.WithTransaction(func() error {
			if forceDelete && usageQuery.Result {
				deleteRefsCmd := &cmd.DeleteImageFileReferences{BlobKey: blobKey}
				if err := bus.Dispatch(c, deleteRefsCmd); err != nil {
					return c.Failure(err)
				}
			}

			deleteCmd := &cmd.DeleteImageFile{BlobKey: blobKey}
			if err := bus.Dispatch(c, deleteCmd); err != nil {
				return c.Failure(err)
			}

			return c.Ok(web.Map{})
		})
	}
}

func GetFileUsage() web.HandlerFunc {
	return func(c *web.Context) error {
		blobKey := c.Param("blobKey")

		if extraPath := c.Param("path"); extraPath != "" {
			blobKey = fmt.Sprintf("%s/%s", blobKey, extraPath)
		}

		usageQuery := &query.IsImageFileInUse{BlobKey: blobKey}
		if err := bus.Dispatch(c, usageQuery); err != nil {
			return c.Failure(err)
		}

		if !usageQuery.Result {
			return c.Ok([]string{})
		}

		return c.Ok(usageQuery.UsedIn)
	}
}

func BulkDeleteFiles() web.HandlerFunc {
	return func(c *web.Context) error {
		action := actions.NewBulkDeleteFiles()
		if result := c.BindTo(action); !result.Ok {
			return c.HandleValidation(result)
		}

		forceDelete := c.QueryParam("force") == "true"

		return c.WithTransaction(func() error {
			deleted := 0
			skipped := 0
			errors := []string{}

			for _, blobKey := range action.BlobKeys {
				usageQuery := &query.IsImageFileInUse{BlobKey: blobKey}
				if err := bus.Dispatch(c, usageQuery); err != nil {
					errors = append(errors, fmt.Sprintf("%s: %v", blobKey, err))
					continue
				}

				if usageQuery.Result && !forceDelete {
					skipped++
					continue
				}

				if forceDelete && usageQuery.Result {
					deleteRefsCmd := &cmd.DeleteImageFileReferences{BlobKey: blobKey}
					if err := bus.Dispatch(c, deleteRefsCmd); err != nil {
						errors = append(errors, fmt.Sprintf("%s: %v", blobKey, err))
						continue
					}
				}

				deleteCmd := &cmd.DeleteImageFile{BlobKey: blobKey}
				if err := bus.Dispatch(c, deleteCmd); err != nil {
					errors = append(errors, fmt.Sprintf("%s: %v", blobKey, err))
					continue
				}

				deleted++
			}

			return c.Ok(web.Map{
				"deleted": deleted,
				"skipped": skipped,
				"errors":  errors,
			})
		})
	}
}

func PruneUnusedFiles() web.HandlerFunc {
	return func(c *web.Context) error {
		prunableQuery := &query.GetPrunableFiles{}
		if err := bus.Dispatch(c, prunableQuery); err != nil {
			return c.Failure(err)
		}

		if len(prunableQuery.Result) == 0 {
			return c.Ok(web.Map{
				"deleted": 0,
				"message": "No unused files to prune",
			})
		}

		return c.WithTransaction(func() error {
			deleted := 0
			errors := []string{}

			for _, blobKey := range prunableQuery.Result {
				deleteCmd := &cmd.DeleteImageFile{BlobKey: blobKey}
				if err := bus.Dispatch(c, deleteCmd); err != nil {
					errors = append(errors, fmt.Sprintf("%s: %v", blobKey, err))
					continue
				}
				deleted++
			}

			return c.Ok(web.Map{
				"deleted": deleted,
				"errors":  errors,
			})
		})
	}
}

func GetPrunableFilesCount() web.HandlerFunc {
	return func(c *web.Context) error {
		prunableQuery := &query.GetPrunableFiles{}
		if err := bus.Dispatch(c, prunableQuery); err != nil {
			return c.Failure(err)
		}

		return c.Ok(web.Map{
			"count": len(prunableQuery.Result),
		})
	}
}
