package postgres

import (
	"context"
	"database/sql"
	"fmt"
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
				(SELECT COUNT(*) FROM attachments a
					LEFT JOIN posts p ON a.post_id = p.id
					LEFT JOIN comments c ON a.comment_id = c.id
					WHERE a.attachment_bkey = $1 AND a.tenant_id = $2
					  AND (a.post_id IS NULL OR p.status != 6)
					  AND (a.comment_id IS NULL OR c.deleted_at IS NULL)
				) AS attachment_count
		`, q.BlobKey, tenant.ID)

		if err != nil {
			return errors.Wrap(err, "failed to check file usage")
		}

		if usageCount.LogoCount > 0 {
			usageLocations = append(usageLocations, "Tenant Logo")
			q.Result = true
		}

		if usageCount.AvatarCount > 0 {
			q.Result = true

			rows, err := trx.Query(`
				SELECT id, name FROM users WHERE avatar_bkey = $1
			`, q.BlobKey)
			if err != nil {
				return errors.Wrap(err, "failed to fetch user avatar usage")
			}
			defer rows.Close()

			for rows.Next() {
				var userID int
				var userName string
				if err := rows.Scan(&userID, &userName); err != nil {
					return errors.Wrap(err, "failed to scan user avatar row")
				}
				usageLocations = append(usageLocations, fmt.Sprintf("User #%d: %s", userID, userName))
			}

			if err := rows.Err(); err != nil {
				return errors.Wrap(err, "error iterating user avatar rows")
			}
		}

		if usageCount.AttachmentCount > 0 {
			q.Result = true

			rows, err := trx.Query(`
				SELECT 
					COALESCE(p.id, c.post_id) as post_id,
					a.comment_id,
					CASE 
						WHEN a.post_id IS NOT NULL THEN 'post'
						WHEN a.comment_id IS NOT NULL THEN 'comment'
						ELSE 'unknown'
					END as attachment_type
				FROM attachments a
				LEFT JOIN posts p ON a.post_id = p.id
				LEFT JOIN comments c ON a.comment_id = c.id
				WHERE a.attachment_bkey = $1 AND a.tenant_id = $2
				  AND (a.post_id IS NULL OR p.status != 6)
				  AND (a.comment_id IS NULL OR c.deleted_at IS NULL)
			`, q.BlobKey, tenant.ID)

			if err != nil {
				return errors.Wrap(err, "failed to fetch detailed attachment usage")
			}
			defer rows.Close()

			for rows.Next() {
				var postID int
				var commentID sql.NullInt64
				var attachmentType string

				if err := rows.Scan(&postID, &commentID, &attachmentType); err != nil {
					return errors.Wrap(err, "failed to scan attachment usage row")
				}

				if attachmentType == "post" {
					usageLocations = append(usageLocations, fmt.Sprintf("Post #%d", postID))
				} else if attachmentType == "comment" && commentID.Valid {
					usageLocations = append(usageLocations, fmt.Sprintf("Comment #%d on Post #%d", commentID.Int64, postID))
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
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		validPrefixes := []string{"files/", "attachments/", "avatars/", "logos/"}

		orderColumn := "created_at"
		switch q.SortBy {
		case "name":
			orderColumn = "key"
		case "size":
			orderColumn = "size"
		case "contentType":
			orderColumn = "content_type"
		}

		orderDir := "DESC"
		if q.SortDir == "asc" {
			orderDir = "ASC"
		}

		prefixConditions := make([]string, len(validPrefixes))
		for i, prefix := range validPrefixes {
			prefixConditions[i] = fmt.Sprintf("key LIKE '%s%%'", prefix)
		}
		prefixWhere := "(" + strings.Join(prefixConditions, " OR ") + ")"

		searchWhere := ""
		args := []interface{}{tenant.ID}
		if q.Search != "" {
			searchWhere = " AND LOWER(key) LIKE $2"
			args = append(args, "%"+strings.ToLower(q.Search)+"%")
		}

		countQuery := fmt.Sprintf(`
			SELECT COUNT(*) FROM blobs 
			WHERE (tenant_id = $1 OR ($1 IS NULL AND tenant_id IS NULL))
			AND %s%s
		`, prefixWhere, searchWhere)

		var total int
		if err := trx.Scalar(&total, countQuery, args...); err != nil {
			return errors.Wrap(err, "failed to count blobs")
		}

		if total == 0 {
			q.Result = []*dto.FileInfo{}
			q.Total = 0
			q.TotalPages = 0
			return nil
		}

		offset := (q.Page - 1) * q.PageSize
		limitArg := len(args) + 1
		offsetArg := len(args) + 2

		blobsQuery := fmt.Sprintf(`
			SELECT key, content_type, size, created_at FROM blobs
			WHERE (tenant_id = $1 OR ($1 IS NULL AND tenant_id IS NULL))
			AND %s%s
			ORDER BY %s %s
			LIMIT $%d OFFSET $%d
		`, prefixWhere, searchWhere, orderColumn, orderDir, limitArg, offsetArg)

		args = append(args, q.PageSize, offset)

		type blobRow struct {
			Key         string    `db:"key"`
			ContentType string    `db:"content_type"`
			Size        int64     `db:"size"`
			CreatedAt   time.Time `db:"created_at"`
		}

		var blobs []*blobRow
		if err := trx.Select(&blobs, blobsQuery, args...); err != nil {
			return errors.Wrap(err, "failed to list blobs")
		}

		if len(blobs) == 0 {
			q.Result = []*dto.FileInfo{}
			q.Total = total
			q.TotalPages = (total + q.PageSize - 1) / q.PageSize
			return nil
		}

		blobKeys := make([]string, len(blobs))
		for i, b := range blobs {
			blobKeys[i] = b.Key
		}

		usageMap, err := getBulkFileUsage(trx, tenant.ID, blobKeys)
		if err != nil {
			return errors.Wrap(err, "failed to get bulk file usage")
		}

		attachmentKeys := make([]string, 0)
		avatarKeys := make([]string, 0)
		for _, key := range blobKeys {
			if strings.HasPrefix(key, "attachments/") {
				attachmentKeys = append(attachmentKeys, key)
			}
			if strings.HasPrefix(key, "avatars/") {
				avatarKeys = append(avatarKeys, key)
			}
		}

		attachmentContextMap := make(map[string][]string)
		if len(attachmentKeys) > 0 {
			attachmentContextMap, err = getBulkAttachmentContext(trx, tenant.ID, attachmentKeys)
			if err != nil {
				return errors.Wrap(err, "failed to get bulk attachment context")
			}
		}

		avatarContextMap := make(map[string][]string)
		if len(avatarKeys) > 0 {
			avatarContextMap, err = getBulkAvatarContext(trx, avatarKeys)
			if err != nil {
				return errors.Wrap(err, "failed to get bulk avatar context")
			}
		}

		results := make([]*dto.FileInfo, len(blobs))
		for i, b := range blobs {
			name := extractNameFromBlobKey(b.Key)
			usage := usageMap[b.Key]

			usedIn := make([]string, 0)
			if usage.LogoCount > 0 {
				usedIn = append(usedIn, "Tenant Logo")
			}
			if usage.AvatarCount > 0 {
				if contexts, ok := avatarContextMap[b.Key]; ok {
					usedIn = append(usedIn, contexts...)
				} else {
					usedIn = append(usedIn, "User Avatar")
				}
			}
			if usage.AttachmentCount > 0 {
				if contexts, ok := attachmentContextMap[b.Key]; ok {
					usedIn = append(usedIn, contexts...)
				}
			}

			results[i] = &dto.FileInfo{
				Name:        name,
				BlobKey:     b.Key,
				Size:        b.Size,
				ContentType: b.ContentType,
				CreatedAt:   b.CreatedAt,
				IsInUse:     usage.LogoCount > 0 || usage.AvatarCount > 0 || usage.AttachmentCount > 0,
				UsedIn:      usedIn,
			}
		}

		q.Result = results
		q.Total = total
		q.TotalPages = (total + q.PageSize - 1) / q.PageSize

		return nil
	})
}

type fileUsageCounts struct {
	LogoCount       int
	AvatarCount     int
	AttachmentCount int
}

func getBulkFileUsage(trx *dbx.Trx, tenantID int, blobKeys []string) (map[string]fileUsageCounts, error) {
	result := make(map[string]fileUsageCounts)
	for _, key := range blobKeys {
		result[key] = fileUsageCounts{}
	}

	if len(blobKeys) == 0 {
		return result, nil
	}

	placeholders := make([]string, len(blobKeys))
	args := make([]interface{}, len(blobKeys)+1)
	args[0] = tenantID
	for i, key := range blobKeys {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args[i+1] = key
	}
	inClause := strings.Join(placeholders, ", ")

	usageQuery := fmt.Sprintf(`
		SELECT key, usage_type, COUNT(*) as count FROM (
			SELECT logo_bkey as key, 'logo' as usage_type FROM tenants WHERE logo_bkey IN (%s) AND id = $1
			UNION ALL
			SELECT avatar_bkey as key, 'avatar' as usage_type FROM users WHERE avatar_bkey IN (%s)
			UNION ALL
			SELECT a.attachment_bkey as key, 'attachment' as usage_type 
			FROM attachments a
			LEFT JOIN posts p ON a.post_id = p.id
			LEFT JOIN comments c ON a.comment_id = c.id
			WHERE a.attachment_bkey IN (%s) AND a.tenant_id = $1
			  AND (a.post_id IS NULL OR p.status != 6)
			  AND (a.comment_id IS NULL OR c.deleted_at IS NULL)
		) combined
		GROUP BY key, usage_type
	`, inClause, inClause, inClause)

	type usageRow struct {
		Key       string `db:"key"`
		UsageType string `db:"usage_type"`
		Count     int    `db:"count"`
	}

	var usageCounts []*usageRow
	if err := trx.Select(&usageCounts, usageQuery, args...); err != nil {
		return nil, errors.Wrap(err, "failed to get usage counts")
	}

	for _, row := range usageCounts {
		if usage, ok := result[row.Key]; ok {
			switch row.UsageType {
			case "logo":
				usage.LogoCount = row.Count
			case "avatar":
				usage.AvatarCount = row.Count
			case "attachment":
				usage.AttachmentCount = row.Count
			}
			result[row.Key] = usage
		}
	}

	return result, nil
}

func getBulkAttachmentContext(trx *dbx.Trx, tenantID int, blobKeys []string) (map[string][]string, error) {
	result := make(map[string][]string)
	if len(blobKeys) == 0 {
		return result, nil
	}

	placeholders := make([]string, len(blobKeys))
	args := make([]interface{}, len(blobKeys)+1)
	args[0] = tenantID
	for i, key := range blobKeys {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args[i+1] = key
	}
	inClause := strings.Join(placeholders, ", ")

	contextQuery := fmt.Sprintf(`
		SELECT 
			a.attachment_bkey as key,
			COALESCE(p.id, c.post_id) as post_id,
			a.comment_id,
			CASE 
				WHEN a.post_id IS NOT NULL THEN 'post'
				WHEN a.comment_id IS NOT NULL THEN 'comment'
				ELSE 'unknown'
			END as attachment_type
		FROM attachments a
		LEFT JOIN posts p ON a.post_id = p.id
		LEFT JOIN comments c ON a.comment_id = c.id
		WHERE a.attachment_bkey IN (%s) AND a.tenant_id = $1
		  AND (a.post_id IS NULL OR p.status != 6)
		  AND (a.comment_id IS NULL OR c.deleted_at IS NULL)
	`, inClause)

	type contextRow struct {
		Key            string        `db:"key"`
		PostID         int           `db:"post_id"`
		CommentID      sql.NullInt64 `db:"comment_id"`
		AttachmentType string        `db:"attachment_type"`
	}

	var contexts []*contextRow
	if err := trx.Select(&contexts, contextQuery, args...); err != nil {
		return nil, errors.Wrap(err, "failed to get attachment contexts")
	}

	for _, row := range contexts {
		var contextStr string
		if row.AttachmentType == "post" {
			contextStr = fmt.Sprintf("Post #%d", row.PostID)
		} else if row.AttachmentType == "comment" && row.CommentID.Valid {
			contextStr = fmt.Sprintf("Comment #%d on Post #%d", row.CommentID.Int64, row.PostID)
		}
		if contextStr != "" {
			result[row.Key] = append(result[row.Key], contextStr)
		}
	}

	return result, nil
}

func getBulkAvatarContext(trx *dbx.Trx, blobKeys []string) (map[string][]string, error) {
	result := make(map[string][]string)
	if len(blobKeys) == 0 {
		return result, nil
	}

	placeholders := make([]string, len(blobKeys))
	args := make([]interface{}, len(blobKeys))
	for i, key := range blobKeys {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = key
	}
	inClause := strings.Join(placeholders, ", ")

	contextQuery := fmt.Sprintf(`
		SELECT avatar_bkey as key, id, name FROM users WHERE avatar_bkey IN (%s)
	`, inClause)

	type contextRow struct {
		Key    string `db:"key"`
		UserID int    `db:"id"`
		Name   string `db:"name"`
	}

	var contexts []*contextRow
	if err := trx.Select(&contexts, contextQuery, args...); err != nil {
		return nil, errors.Wrap(err, "failed to get avatar contexts")
	}

	for _, row := range contexts {
		contextStr := fmt.Sprintf("User #%d: %s", row.UserID, row.Name)
		result[row.Key] = append(result[row.Key], contextStr)
	}

	return result, nil
}

func extractNameFromBlobKey(blobKey string) string {
	if blobKey == "" {
		return ""
	}

	parts := strings.Split(blobKey, "/")
	if len(parts) == 0 {
		return blobKey
	}

	filename := parts[len(parts)-1]

	if strings.HasPrefix(blobKey, "logos/") || strings.HasPrefix(blobKey, "avatars/") {
		extensionParts := strings.Split(filename, ".")
		if len(extensionParts) > 1 {
			extension := "." + extensionParts[len(extensionParts)-1]
			if strings.HasPrefix(blobKey, "logos/") {
				return "logo" + extension
			}
			return "avatar" + extension
		}
		if strings.HasPrefix(blobKey, "logos/") {
			return "logo"
		}
		return "avatar"
	}

	nameParts := strings.SplitN(filename, "-", 2)
	if len(nameParts) > 1 && len(nameParts[0]) > 10 {
		filename = nameParts[1]
	}

	extensionParts := strings.Split(filename, ".")
	extension := ""
	if len(extensionParts) > 1 {
		extension = "." + extensionParts[len(extensionParts)-1]
		filename = strings.TrimSuffix(filename, extension)
	}

	return filename + extension
}

func getPrunableFiles(ctx context.Context, q *query.GetPrunableFiles) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		q.Result = []string{}

		rows, err := trx.Query(`
			SELECT b.key 
			FROM blobs b
			WHERE b.tenant_id = $1
			  AND b.key NOT IN (SELECT logo_bkey FROM tenants WHERE logo_bkey IS NOT NULL AND id = $1)
			  AND b.key NOT IN (SELECT avatar_bkey FROM users WHERE avatar_bkey IS NOT NULL)
			  AND b.key NOT IN (SELECT attachment_bkey FROM attachments WHERE attachment_bkey IS NOT NULL AND tenant_id = $1)
		`, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to get prunable files")
		}
		defer rows.Close()

		for rows.Next() {
			var blobKey string
			if err := rows.Scan(&blobKey); err != nil {
				return errors.Wrap(err, "failed to scan prunable file row")
			}
			q.Result = append(q.Result, blobKey)
		}

		return rows.Err()
	})
}

