package s3

import (
	"context"
	"strings"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/blob"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

func scanBlobMetadata(ctx context.Context, q *query.ScanBlobMetadata) error {
	scope := basePath(ctx, "")
	cursor := q.Cursor
	for {
		request := &s3.ListObjectsV2Input{
			Bucket:  aws.String(env.Config.BlobStorage.S3.BucketName),
			Prefix:  aws.String(scope),
			MaxKeys: aws.Int64(int64(q.BatchSize)),
		}
		if cursor != "" {
			request.ContinuationToken = aws.String(cursor)
		}
		response, err := DefaultClient.ListObjectsV2WithContext(ctx, request)
		if err != nil {
			return err
		}
		complete := !aws.BoolValue(response.IsTruncated)
		next := aws.StringValue(response.NextContinuationToken)
		if !complete && (next == "" || next == cursor) {
			return errors.New("S3 inventory did not provide a new continuation token")
		}
		batch := make([]dto.BlobMetadata, 0, len(response.Contents))
		for _, item := range response.Contents {
			key := aws.StringValue(item.Key)
			if !strings.HasPrefix(key, scope) {
				return blob.ErrInvalidKeyFormat
			}
			if strings.HasSuffix(key, "/") {
				continue
			}
			key = strings.TrimPrefix(key, scope)
			batch = append(batch, dto.BlobMetadata{
				Key:         key,
				ContentType: "application/octet-stream",
				Size:        aws.Int64Value(item.Size),
				ModifiedAt:  aws.TimeValue(item.LastModified),
			})
		}
		if err := q.Accept(batch, next, complete); err != nil {
			return err
		}
		if complete {
			return nil
		}
		cursor = next
	}
}
