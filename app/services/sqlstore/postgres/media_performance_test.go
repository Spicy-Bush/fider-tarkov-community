package postgres_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
)

func BenchmarkAttachmentReadAuthorization(b *testing.B) {
	f := newPostWorkflow(b)
	post := &cmd.AddNewPost{Title: "Public image owner", Description: "A public post"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		b.Fatal(err)
	}

	const key = "attachments/read-benchmark.webp"
	if _, err := mediaFixtureSQL(`
		INSERT INTO media_assets (tenant_id, key, name, content_type, size)
		VALUES ($1, $2, 'Read benchmark', 'image/webp', 10240);
	`, f.tenant.ID, key); err != nil {
		b.Fatal(err)
	}
	if _, err := mediaFixtureSQL(`
		INSERT INTO attachments (tenant_id, post_id, user_id, attachment_bkey)
		VALUES ($1, $2, $3, $4)
	`, f.tenant.ID, post.Result.ID, f.user.ID, key); err != nil {
		b.Fatal(err)
	}

	for _, count := range []int{1, 50000} {
		requestedKey := key
		if count > 1 {
			for first := 1; first < count; first += 1000 {
				if _, err := mediaFixtureSQL(`
					INSERT INTO attachments (tenant_id, post_id, user_id, attachment_bkey)
					SELECT $1, $2, $3, 'attachments/noise-' || i
					FROM generate_series($4::integer, $5::integer) i
				`, f.tenant.ID, post.Result.ID, f.user.ID, first, min(first+999, count-1)); err != nil {
					b.Fatal(err)
				}
			}
			if _, err := dbx.Connection().Exec("ANALYZE attachments; ANALYZE media_assets"); err != nil {
				b.Fatal(err)
			}
			requestedKey = fmt.Sprintf("attachments/noise-%d", count-1)
		}

		for _, viewer := range []struct {
			name    string
			user    *entity.User
			missing bool
		}{
			{name: "anonymous"},
			{name: "administrator", user: f.user},
			{name: "missing", missing: true},
		} {
			b.Run(fmt.Sprintf("%d/%s", count, viewer.name), func(b *testing.B) {
				ctx := context.WithValue(f.ctx, app.UserCtxKey, viewer.user)
				b.ReportAllocs()
				b.ResetTimer()

				for iteration := 0; iteration < b.N; iteration++ {
					access := &query.CanReadAttachment{Key: requestedKey}
					if viewer.missing {
						access.Key = "attachments/missing.webp"
					}
					if err := bus.Dispatch(ctx, access); err != nil || access.Result == viewer.missing {
						b.Fatalf("public image access=%t error=%v", access.Result, err)
					}
				}
			})
		}
	}
}

func BenchmarkPageImageAuthorization(b *testing.B) {
	f := newPostWorkflow(b)
	for _, count := range []int{1, 10000} {
		first := 1
		if count > 1 {
			first = 2
		}
		for start := first; start <= count; start += 1000 {
			if _, err := mediaFixtureSQL(`
				INSERT INTO pages (
					tenant_id, title, slug, content, status, visibility,
					created_by_id, updated_by_id, banner_image_bkey
				)
				SELECT $1, 'Page ' || i, 'image-read-' || i, 'Page content',
				       'published', 'public', $2, $2, 'pages/read-' || i
				FROM generate_series($3::integer, $4::integer) i
			`, f.tenant.ID, f.user.ID, start, min(start+999, count)); err != nil {
				b.Fatal(err)
			}
		}
		if _, err := dbx.Connection().Exec("ANALYZE pages; ANALYZE media_asset_refs"); err != nil {
			b.Fatal(err)
		}

		for _, key := range []string{fmt.Sprintf("pages/read-%d", count), "pages/missing"} {
			b.Run(fmt.Sprintf("%d/%s", count, key), func(b *testing.B) {
				ctx := context.WithValue(f.ctx, app.UserCtxKey, (*entity.User)(nil))
				b.ReportAllocs()
				b.ResetTimer()
				for iteration := 0; iteration < b.N; iteration++ {
					access := &query.CanReadAttachment{Key: key}
					if err := bus.Dispatch(ctx, access); err != nil || access.Result != (key != "pages/missing") {
						b.Fatalf("Page image access=%t error=%v", access.Result, err)
					}
				}
			})
		}
	}
}

