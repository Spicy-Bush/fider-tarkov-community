package s3

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
)

func TestS3InventoryResumesWithoutObjectReads(t *testing.T) {
	calls := 0
	ctx := s3Fixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodGet || r.URL.Query().Get("list-type") != "2" ||
			r.URL.Query().Get("max-keys") != "200" || r.URL.Query().Get("continuation-token") != "saved" {
			t.Errorf("unexpected provider request: %s %s", r.Method, r.URL.RequestURI())
		}
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, `<ListBucketResult><IsTruncated>false</IsTruncated>
            <Contents><Key>tenants/1/files/existing</Key><Size>987654</Size>
            <LastModified>2026-09-28T01:00:00Z</LastModified></Contents></ListBucketResult>`)
	})
	pages := 0
	request := &query.ScanBlobMetadata{Cursor: "saved", BatchSize: 200}
	request.Accept = func(files []dto.BlobMetadata, next string, complete bool) error {
		pages++
		if len(files) != 1 || files[0].Key != "files/existing" || files[0].Size != 987654 || !complete {
			t.Fatalf("wrong inventory page: %+v complete=%t", files, complete)
		}
		return nil
	}
	if err := backend.Scan(ctx, request); err != nil || calls != 1 || pages != 1 {
		t.Fatalf("calls=%d pages=%d err=%v", calls, pages, err)
	}
}

func TestS3InventoryStopsAfterRejectedBatch(t *testing.T) {
	calls := 0
	ctx := s3Fixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, `<ListBucketResult><IsTruncated>true</IsTruncated>
            <NextContinuationToken>next</NextContinuationToken></ListBucketResult>`)
	})
	stop := errors.New("inventory refreshed")
	request := &query.ScanBlobMetadata{
		BatchSize: 200,
		Accept: func(files []dto.BlobMetadata, next string, complete bool) error {
			return stop
		},
	}
	if err := backend.Scan(ctx, request); !errors.Is(err, stop) || calls != 1 {
		t.Fatalf("rejected batch continued: calls=%d err=%v", calls, err)
	}
}

func TestS3InventoryAdvancesPastInvalidStoredNames(t *testing.T) {
	calls := 0
	ctx := s3Fixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/xml")
		if r.URL.Query().Get("continuation-token") == "" {
			fmt.Fprint(w, `<ListBucketResult><IsTruncated>true</IsTruncated>
                <NextContinuationToken>second</NextContinuationToken>
                <Contents><Key>tenants/1/old file.webp</Key><Size>7</Size></Contents></ListBucketResult>`)
			return
		}
		fmt.Fprint(w, `<ListBucketResult><IsTruncated>false</IsTruncated>
            <Contents><Key>tenants/1/valid.webp</Key><Size>7</Size></Contents></ListBucketResult>`)
	})

	seen, complete := 0, false
	scan := &query.ScanBlobMetadata{BatchSize: 1}
	scan.Accept = func(files []dto.BlobMetadata, next string, done bool) error {
		for _, file := range files {
			if file.Key != "valid.webp" {
				t.Fatalf("invalid file reached catalog: %q", file.Key)
			}
			seen++
		}
		complete = done
		return nil
	}
	if err := backend.Scan(ctx, scan); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || seen != 1 || scan.Skipped != 1 || !complete {
		t.Fatalf("calls=%d seen=%d skipped=%d complete=%t", calls, seen, scan.Skipped, complete)
	}
}
