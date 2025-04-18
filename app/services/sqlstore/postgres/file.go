package postgres

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/rand"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/blob"
)

func getNameFromBlobKey(ctx context.Context, q *query.GetNameFromBlobKey) error {
	if q.BlobKey == "" {
		q.Result = ""
		return nil
	}

	parts := strings.Split(q.BlobKey, "/")
	if len(parts) == 0 {
		q.Result = q.BlobKey
		return nil
	}

	filename := parts[len(parts)-1]

	if strings.HasPrefix(q.BlobKey, "logos/") || strings.HasPrefix(q.BlobKey, "avatars/") {
		extensionParts := strings.Split(filename, ".")
		if len(extensionParts) > 1 {
			extension := "." + extensionParts[len(extensionParts)-1]

			if strings.HasPrefix(q.BlobKey, "logos/") {
				q.Result = "logo" + extension
			} else {
				q.Result = "avatar" + extension
			}
			return nil
		}

		if strings.HasPrefix(q.BlobKey, "logos/") {
			q.Result = "logo"
		} else {
			q.Result = "avatar"
		}
		return nil
	}

	nameParts := strings.SplitN(filename, "-", 2)
	if len(nameParts) > 1 {
		if len(nameParts[0]) > 10 {
			filename = nameParts[1]
		}
	}

	extensionParts := strings.Split(filename, ".")
	extension := ""
	if len(extensionParts) > 1 {
		extension = "." + extensionParts[len(extensionParts)-1]
		filename = strings.TrimSuffix(filename, extension)
	}

	q.Result = filename + extension
	return nil
}

func isImageFileInUse(ctx context.Context, q *query.IsImageFileInUse) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		usageLocations := []string{}
		q.Result = false

		var usageCount struct {
			LogoCount       int `db:"logo_count"`
			AvatarCount     int `db:"avatar_count"`
			AttachmentCount int `db:"attachment_count"`
		}

		err := trx.Get(&usageCount, `
			SELECT 
				(SELECT COUNT(*) FROM tenants WHERE logo_bkey = $1 AND id = $2) AS logo_count,
				(SELECT COUNT(*) FROM users WHERE avatar_bkey = $1) AS avatar_count,
				(SELECT COUNT(*) FROM attachments WHERE attachment_bkey = $1 AND tenant_id = $2) AS attachment_count
		`, q.BlobKey, tenant.ID)

		if err != nil {
			return errors.Wrap(err, "failed to check file usage")
		}

		if usageCount.LogoCount > 0 {
			usageLocations = append(usageLocations, "Tenant Logo")
			q.Result = true
		}

		if usageCount.AvatarCount > 0 {
			usageLocations = append(usageLocations, "User Avatar")
			q.Result = true
		}

		if usageCount.AttachmentCount > 0 {
			q.Result = true

			rows, err := trx.Query(`
				SELECT 
					COALESCE(p.title, 'Untitled Post') as post_title,
					COALESCE(p.id, c.post_id) as post_id,
					CASE 
						WHEN a.post_id IS NOT NULL THEN 'post'
						WHEN a.comment_id IS NOT NULL THEN 'comment'
						ELSE 'unknown'
					END as attachment_type
				FROM attachments a
				LEFT JOIN posts p ON a.post_id = p.id
				LEFT JOIN comments c ON a.comment_id = c.id
				WHERE a.attachment_bkey = $1 AND a.tenant_id = $2
			`, q.BlobKey, tenant.ID)

			if err != nil {
				return errors.Wrap(err, "failed to fetch detailed attachment usage")
			}
			defer rows.Close()

			for rows.Next() {
				var postTitle string
				var postID int
				var attachmentType string

				if err := rows.Scan(&postTitle, &postID, &attachmentType); err != nil {
					return errors.Wrap(err, "failed to scan attachment usage row")
				}

				if attachmentType == "post" {
					usageLocations = append(usageLocations,
						fmt.Sprintf("Post Attachment: %s (ID: %d)", postTitle, postID))
				} else if attachmentType == "comment" {
					usageLocations = append(usageLocations,
						fmt.Sprintf("Comment Attachment (Post: %s, ID: %d)", postTitle, postID))
				}
			}

			if err := rows.Err(); err != nil {
				return errors.Wrap(err, "error iterating attachment usage rows")
			}
		}

		if len(usageLocations) > 0 {
			q.UsedIn = usageLocations
		}

		return nil
	})
}

