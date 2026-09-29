package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/imagic"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/blob"
)

func setMediaFileURLs(file *dto.FileInfo) {
	file.URL = (&url.URL{Path: "/static/images/" + file.BlobKey}).String()
	file.ThumbnailURL = "/api/admin/files/thumbnail?key=" + url.QueryEscape(file.BlobKey) + "&size=" + strconv.Itoa(imagic.ThumbnailSmall)
}

func fileSearchPattern(search string) string {
	search = strings.TrimSpace(search)
	if search == "" {
		return ""
	}

	escape := strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`)
	return "%" + escape.Replace(strings.ToLower(search)) + "%"
}

func listMediaFiles(ctx context.Context, q *query.ListImageFiles) error {
	tenant := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	searchPattern := fileSearchPattern(q.Search)
	storageSource := blob.StorageSource()
	trx, err := dbx.BeginTxWithOptions(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return err
	}
	defer trx.Rollback()

	var totals struct {
		Count    int       `db:"count"`
		Bytes    int64     `db:"bytes"`
		ListedAt time.Time `db:"listed_at"`
	}
	err = trx.Get(&totals, `
		SELECT COUNT(*) AS count, COALESCE(SUM(size), 0) AS bytes, statement_timestamp() AS listed_at
		FROM media_file_listing($1, $2, $3, $4, NULL, $5, $6, $7)
	`, tenant.ID, searchPattern, q.Type, q.Usage, q.IncludeDeleted, q.IncludeDrafts, storageSource)
	if err != nil {
		return err
	}

	q.Total = totals.Count
	q.TotalBytes = totals.Bytes
	q.ListedAt = totals.ListedAt
	q.TotalPages = (totals.Count + q.PageSize - 1) / q.PageSize
	q.Page = max(1, min(q.Page, q.TotalPages))
	q.Result = []*dto.FileInfo{}
	if totals.Count == 0 {
		return trx.Commit()
	}

	pageStart := (q.Page - 1) * q.PageSize
	pageSize := min(q.PageSize, totals.Count-pageStart)
	offset := pageStart
	direction := q.SortDir
	fromEnd := totals.Count-pageStart-pageSize < pageStart
	if fromEnd {
		offset = totals.Count - pageStart - pageSize
		if direction == "asc" {
			direction = "desc"
		} else {
			direction = "asc"
		}
	}

	err = trx.Select(&q.Result, fmt.Sprintf(`
		WITH page AS MATERIALIZED (
			SELECT key
			FROM media_file_listing($1, $2, $3, $4, NULL, $7, $8, $9)
			ORDER BY %s
			LIMIT $5 OFFSET $6
		)
		SELECT file.*, COALESCE(refs.is_in_use, false) AS is_in_use,
		       COALESCE(refs.has_protected_references, false) AS has_protected_references
		FROM page
		JOIN media_file_listing($1, '', 'all', 'all', NULL, false, false, $9) file USING (key)
		LEFT JOIN media_reference_flags($1, ARRAY(SELECT key FROM page), $7, $8) refs USING (key)
		ORDER BY %s
	`, mediaFileOrder(q.SortBy, direction), mediaFileOrder(q.SortBy, q.SortDir)),
		tenant.ID, searchPattern, q.Type, q.Usage,
		pageSize, offset, q.IncludeDeleted, q.IncludeDrafts, storageSource)
	if err != nil {
		return err
	}

	for _, file := range q.Result {
		setMediaFileURLs(file)
	}

	return trx.Commit()
}

func mediaFileOrder(sortBy, direction string) string {
	var column string
	switch sortBy {
	case "name":
		column = `lower("name")`
	case "size":
		column = `"size"`
	case "createdAt":
		column = `"created_at"`
	default:
		panic("file sort field was not validated")
	}

	switch direction {
	case "asc":
		return column + ` ASC, "key" ASC`
	case "desc":
		return column + ` DESC, "key" DESC`
	default:
		panic("file sort direction was not validated")
	}
}

func getMediaFile(ctx context.Context, q *query.GetMediaFile) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var file dto.FileInfo
		err := trx.Get(&file, `
			SELECT file.key, file.name, file.content_type, file.size, file.created_at,
			       file.width, file.height, file.state, file.last_error,
			       COALESCE(refs.is_in_use, false) AS is_in_use,
			       COALESCE(refs.has_protected_references, false) AS has_protected_references
			FROM media_assets file
			LEFT JOIN media_reference_flags($1, ARRAY[$2]) refs USING (key)
			WHERE file.tenant_id=$1 AND file.key=$2 AND file.deleted_at IS NULL AND file.storage_source=$3
		`, tenant.ID, q.BlobKey, blob.StorageSource())
		if err != nil {
			return err
		}

		setMediaFileURLs(&file)
		q.Result = &file
		return nil
	})
}

func getFileUsage(ctx context.Context, q *query.GetFileUsage) error {
	q.PageSize = 50
	q.Page = max(1, q.Page)
	q.Result = []*dto.FileReference{}
	tenant := ctx.Value(app.TenantCtxKey).(*entity.Tenant)

	trx, err := dbx.BeginTxWithOptions(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return err
	}
	defer trx.Rollback()

	if err := trx.Scalar(&q.Total, "SELECT COUNT(*) FROM media_asset_refs WHERE tenant_id=$1 AND key=$2", tenant.ID, q.BlobKey); err != nil {
		return err
	}

	q.TotalPages = (q.Total + q.PageSize - 1) / q.PageSize
	q.Page = max(1, min(q.Page, q.TotalPages))

	err = trx.Select(&q.Result, `
		SELECT reference.kind, reference.id, source.title,
		       COALESCE(source.url, '') AS url, reference.field, reference.scope
		FROM (
			SELECT kind, id, field, scope
			FROM media_references WHERE tenant_id=$1 AND key=$2
			ORDER BY kind, id, field
			LIMIT $3 OFFSET $4
		) reference
		CROSS JOIN LATERAL (
			SELECT title, url FROM media_reference_sources
			WHERE tenant_id=$1 AND kind=reference.kind AND owner_id=reference.id
			LIMIT 1
		) source
		ORDER BY reference.kind, reference.id, reference.field
	`, tenant.ID, q.BlobKey, q.PageSize, (q.Page-1)*q.PageSize)
	if err != nil {
		return err
	}

	return trx.Commit()
}
