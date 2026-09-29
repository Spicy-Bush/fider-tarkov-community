package s3

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	awss3 "github.com/aws/aws-sdk-go/service/s3"
)

func s3Fixture(t *testing.T, handler http.HandlerFunc) context.Context {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	previousClient := DefaultClient
	previousBucket := env.Config.BlobStorage.S3.BucketName
	awsSession := session.Must(session.NewSession(&aws.Config{
		Endpoint: aws.String(server.URL),
		Region: aws.String("test"),
		Credentials: credentials.NewStaticCredentials("test", "test", ""),
		S3ForcePathStyle: aws.Bool(true),
		MaxRetries: aws.Int(0),
	}))
	DefaultClient = awss3.New(awsSession)
	env.Config.BlobStorage.S3.BucketName = "test"
	t.Cleanup(func() {
		DefaultClient = previousClient
		env.Config.BlobStorage.S3.BucketName = previousBucket
	})
	return context.WithValue(context.Background(), app.TenantCtxKey, &entity.Tenant{ID: 1})
}

func TestS3ListingPages(t *testing.T) {
	calls := 0
	ctx := s3Fixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("prefix") != "tenants/1/images/" {
			t.Errorf("wrong scope: %s", r.URL.Query().Get("prefix"))
		}
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, "<ListBucketResult>")
		if r.URL.Query().Get("continuation-token") == "" {
			fmt.Fprint(w, "<IsTruncated>true</IsTruncated><NextContinuationToken>second</NextContinuationToken>")
			for id := 0; id < 1000; id++ {
				fmt.Fprintf(w, "<Contents><Key>tenants/1/images/%04d</Key></Contents>", id)
			}
		} else {
			if r.URL.Query().Get("continuation-token") != "second" {
				t.Error("continuation token changed")
			}
			fmt.Fprint(w, "<IsTruncated>false</IsTruncated><Contents><Key>tenants/1/images/1000</Key></Contents>")
		}
		fmt.Fprint(w, "</ListBucketResult>")
	})

	q := &query.ListBlobs{Prefix: "images/"}
	if err := backend.List(ctx, q); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(q.Result) != 1001 || q.Result[0] != "images/0000" || q.Result[1000] != "images/1000" {
		t.Fatalf("pagination calls=%d files=%d", calls, len(q.Result))
	}
}

func TestS3ListingRejectsIncompleteResults(t *testing.T) {
	for _, failure := range []string{"later failure", "missing token", "repeated token", "foreign key"} {
		t.Run(failure, func(t *testing.T) {
			calls := 0
			ctx := s3Fixture(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/xml")
				if failure == "later failure" && calls == 2 {
					w.WriteHeader(http.StatusServiceUnavailable)
					fmt.Fprint(w, "<Error><Code>SlowDown</Code><Message>Fixture unavailable</Message></Error>")
					return
				}
				if failure == "foreign key" {
					fmt.Fprint(w, "<ListBucketResult><IsTruncated>false</IsTruncated><Contents><Key>tenants/2/images/private</Key></Contents></ListBucketResult>")
					return
				}
				fmt.Fprint(w, "<ListBucketResult><IsTruncated>true</IsTruncated>")
				if failure != "missing token" {
					fmt.Fprint(w, "<NextContinuationToken>next</NextContinuationToken>")
				}
				fmt.Fprint(w, "<Contents><Key>tenants/1/images/first</Key></Contents></ListBucketResult>")
			})

			q := &query.ListBlobs{Prefix: "images/", Result: []string{"stale"}}
			if err := backend.List(ctx, q); err == nil {
				t.Fatal("incomplete listing returned success")
			}
			if q.Result != nil || calls > 2 {
				t.Fatalf("partial results=%v requests=%d", q.Result, calls)
			}
		})
	}
}

func TestS3RejectsInvalidKeysBeforeRequest(t *testing.T) {
	calls := 0
	ctx := s3Fixture(t, func(w http.ResponseWriter, r *http.Request) { calls++ })
	for _, key := range []string{"../escape", "a/../b", "a//b", "a\\b", "/absolute", "a\x00b"} {
		if err := backend.Get(ctx, &query.GetBlobByKey{Key: key}); err == nil {
			t.Errorf("invalid read accepted: %q", key)
		}
		if err := backend.Store(ctx, &cmd.StoreBlob{Key: key}); err == nil {
			t.Errorf("invalid write accepted: %q", key)
		}
		if err := backend.Delete(ctx, &cmd.DeleteBlob{Key: key}); err == nil {
			t.Errorf("invalid delete accepted: %q", key)
		}
	}
	if calls != 0 {
		t.Fatalf("invalid keys reached S3: requests=%d", calls)
	}
}

func TestS3GlobalListingExcludesTenantObjects(t *testing.T) {
	s3Fixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, `<ListBucketResult><IsTruncated>false</IsTruncated>
            <Contents><Key>etc/certificate</Key></Contents>
            <Contents><Key>tenants/1/private</Key></Contents>
            </ListBucketResult>`)
	})
	q := &query.ListBlobs{}
	if err := backend.List(context.Background(), q); err != nil || strings.Join(q.Result, ",") != "etc/certificate" {
		t.Fatalf("global listing=%v err=%v", q.Result, err)
	}
}