func BenchmarkMediaFileListing(b *testing.B) {
	f := newPostWorkflow(b)
	for _, count := range []int{1000, 10000, 100000} {
		b.Run(fmt.Sprintf("files_%d", count), func(b *testing.B) {
			if _, err := dbx.Connection().Exec("DELETE FROM media_assets WHERE tenant_id=$1", f.tenant.ID); err != nil {
				b.Fatal(err)
			}

			for start := 1; start <= count; start += 500 {
				_, err := dbx.Connection().Exec(`
					INSERT INTO media_assets (tenant_id,key,name,content_type,size)
					SELECT $1, 'files/benchmark-' || lpad(i::text,6,'0'),
					       'Screenshot ' || i, 'image/webp', 10000
					FROM generate_series($2::integer,$3::integer) i
				`, f.tenant.ID, start, min(start+499, count))
				if err != nil {
					b.Fatal(err)
				}
			}
			if _, err := dbx.Connection().Exec("ANALYZE media_assets; ANALYZE media_asset_refs"); err != nil {
				b.Fatal(err)
			}

			for _, test := range []struct {
				name   string
				page   int
				usage  string
				sortBy string
				dir    string
			}{
				{"first", 1, "all", "createdAt", "desc"},
				{"middle", count / 40, "all", "createdAt", "desc"},
				{"middle_name", count / 40, "all", "name", "asc"},
				{"middle_size", count / 40, "all", "size", "desc"},
				{"last", count / 20, "all", "createdAt", "desc"},
				{"unused", 1, "unused", "createdAt", "desc"},
				{"oldest", 1, "all", "createdAt", "asc"},
				{"name_asc", 1, "all", "name", "asc"},
				{"name_desc", 1, "all", "name", "desc"},
				{"size_asc", 1, "all", "size", "asc"},
				{"size_desc", 1, "all", "size", "desc"},
			} {
				b.Run(test.name, func(b *testing.B) {
					b.ReportAllocs()
					for iteration := 0; iteration < b.N; iteration++ {
						request := query.NewListImageFiles()
						request.Page = test.page
						request.Usage = test.usage
						request.SortBy = test.sortBy
						request.SortDir = test.dir
						if err := bus.Dispatch(f.ctx, request); err != nil {
							b.Fatal(err)
						}
						if len(request.Result) != 20 || request.Total != count || request.TotalBytes != int64(count)*10000 {
							b.Fatal("listing changed the measured workload")
						}
					}
				})
			}
		})
	}
}

func BenchmarkMediaCleanupReferences(b *testing.B) {
	f := newPostWorkflow(b)
	for start := 1; start <= 25000; start += 500 {
		_, err := dbx.Connection().Exec(`
			INSERT INTO media_assets (tenant_id,key,name,content_type,size)
			SELECT $1, 'files/reference-' || i, 'Screenshot ' || i, 'image/png', 10000
			FROM generate_series($2::integer,$3::integer) i
		`, f.tenant.ID, start, start+499)
		if err != nil {
			b.Fatal(err)
		}
	}

	for start := 1; start <= 10000; start += 100 {
		_, err := mediaFixtureSQL(`
			INSERT INTO posts (tenant_id,user_id,title,slug,number,status,created_at,description)
			SELECT $1,$2,'Image reference ' || i,'reference-' || i,i,
			       CASE WHEN i%2=0 THEN 1 ELSE 6 END,NOW(),
			       '![own](/static/images/files/reference-' || i ||
			       ') ![shared](/static/images/files/reference-25000)'
			FROM generate_series($3::integer,$4::integer) i
		`, f.tenant.ID, f.user.ID, start, start+99)
		if err != nil {
			b.Fatal(err)
		}
	}
	if _, err := dbx.Connection().Exec("ANALYZE media_assets; ANALYZE media_asset_refs; ANALYZE posts"); err != nil {
		b.Fatal(err)
	}

	for _, includeDeleted := range []bool{false, true} {
		b.Run(fmt.Sprintf("unused_include_deleted_%t", includeDeleted), func(b *testing.B) {
			expected := 14999
			if includeDeleted {
				expected += 5000
			}
			b.ReportAllocs()
			for iteration := 0; iteration < b.N; iteration++ {
				request := query.NewListImageFiles()
				request.Usage = "unused"
				request.IncludeDeleted = includeDeleted
				if err := bus.Dispatch(f.ctx, request); err != nil {
					b.Fatal(err)
				}
				if len(request.Result) != 20 || request.Total != expected {
					b.Fatalf("files=%d total=%d expected=%d", len(request.Result), request.Total, expected)
				}
			}
		})
	}

	b.Run("usage_with_10000_references", func(b *testing.B) {
		b.ReportAllocs()
		for iteration := 0; iteration < b.N; iteration++ {
			request := &query.GetFileUsage{BlobKey: "files/reference-25000", Page: 1}
			if err := bus.Dispatch(f.ctx, request); err != nil {
				b.Fatal(err)
			}
			if len(request.Result) != 50 || request.Total != 10000 {
				b.Fatalf("references=%d total=%d", len(request.Result), request.Total)
			}
		}
	})

	if _, err := mediaFixtureSQL("UPDATE posts SET status=6 WHERE tenant_id=$1", f.tenant.ID); err != nil {
		b.Fatal(err)
	}

	b.Run("shared_image_with_10000_deleted_references", func(b *testing.B) {
		b.ReportAllocs()
		for iteration := 0; iteration < b.N; iteration++ {
			request := query.NewListImageFiles()
			request.Search = "Screenshot 25000"
			request.IncludeDeleted = true
			if err := bus.Dispatch(f.ctx, request); err != nil {
				b.Fatal(err)
			}
			if request.Total != 1 || len(request.Result) != 1 || !request.Result[0].IsInUse || request.Result[0].HasProtectedReferences {
				b.Fatalf("shared image protection changed: %+v", request.Result)
			}
		}
	})
}
