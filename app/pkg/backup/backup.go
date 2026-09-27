package backup

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"fmt"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
)

func Create(ctx context.Context) (*bytes.Buffer, error) {
	trx, err := dbx.BeginTxWithOptions(ctx, &sql.TxOptions{
		Isolation: sql.LevelRepeatableRead,
		ReadOnly:  true,
	})
	if err != nil {
		return nil, err
	}

	defer trx.Rollback()
	ctx = context.WithValue(ctx, app.TransactionCtxKey, trx)

	buffer := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buffer)

	for _, tableName := range []string{
		"attachments",
		"comments",
		"email_verifications",
		"notifications",
		"oauth_providers",
		"pages",
		"page_authors",
		"page_drafts",
		"page_reactions",
		"page_subscriptions",
		"page_topics",
		"page_topics_map",
		"page_tags",
		"page_tags_map",
		"posts",
		"post_subscribers",
		"post_tags",
		"post_votes",
		"post_vote_revisions",
		"reactions",
		"post_notification_deliveries",
		"post_notification_recipients",
		"tags",
		"tenants",
		"user_providers",
		"users",
		"user_settings",
	} {
		err := addTableDataToZipFile(ctx, zipWriter, tableName)
		if err != nil {
			return nil, err
		}
	}

	listBlobs := &query.ListBlobs{}
	if err := bus.Dispatch(ctx, listBlobs); err != nil {
		return nil, err
	}

	for _, bkey := range listBlobs.Result {
		err := addBlobToZipFile(ctx, zipWriter, bkey)
		if err != nil {
			return nil, err
		}
	}

	err = zipWriter.Close()
	if err != nil {
		return nil, errors.Wrap(err, "failed to close zip file")
	}

	if err := trx.Commit(); err != nil {
		return nil, err
	}

	return buffer, nil
}

func addBlobToZipFile(ctx context.Context, zipWriter *zip.Writer, bkey string) error {
	getBlob := &query.GetBlobByKey{Key: bkey}
	if err := bus.Dispatch(ctx, getBlob); err != nil {
		return errors.Wrap(err, "failed to get blob with key %s", bkey)
	}

	fileName := fmt.Sprintf("blobs/%s", bkey)
	fileWriter, err := zipWriter.Create(fileName)
	if err != nil {
		return errors.Wrap(err, "failed to create %s in zip file", fileName)
	}
	_, err = fileWriter.Write(getBlob.Result.Content)
	if err != nil {
		return errors.Wrap(err, "failed to write %s to zip file", fileName)
	}

	return nil
}

func addTableDataToZipFile(ctx context.Context, zipWriter *zip.Writer, tableName string) error {
	tableData, err := exportTable(ctx, tableName)
	if err != nil {
		return errors.Wrap(err, "failed to export %s table", tableName)
	}

	fileWriter, err := zipWriter.Create(fmt.Sprintf("%s.json", tableName))
	if err != nil {
		return errors.Wrap(err, "failed to create %s.json in zip file", tableName)
	}
	_, err = fileWriter.Write(tableData)
	if err != nil {
		return errors.Wrap(err, "failed to write %s.json to zip file", tableName)
	}

	return nil
}