func getImageFile(ctx context.Context, q *query.GetImageFile) error {
	blob.EnsureAuthorizedPrefix(ctx, q.BlobKey)

	blobQuery := &query.GetBlobByKey{
		Key: q.BlobKey,
	}
	if err := bus.Dispatch(ctx, blobQuery); err != nil {
		return errors.Wrap(err, "failed to get blob")
	}

	errChan := make(chan error, 2)
	var isInUseQuery query.IsImageFileInUse
	var nameQuery query.GetNameFromBlobKey

	go func() {
		isInUseQuery.BlobKey = q.BlobKey
		errChan <- bus.Dispatch(ctx, &isInUseQuery)
	}()

	go func() {
		nameQuery.BlobKey = q.BlobKey
		errChan <- bus.Dispatch(ctx, &nameQuery)
	}()

	for i := 0; i < 2; i++ {
		if err := <-errChan; err != nil {
			return errors.Wrap(err, "failed to get file info")
		}
	}

	q.Result = &dto.FileInfo{
		Name:        nameQuery.Result,
		BlobKey:     q.BlobKey,
		Size:        blobQuery.Result.Size,
		ContentType: blobQuery.Result.ContentType,
		CreatedAt:   time.Now(), // TODO: add created at to blobs - why was this not added?
		IsInUse:     isInUseQuery.Result,
		UsedIn:      isInUseQuery.UsedIn,
	}

	return nil
}

func uploadImageFile(ctx context.Context, c *cmd.UploadImageFile) error {
	sanitizedName := blob.SanitizeFileName(c.Name)
	if sanitizedName == "" {
		sanitizedName = "file"
	}

	prefix := c.Prefix
	if prefix == "" {
		prefix = "files/"
	}

	blobKey := prefix + sanitizedName + "-" + rand.String(12)

	storeCmd := &cmd.StoreBlob{
		Key:         blobKey,
		Content:     c.Content,
		ContentType: c.ContentType,
	}

	if err := bus.Dispatch(ctx, storeCmd); err != nil {
		return errors.Wrap(err, "failed to store blob")
	}

	c.Result = &dto.FileInfo{
		Name:        c.Name,
		BlobKey:     blobKey,
		Size:        int64(len(c.Content)),
		ContentType: c.ContentType,
		CreatedAt:   time.Now(),
		IsInUse:     false,
	}

	return nil
}

func renameImageFile(ctx context.Context, c *cmd.RenameImageFile) error {
	blob.EnsureAuthorizedPrefix(ctx, c.BlobKey)

	blobQuery := &query.GetBlobByKey{
		Key: c.BlobKey,
	}
	if err := bus.Dispatch(ctx, blobQuery); err != nil {
		return errors.Wrap(err, "failed to get blob")
	}

	// Since we cant rename blobs directly, we need to:
	// 1 create a new blob with a new name
	// 2 Update any references to point to the new blob
	// 3 delete the old blob

	sanitizedName := blob.SanitizeFileName(c.Name)
	if sanitizedName == "" {
		sanitizedName = "file"
	}

	parts := strings.Split(c.BlobKey, "/")
	dir := strings.Join(parts[:len(parts)-1], "/")
	newBlobKey := dir + "/" + sanitizedName + "-" + rand.String(8)

	storeCmd := &cmd.StoreBlob{
		Key:         newBlobKey,
		Content:     blobQuery.Result.Content,
		ContentType: blobQuery.Result.ContentType,
	}
	if err := bus.Dispatch(ctx, storeCmd); err != nil {
		return errors.Wrap(err, "failed to store blob with new name")
	}

	updateRefsCmd := &cmd.UpdateImageFileReferences{
		OldBlobKey: c.BlobKey,
		NewBlobKey: newBlobKey,
	}
	if err := bus.Dispatch(ctx, updateRefsCmd); err != nil {
		return errors.Wrap(err, "failed to update file references")
	}

	isInUseQuery := &query.IsImageFileInUse{
		BlobKey: newBlobKey,
	}
	if err := bus.Dispatch(ctx, isInUseQuery); err != nil {
		return errors.Wrap(err, "failed to check if file is in use")
	}

	deleteCmd := &cmd.DeleteBlob{
		Key: c.BlobKey,
	}
	_ = bus.Dispatch(ctx, deleteCmd) // Ignore error, fmd will clean up later

	c.Result = &dto.FileInfo{
		Name:        c.Name,
		BlobKey:     newBlobKey,
		Size:        blobQuery.Result.Size,
		ContentType: blobQuery.Result.ContentType,
		CreatedAt:   time.Now(),
		IsInUse:     isInUseQuery.Result,
	}

	return nil
}

