package blob

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
)

type Capabilities struct {
	TransactionalWrites bool
	ResumableScan       bool
}

func StorageCapabilities() Capabilities {
	switch env.Config.BlobStorage.Type {
	case "sql":
		return Capabilities{TransactionalWrites: true, ResumableScan: true}
	case "fs":
		return Capabilities{}
	case "s3":
		return Capabilities{ResumableScan: true}
	default:
		panic(fmt.Sprintf("Unsupported blob storage provider %q", env.Config.BlobStorage.Type))
	}
}

func StorageSource() string {
	storage := env.Config.BlobStorage
	if storage.Type == "sql" {
		return "sql"
	}

	var location string
	switch storage.Type {
	case "fs":
		var err error
		location, err = filepath.Abs(storage.FS.Path)
		if err != nil {
			panic(err)
		}
	case "s3":
		location = strings.Join([]string{
			storage.S3.EndpointURL,
			storage.S3.Region,
			storage.S3.BucketName,
		}, "\x00")
	default:
		panic(fmt.Sprintf("Unsupported blob storage provider %q", storage.Type))
	}
	digest := sha256.Sum256([]byte(location))
	return storage.Type + ":" + hex.EncodeToString(digest[:])
}
