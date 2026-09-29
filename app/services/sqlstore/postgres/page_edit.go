package postgres

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/pagedoc"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/rand"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
	"github.com/reearth/ygo/crdt"
)

type dbPageEdit struct {
	State     []byte    `db:"collaborative_state"`
	Banner    string    `db:"banner_image_bkey"`
	UpdatedAt time.Time `db:"updated_at"`
}

func lockPageEdit(trx *dbx.Trx, tenantID, pageID int) error {
	var id int
	return trx.Scalar(&id, `SELECT id FROM pages WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, tenantID, pageID)
}

func readPageEdit(trx *dbx.Trx, tenantID, pageID int) (*dbPageEdit, error) {
	var saved dbPageEdit
	err := trx.Get(&saved, `
		SELECT collaborative_state, COALESCE(banner_image_bkey, '') AS banner_image_bkey, updated_at
		FROM page_drafts WHERE tenant_id=$1 AND page_id=$2 AND shared
	`, tenantID, pageID)
	return &saved, err
}

func pageEditSession(ctx context.Context, trx *dbx.Trx, tenantID, pageID, userID int) (*entity.PageEditSession, error) {
	saved, err := readPageEdit(trx, tenantID, pageID)
	if err != nil {
		return nil, err
	}
	vector, err := pagedoc.StateVector(saved.State)
	if err != nil {
		return nil, err
	}

	legacy := &query.GetPageDraft{PageID: pageID, UserID: userID}
	if err := getPageDraft(ctx, legacy); err != nil {
		return nil, err
	}
	result := &entity.PageEditSession{
		PageID:       pageID,
		State:        saved.State,
		StateVector:  vector,
		UpdatedAt:    saved.UpdatedAt,
		LegacyDrafts: []*entity.PageDraft{},
	}
	if legacy.Result != nil {
		result.LegacyDrafts = append(result.LegacyDrafts, legacy.Result)
	}
	return result, nil
}

func openPageEdit(ctx context.Context, c *cmd.OpenPageEdit) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		ctx, err := permissionContext(ctx, trx, tenant, user, entity.ManagePages)
		if err != nil {
			return err
		}

		pageID := c.PageID
		if pageID == 0 {
			if !validate.ValidSubmissionID(c.SubmissionID) {
				return validate.Failed("A Page creation ID is required.")
			}
			receipt := commandReceipt{
				TenantID: tenant.ID, UserID: user.ID, Kind: "page-open", SubmissionID: c.SubmissionID,
			}
			replayed, err := receipt.read(trx, &pageID)
			if err != nil {
				return err
			}
			if replayed {
				if pageID == 0 {
					return app.ErrNotFound
				}
			} else {
				created := &cmd.CreatePage{
					Title:          "Untitled Page",
					Slug:           "draft-" + rand.String(32),
					Status:         entity.PageStatusDraft,
					Visibility:     entity.PageVisibilityPublic,
					AllowReactions: true,
				}
				if err := createPage(ctx, created); err != nil {
					return err
				}
				pageID = created.Result.ID
				if err := receipt.save(trx, pageID); err != nil {
					return err
				}
			}
		}

		if err := lockPageEdit(trx, tenant.ID, pageID); err != nil {
			return err
		}
		_, err = readPageEdit(trx, tenant.ID, pageID)
		if errors.Cause(err) == app.ErrNotFound {
			page := &query.GetPageByID{ID: pageID}
			if err := getPageByID(ctx, page); err != nil {
				return err
			}
			if c.PageID == 0 {
				page.Result.Title = ""
				page.Result.Slug = ""
			}
			state := pagedoc.New(page.Result)
			materialized, err := pagedoc.Read(state)
			if err != nil {
				return err
			}
			if _, err := trx.Execute(`
				INSERT INTO page_drafts (tenant_id, page_id, user_id, shared, collaborative_state)
				VALUES ($1,$2,$3,true,$4)
			`, tenant.ID, pageID, user.ID, state); err != nil {
				return err
			}
			if _, err := savePageEdit(trx, tenant.ID, pageID, state, materialized); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

		c.Result, err = pageEditSession(ctx, trx, tenant.ID, pageID, user.ID)
		return err
	})
}

func getPageEdit(ctx context.Context, q *query.GetPageEdit) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		ctx, err := permissionContext(ctx, trx, tenant, user, entity.ManagePages)
		if err != nil {
			return err
		}
		var pageID int
		if err := trx.Scalar(&pageID, `
			SELECT id FROM pages WHERE tenant_id=$1 AND id=$2 FOR SHARE
		`, tenant.ID, q.PageID); err != nil {
			return err
		}
		q.Result, err = pageEditSession(ctx, trx, tenant.ID, q.PageID, user.ID)
		return err
	})
}

func savePageEdit(trx *dbx.Trx, tenantID, pageID int, state []byte, page *cmd.UpdatePage) (time.Time, error) {
	var updatedAt time.Time
	err := trx.Scalar(&updatedAt, `
		UPDATE page_drafts SET collaborative_state=$3, title=$4, slug=$5, content=$6, excerpt=$7,
			banner_image_bkey=$8, meta_description=$9, show_toc=$10, draft_data='{}', updated_at=clock_timestamp()
		WHERE tenant_id=$1 AND page_id=$2 AND shared
		RETURNING updated_at
	`, tenantID, pageID, state, page.Title, page.Slug, page.Content, page.Excerpt,
		page.BannerImage.BlobKey, page.MetaDescription, page.ShowTOC)
	return updatedAt, err
}

func unlinkPageEditBanners(trx *dbx.Trx, tenantID int, key string) error {
	var drafts []*struct {
		PageID int    `db:"page_id"`
		State  []byte `db:"collaborative_state"`
	}
	if err := trx.Select(&drafts, `
		SELECT page_id, collaborative_state FROM page_drafts
		WHERE tenant_id=$1 AND shared AND banner_image_bkey=$2
		ORDER BY page_id FOR UPDATE
	`, tenantID, key); err != nil {
		return err
	}

	for _, draft := range drafts {
		state, err := pagedoc.ClearBanner(draft.State)
		if err != nil {
			return err
		}
		page, err := pagedoc.Read(state)
		if err != nil {
			return err
		}
		if _, err := savePageEdit(trx, tenantID, draft.PageID, state, page); err != nil {
			return err
		}
	}
	return nil
}

func syncPageEdit(ctx context.Context, c *cmd.SyncPageEdit) error {
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		ctx, err := permissionContext(ctx, trx, tenant, user, entity.ManagePages)
		if err != nil {
			return err
		}
		if err := lockPageEdit(trx, tenant.ID, c.PageID); err != nil {
			return err
		}
		saved, err := readPageEdit(trx, tenant.ID, c.PageID)
		if err != nil {
			return err
		}
		merged, err := pagedoc.Merge(saved.State, c.Update, c.StateVector)
		if err != nil {
			return err
		}
		if err := claimPageBanner(ctx, merged.Page.BannerImage, saved.Banner); err != nil {
			if _, invalidClaim := err.(*validate.Result); !invalidClaim {
				return err
			}

			var deletedUpload bool
			if lookupError := trx.Scalar(&deletedUpload, `
				SELECT EXISTS (
					SELECT 1 FROM media_assets asset
					WHERE asset.tenant_id=$1 AND asset.page_id=$2 AND asset.key=$3
					  AND (asset.deleted_at IS NOT NULL OR asset.deletion_requested_at IS NOT NULL)
				)
			`, tenant.ID, c.PageID, merged.Page.BannerImage.BlobKey); lookupError != nil {
				return lookupError
			}
			if !deletedUpload {
				return err
			}

			state, err := pagedoc.ClearBanner(merged.State)
			if err != nil {
				return err
			}
			merged, err = pagedoc.Merge(state, c.Update, c.StateVector)
			if err != nil {
				return err
			}
		}
		c.AcceptedUpdate, err = crdt.MergeUpdatesV1(c.Update, merged.Update)
		if err != nil {
			return err
		}

		updatedAt := saved.UpdatedAt
		if !bytes.Equal(saved.State, merged.State) {
			updatedAt, err = savePageEdit(trx, tenant.ID, c.PageID, merged.State, merged.Page)
			if err != nil {
				return err
			}
		}
		c.Result = &entity.PageEditSync{Update: merged.Update, StateVector: merged.StateVector, UpdatedAt: updatedAt}
		return nil
	})
}

func publishPageEdit(ctx context.Context, c *cmd.PublishPageEdit) error {
	c.Replayed = false
	return using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		ctx, err := permissionContext(ctx, trx, tenant, user, entity.ManagePages)
		if err != nil {
			return err
		}
		if !validate.ValidSubmissionID(c.SubmissionID) {
			return validate.Failed("A Page publication ID is required.")
		}
		if err := lockPageEdit(trx, tenant.ID, c.PageID); err != nil {
			return err
		}
		receipt := commandReceipt{
			TenantID: tenant.ID, UserID: user.ID, Kind: "page-publish",
			SubmissionID: c.SubmissionID, Fingerprint: fmt.Sprintf("%d:%s", c.PageID, c.Status),
		}
		replayed, err := receipt.read(trx, &c.Result)
		if err != nil {
			return err
		}
		if replayed {
			c.Replayed = true
			return nil
		}
		saved, err := readPageEdit(trx, tenant.ID, c.PageID)
		if err != nil {
			return err
		}
		state, err := pagedoc.SetStatus(saved.State, c.Status)
		if err != nil {
			return err
		}
		page, err := pagedoc.Read(state)
		if err != nil {
			return err
		}
		page.PageID = c.PageID
		action := &actions.CreateUpdatePage{
			PageID:             c.PageID,
			Title:              page.Title,
			Slug:               page.Slug,
			Content:            page.Content,
			Excerpt:            page.Excerpt,
			BannerImage:        page.BannerImage,
			Status:             string(page.Status),
			Visibility:         string(page.Visibility),
			AllowedRoles:       page.AllowedRoles,
			ParentPageID:       page.ParentPageID,
			AllowComments:      page.AllowComments,
			AllowCommentImages: page.AllowCommentImages,
			AllowReactions:     page.AllowReactions,
			ShowTOC:            page.ShowTOC,
			ScheduledFor:       page.ScheduledFor,
			Authors:            page.Authors,
			Topics:             page.Topics,
			Tags:               page.Tags,
			MetaDescription:    page.MetaDescription,
			CanonicalURL:       page.CanonicalURL,
		}
		if result := action.Validate(ctx, ctx.Value(app.UserCtxKey).(*entity.User)); !result.Ok {
			return result
		}
		if err := updatePage(ctx, page); err != nil {
			return err
		}
		if !bytes.Equal(saved.State, state) {
			if _, err := savePageEdit(trx, tenant.ID, c.PageID, state, page); err != nil {
				return err
			}
		}
		c.Result = page.Result
		return receipt.save(trx, c.Result)
	})
}

func uploadPageEditBanner(ctx context.Context, c *cmd.UploadPageEditBanner) error {
	if c.Image == nil || c.Image.Upload == nil || c.Image.Remove || len(c.Image.Upload.Content) == 0 {
		return validate.Failed("Choose an image to upload.")
	}
	if !validate.ValidSubmissionID(c.SubmissionID) {
		return validate.Failed("An image upload ID is required.")
	}
	messages, err := validate.ImageUpload(ctx, c.Image, validate.ImageUploadOpts{MaxKilobytes: 5000})
	if err != nil {
		return err
	}
	if len(messages) > 0 {
		return validate.Failed(messages...)
	}

	tenant := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	user := ctx.Value(app.UserCtxKey).(*entity.User)
	operation := sha256.Sum256([]byte(fmt.Sprintf("page-banner:%d:%s", c.PageID, c.SubmissionID)))
	contentHash := sha256.Sum256(c.Image.Upload.Content)
	fingerprint, err := json.Marshal([]any{c.PageID, c.Image.Upload.FileName, c.Image.Upload.ContentType, contentHash})
	if err != nil {
		return err
	}
	receipt := commandReceipt{
		TenantID: tenant.ID, UserID: user.ID, Kind: "media-upload",
		SubmissionID: fmt.Sprintf("page:%x", operation), Fingerprint: string(fingerprint),
	}

	c.Result = ""
	var prepared *dto.PreparedImage
	for {
		err := using(ctx, func(ctx context.Context, trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
			ctx, err := permissionContext(ctx, trx, tenant, user, entity.ManagePages)
			if err != nil {
				return err
			}
			if err := lockPageEdit(trx, tenant.ID, c.PageID); err != nil {
				return err
			}
			if _, err := readPageEdit(trx, tenant.ID, c.PageID); err != nil {
				return err
			}

			var key string
			replayed, err := receipt.read(trx, &key)
			if err != nil {
				return err
			}
			if !replayed {
				if prepared == nil {
					return nil
				}

				key = prepared.Key
				if err := saveMediaImage(ctx, mediaUpload{
					PreparedImage: prepared, Name: c.Image.Upload.FileName, PageID: c.PageID,
				}); err != nil {
					return err
				}
				if err := receipt.save(trx, key); err != nil {
					return err
				}
			}
			c.Result, err = pageEditBannerResult(ctx, key)
			return err
		})
		if err != nil || c.Result != "" {
			return err
		}

		identity := sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%s:%s", tenant.ID, user.ID, receipt.SubmissionID, receipt.Fingerprint)))
		prepared, err = prepareInlineImage(ctx, c.Image.Upload.Content, fmt.Sprintf("pages/%x.webp", identity))
		if err != nil {
			return err
		}
	}
}

func pageEditBannerResult(ctx context.Context, key string) (string, error) {
	stored := &query.GetMediaFile{BlobKey: key}
	if err := getMediaFile(ctx, stored); err != nil {
		return "", err
	}
	if stored.Result.State != "ready" {
		return "", validate.Failed("This image was deleted. Choose a new image.")
	}
	return key, nil
}