func deleteImageFileReferences(ctx context.Context, c *cmd.DeleteImageFileReferences) error {
	blob.EnsureAuthorizedPrefix(ctx, c.BlobKey)

	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		queries := []struct {
			query string
			args  []interface{}
			desc  string
		}{
			{
				query: "UPDATE tenants SET logo_bkey = NULL WHERE logo_bkey = $1 AND id = $2",
				args:  []interface{}{c.BlobKey, tenant.ID},
				desc:  "remove from tenant logos",
			},
			{
				query: "UPDATE users SET avatar_bkey = '', avatar_type = $1 WHERE avatar_bkey = $2",
				args:  []interface{}{enum.AvatarTypeLetter, c.BlobKey},
				desc:  "remove from user avatars",
			},
			{
				query: "DELETE FROM attachments WHERE attachment_bkey = $1 AND tenant_id = $2",
				args:  []interface{}{c.BlobKey, tenant.ID},
				desc:  "delete from attachments",
			},
		}

		for _, q := range queries {
			_, err := trx.Execute(q.query, q.args...)
			if err != nil {
				return errors.Wrap(err, "failed to "+q.desc)
			}
		}

		return nil
	})
}

func deleteImageFile(ctx context.Context, c *cmd.DeleteImageFile) error {
	blob.EnsureAuthorizedPrefix(ctx, c.BlobKey)

	isInUseQuery := &query.IsImageFileInUse{
		BlobKey: c.BlobKey,
	}
	if err := bus.Dispatch(ctx, isInUseQuery); err != nil {
		return errors.Wrap(err, "failed to check if file is in use")
	}

	if isInUseQuery.Result {
		return errors.New("Cannot delete file that is in use")
	}

	deleteCmd := &cmd.DeleteBlob{
		Key: c.BlobKey,
	}
	if err := bus.Dispatch(ctx, deleteCmd); err != nil {
		return errors.Wrap(err, "failed to delete blob")
	}

	return nil
}

func updateImageFileReferences(ctx context.Context, c *cmd.UpdateImageFileReferences) error {
	blob.EnsureAuthorizedPrefix(ctx, c.OldBlobKey)
	blob.EnsureAuthorizedPrefix(ctx, c.NewBlobKey)

	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		queries := []struct {
			query string
			args  []interface{}
			desc  string
		}{
			{
				query: "UPDATE tenants SET logo_bkey = $1 WHERE logo_bkey = $2 AND id = $3",
				args:  []interface{}{c.NewBlobKey, c.OldBlobKey, tenant.ID},
				desc:  "update tenant logos",
			},
			{
				query: "UPDATE users SET avatar_bkey = $1 WHERE avatar_bkey = $2",
				args:  []interface{}{c.NewBlobKey, c.OldBlobKey},
				desc:  "update user avatars",
			},
			{
				query: "UPDATE attachments SET attachment_bkey = $1 WHERE attachment_bkey = $2",
				args:  []interface{}{c.NewBlobKey, c.OldBlobKey},
				desc:  "update attachments",
			},
		}

		for _, q := range queries {
			_, err := trx.Execute(q.query, q.args...)
			if err != nil {
				return errors.Wrap(err, "failed to "+q.desc)
			}
		}

		return nil
	})
}

