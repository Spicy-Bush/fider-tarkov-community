package fs

import (
	"context"
	"io"
	"mime"
	"os"
	"path"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
)

func scanBlobMetadata(ctx context.Context, q *query.ScanBlobMetadata) error {
	root, err := openRoot(ctx, false)
	if os.IsNotExist(err) {
		return q.Accept(nil, "", true)
	}
	if err != nil {
		return err
	}
	defer root.Close()

	batch := make([]dto.BlobMetadata, 0, q.BatchSize)
	var scanDirectory func(string) error
	scanDirectory = func(directory string) error {
		file, err := root.Open(directory)
		if err != nil {
			return err
		}
		defer file.Close()

		for {
			if err := ctx.Err(); err != nil {
				return err
			}
			entries, readErr := file.ReadDir(q.BatchSize)
			for _, entry := range entries {
				key := path.Join(directory, entry.Name())
				if entry.IsDir() {
					if entry.Name() == stagingDirectory {
						continue
					}
					if err := scanDirectory(key); err != nil {
						return err
					}
					continue
				}
				if !entry.Type().IsRegular() {
					continue
				}
				info, err := entry.Info()
				if err != nil {
					return err
				}
				contentType := mime.TypeByExtension(path.Ext(key))
				if contentType == "" {
					contentType = "application/octet-stream"
				}
				batch = append(batch, dto.BlobMetadata{
					Key:         key,
					ContentType: contentType,
					Size:        info.Size(),
					ModifiedAt:  info.ModTime(),
				})
				if len(batch) == q.BatchSize {
					if err := q.Accept(batch, "", false); err != nil {
						return err
					}
					batch = batch[:0]
				}
			}
			if readErr == io.EOF {
				return nil
			}
			if readErr != nil {
				return readErr
			}
		}
	}
	if err := scanDirectory("."); err != nil {
		return err
	}
	return q.Accept(batch, "", true)
}
