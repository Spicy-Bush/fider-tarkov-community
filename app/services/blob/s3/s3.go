package s3

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/readlimit"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/blob"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
)

// DefaultClient is an S3 Client
var DefaultClient *s3.S3

func init() {
	bus.Register(Service{})
}

type Service struct{}

var backend = blob.Backend{
	Read:         getBlobByKey,
	Write:        storeBlob,
	Remove:       deleteBlob,
	ListKeys:     listBlobs,
	ScanMetadata: scanBlobMetadata,
}

func (s Service) Name() string {
	return "S3"
}

func (s Service) Category() string {
	return "blobstorage"
}

func (s Service) Enabled() bool {
	return env.Config.BlobStorage.Type == "s3"
}

func (s Service) Init() {
	s3EnvConfig := env.Config.BlobStorage.S3
	s3Config := &aws.Config{Region: aws.String(s3EnvConfig.Region)}
	if s3EnvConfig.AccessKeyID != "" || s3EnvConfig.SecretAccessKey != "" {
		s3Config.Credentials = credentials.NewStaticCredentials(s3EnvConfig.AccessKeyID, s3EnvConfig.SecretAccessKey, "")
	}
	if s3EnvConfig.EndpointURL != "" {
		s3Config.Endpoint = aws.String(s3EnvConfig.EndpointURL)
		s3Config.DisableSSL = aws.Bool(strings.HasPrefix(s3EnvConfig.EndpointURL, "http://"))
		s3Config.S3ForcePathStyle = aws.Bool(true)
	}
	awsSession, err := session.NewSession(s3Config)
	if err != nil {
		panic(err)
	}
	DefaultClient = s3.New(awsSession)

	backend.Register()
}

func listBlobs(ctx context.Context, q *query.ListBlobs) error {
	prefix := basePath(ctx, q.Prefix)
	scope := basePath(ctx, "")
	q.Result = nil
	files := make([]string, 0)
	var pageError error
	continuations := make(map[string]bool)
	err := DefaultClient.ListObjectsV2PagesWithContext(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(env.Config.BlobStorage.S3.BucketName),
		MaxKeys: aws.Int64(1000),
		Prefix:  aws.String(prefix),
	}, func(response *s3.ListObjectsV2Output, _ bool) bool {
		if aws.BoolValue(response.IsTruncated) {
			token := aws.StringValue(response.NextContinuationToken)
			if token == "" || continuations[token] {
				pageError = errors.New("S3 listing did not provide a new continuation token")
				return false
			}
			continuations[token] = true
		}
		for _, item := range response.Contents {
			key := aws.StringValue(item.Key)
			if !strings.HasPrefix(key, prefix) {
				pageError = blob.ErrInvalidKeyFormat
				return false
			}
			if strings.HasSuffix(key, "/") || (scope == "" && strings.HasPrefix(key, "tenants/")) {
				continue
			}
			key = strings.TrimPrefix(key, scope)
			files = append(files, key)
		}
		return true
	})
	if err != nil {
		return wrap(err, "failed to list blobs from S3")
	}
	if pageError != nil {
		return pageError
	}

	sort.Strings(files)
	q.Result = files
	return nil
}

func getBlobByKey(ctx context.Context, q *query.GetBlobByKey) error {
	resp, err := DefaultClient.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: aws.String(env.Config.BlobStorage.S3.BucketName),
		Key:    aws.String(keyFullPathURL(ctx, q.Key)),
	})
	if err != nil {
		if isNotFound(err) {
			return wrap(blob.ErrNotFound, "unable to find blob '%s' on S3", q.Key)
		}
		return wrap(err, "failed to get blob '%s' from S3", q.Key)
	}
	defer resp.Body.Close()
	if q.MaxBytes > 0 && aws.Int64Value(resp.ContentLength) > q.MaxBytes {
		return readlimit.ErrTooLarge
	}

	bytes, err := readlimit.ReadAll(resp.Body, q.MaxBytes)
	if err != nil {
		return wrap(err, "failed to read blob body '%s' from S3", q.Key)
	}

	q.Result = &dto.Blob{
		Content:     bytes,
		ContentType: aws.StringValue(resp.ContentType),
		Size:        int64(len(bytes)),
	}
	return nil
}

func storeBlob(ctx context.Context, c *cmd.StoreBlob) error {
	reader := bytes.NewReader(c.Content)
	_, err := DefaultClient.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(env.Config.BlobStorage.S3.BucketName),
		Key:         aws.String(keyFullPathURL(ctx, c.Key)),
		ContentType: aws.String(c.ContentType),
		ACL:         aws.String(s3.ObjectCannedACLPrivate),
		Body:        reader,
	})
	if err != nil {
		return wrap(err, "failed to upload blob '%s' to S3", c.Key)
	}
	return nil
}

func deleteBlob(ctx context.Context, c *cmd.DeleteBlob) error {
	_, err := DefaultClient.DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(env.Config.BlobStorage.S3.BucketName),
		Key:    aws.String(keyFullPathURL(ctx, c.Key)),
	})
	if err != nil && !isNotFound(err) {
		return wrap(err, "failed to delete blob '%s' from S3", c.Key)
	}
	return nil
}

func keyFullPathURL(ctx context.Context, key string) string {
	return basePath(ctx, "") + key
}

func basePath(ctx context.Context, segment string) string {
	tenant, ok := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	if ok {
		return fmt.Sprintf("tenants/%s/%s", strconv.Itoa(tenant.ID), segment)
	}
	return segment
}

func isNotFound(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		return awsErr.Code() == s3.ErrCodeNoSuchKey
	}
	return false
}

func wrap(err error, format string, a ...any) error {
	return errors.Wrap(err, format, a...)
}
