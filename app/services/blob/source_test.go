package blob_test

import (
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/blob"
)

func TestStorageSourceTracksLocationWithoutCredentials(t *testing.T) {
	previous := env.Config.BlobStorage
	t.Cleanup(func() { env.Config.BlobStorage = previous })
	env.Config.BlobStorage.Type = "sql"
	if blob.StorageSource() != "sql" {
		t.Fatal("SQL inventory source changed")
	}

	env.Config.BlobStorage.Type = "fs"
	env.Config.BlobStorage.FS.Path = "/tmp/source-one"
	first := blob.StorageSource()
	env.Config.BlobStorage.FS.Path = "/tmp/source-two"
	if first == blob.StorageSource() {
		t.Fatal("different filesystem roots shared inventory")
	}

	env.Config.BlobStorage.Type = "s3"
	env.Config.BlobStorage.S3.EndpointURL = "http://s3.example"
	env.Config.BlobStorage.S3.BucketName = "one"
	first = blob.StorageSource()
	env.Config.BlobStorage.S3.AccessKeyID = "rotated"
	env.Config.BlobStorage.S3.SecretAccessKey = "rotated"
	if first != blob.StorageSource() {
		t.Fatal("credential rotation restarted inventory")
	}
	env.Config.BlobStorage.S3.BucketName = "two"
	if first == blob.StorageSource() {
		t.Fatal("different buckets shared inventory")
	}
}