func listImageFiles(ctx context.Context, q *query.ListImageFiles) error {
	listQuery := &query.ListBlobs{
		Prefix: "",
	}
	if err := bus.Dispatch(ctx, listQuery); err != nil {
		return errors.Wrap(err, "failed to list blobs")
	}

	validPrefixes := []string{"files/", "attachments/", "avatars/", "logos/"}
	filteredKeys := make([]string, 0, len(listQuery.Result))

	for _, blobKey := range listQuery.Result {
		valid := false
		for _, prefix := range validPrefixes {
			if strings.HasPrefix(blobKey, prefix) {
				valid = true
				break
			}
		}

		if valid {
			if q.Search != "" {
				searchLower := strings.ToLower(q.Search)
				if !strings.Contains(strings.ToLower(blobKey), searchLower) {
					continue
				}
			}
			filteredKeys = append(filteredKeys, blobKey)
		}
	}

	allFiles := make([]*dto.FileInfo, 0, len(filteredKeys))
	batchSize := 20

	for i := 0; i < len(filteredKeys); i += batchSize {
		end := i + batchSize
		if end > len(filteredKeys) {
			end = len(filteredKeys)
		}

		batch := filteredKeys[i:end]
		batchFiles := processFileBatch(ctx, batch)
		allFiles = append(allFiles, batchFiles...)
	}

	filteredFiles := allFiles
	if q.Search != "" {
		searchLower := strings.ToLower(q.Search)
		filteredFiles = make([]*dto.FileInfo, 0)
		for _, file := range allFiles {
			if strings.Contains(strings.ToLower(file.Name), searchLower) ||
				strings.Contains(strings.ToLower(file.ContentType), searchLower) {
				filteredFiles = append(filteredFiles, file)
			}
		}
	}

	sort.Slice(filteredFiles, func(i, j int) bool {
		var result bool
		switch q.SortBy {
		case "name":
			result = filteredFiles[i].Name < filteredFiles[j].Name
		case "size":
			result = filteredFiles[i].Size < filteredFiles[j].Size
		case "contentType":
			result = filteredFiles[i].ContentType < filteredFiles[j].ContentType
		default:
			result = filteredFiles[i].CreatedAt.After(filteredFiles[j].CreatedAt)
		}

		if q.SortDir == "desc" {
			return !result
		}
		return result
	})

	total := len(filteredFiles)
	totalPages := (total + q.PageSize - 1) / q.PageSize

	start := (q.Page - 1) * q.PageSize
	end := start + q.PageSize
	if start >= total {
		start = 0
		end = 0
	}
	if end > total {
		end = total
	}

	if start < end {
		q.Result = filteredFiles[start:end]
	} else {
		q.Result = []*dto.FileInfo{}
	}

	q.Total = total
	q.TotalPages = totalPages

	return nil
}

func processFileBatch(ctx context.Context, blobKeys []string) []*dto.FileInfo {
	results := make([]*dto.FileInfo, 0, len(blobKeys))

	for _, blobKey := range blobKeys {
		blobQuery := &query.GetBlobByKey{
			Key: blobKey,
		}
		blobErr := bus.Dispatch(ctx, blobQuery)
		if blobErr != nil {
			continue
		}

		isInUseQuery := &query.IsImageFileInUse{
			BlobKey: blobKey,
		}
		if err := bus.Dispatch(ctx, isInUseQuery); err != nil {
			continue
		}

		nameQuery := &query.GetNameFromBlobKey{
			BlobKey: blobKey,
		}
		if err := bus.Dispatch(ctx, nameQuery); err != nil {
			continue
		}

		fileInfo := &dto.FileInfo{
			Name:        nameQuery.Result,
			BlobKey:     blobKey,
			Size:        blobQuery.Result.Size,
			ContentType: blobQuery.Result.ContentType,
			CreatedAt:   time.Now(),
			IsInUse:     isInUseQuery.Result,
			UsedIn:      isInUseQuery.UsedIn,
		}

		results = append(results, fileInfo)
	}

	return results
}
