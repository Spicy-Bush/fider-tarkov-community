package handlers_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/middlewares"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/mock"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/blob"
)

func TestImageRedirectPreservesLiteralStorageKey(t *testing.T) {
	key := "files/literal?#percent%25.png"
	for _, handler := range []web.HandlerFunc{handlers.ViewUploadedImage(), handlers.Favicon()} {
		status, response := mock.NewServer().
			OnTenant(mock.DemoTenant).
			AsUser(&entity.User{ID: 1, Role: enum.RoleAdministrator, Status: enum.UserActive}).
			WithURL("http://localhost/static/images/placeholder?size=201").
			AddParam("bkey", key).
			Execute(handler)

		location, err := url.Parse(response.Header().Get("Location"))
		if err != nil || status != http.StatusMovedPermanently {
			t.Fatalf("redirect status=%d location=%v error=%v", status, location, err)
		}
		if !strings.HasSuffix(location.Path, key) || location.Fragment != "" || location.Query().Get("size") != "512" {
			t.Fatalf("redirect changed the storage key: %s", location)
		}
	}
}

func TestAdministrativeImagesAreNotPubliclyCached(t *testing.T) {
	bus.Reset()
	bus.AddHandler(func(ctx context.Context, q *query.GetBlobByKey) error {
		if err := blob.AuthorizeRead(ctx, q); err != nil {
			return err
		}
		q.Result = &dto.Blob{ContentType: "image/png", Content: []byte("fixture")}
		return nil
	})

	admin := &entity.User{ID: 1, Role: enum.RoleAdministrator, Status: enum.UserActive}
	for _, test := range []struct {
		name string
		user *entity.User
		url string
		status int
	}{
		{"original", admin, "http://localhost/static/images/files/private", http.StatusOK},
		{"redirect", admin, "http://localhost/static/images/files/private?size=201", http.StatusMovedPermanently},
		{"anonymous", nil, "http://localhost/static/images/files/private", http.StatusNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			status, response := mock.NewServer().
				OnTenant(mock.DemoTenant).
				AsUser(test.user).
				WithURL(test.url).
				AddHeader("Accept", "application/json").
				AddParam("bkey", "files/private").
				Use(middlewares.ClientCache(30 * 24 * time.Hour)).
				Execute(handlers.ViewUploadedImage())
			cache := response.Header().Get("Cache-Control")
			if status != test.status || !strings.Contains(cache, "no-store") || strings.Contains(cache, "public") {
				t.Fatalf("status=%d cache=%q", status, cache)
			}
		})
	}
}

func TestAttachmentThumbnailChecksCurrentVisibility(t *testing.T) {
	bus.Reset()
	visible := true
	reads := 0
	bus.AddHandler(func(ctx context.Context, q *query.CanReadAttachment) error {
		q.Result = visible
		return nil
	})
	bus.AddHandler(func(ctx context.Context, q *query.GetMediaThumbnail) error {
		reads++
		if q.AllowUnpublishedAvatar || q.Size != 200 || q.Key != "attachments/private" {
			t.Fatal("public request changed thumbnail authority")
		}
		q.Result = &dto.Blob{ContentType: "image/webp", Content: []byte("stored thumbnail")}
		return nil
	})

	for _, expected := range []int{http.StatusOK, http.StatusNotFound} {
		status, response := mock.NewServer().
			OnTenant(mock.DemoTenant).
			WithURL("http://localhost/static/images/attachments/private?size=200").
			AddHeader("Accept", "application/json").
			AddParam("bkey", "attachments/private").
			Use(middlewares.ClientCache(30 * 24 * time.Hour)).
			Execute(handlers.ViewUploadedImage())
		if status != expected || !strings.Contains(response.Header().Get("Cache-Control"), "no-cache") {
			t.Fatalf("visibility changed: status=%d headers=%v", status, response.Header())
		}
		visible = false
	}
	if reads != 1 {
		t.Fatalf("hidden attachment reached thumbnail store: reads=%d", reads)
	}
}

func TestCatalogedOrphanAttachmentOriginalRequiresFileManagement(t *testing.T) {
	bus.Reset()
	state := "ready"
	reads := 0
	bus.AddHandler(func(ctx context.Context, q *query.CanReadAttachment) error {
		q.Result = false
		return nil
	})
	bus.AddHandler(func(ctx context.Context, q *query.GetMediaFile) error {
		q.Result = &dto.FileInfo{BlobKey: q.BlobKey, State: state}
		return nil
	})
	bus.AddHandler(func(ctx context.Context, q *query.GetBlobByKey) error {
		reads++
		q.Result = &dto.Blob{ContentType: "image/webp", Content: []byte("original")}
		return nil
	})

	admin := &entity.User{ID: 1, Role: enum.RoleAdministrator, Status: enum.UserActive}
	visitor := &entity.User{ID: 2, Role: enum.RoleVisitor, Status: enum.UserActive}
	for _, test := range []struct {
		user   *entity.User
		state  string
		status int
	}{
		{admin, "ready", http.StatusOK},
		{admin, "pending", http.StatusNotFound},
		{visitor, "ready", http.StatusNotFound},
		{nil, "ready", http.StatusNotFound},
	} {
		state = test.state
		status, response := mock.NewServer().
			OnTenant(mock.DemoTenant).
			AsUser(test.user).
			WithURL("http://localhost/static/images/attachments/orphan").
			AddHeader("Accept", "application/json").
			AddParam("bkey", "attachments/orphan").
			Execute(handlers.ViewUploadedImage())
		if status != test.status || !strings.Contains(response.Header().Get("Cache-Control"), "no-cache") {
			t.Fatalf("state=%s status=%d headers=%v", state, status, response.Header())
		}
	}
	if reads != 1 {
		t.Fatalf("unauthorized or pending asset reached byte storage: reads=%d", reads)
	}
}

func TestImageRevalidationChecksVisibilityBeforeReturningNotModified(t *testing.T) {
	for _, size := range []string{"", "?size=200"} {
		t.Run("size"+size, func(t *testing.T) {
			bus.Reset()
			visible, version := true, "first"
			checks, reads := 0, 0
			bus.AddHandler(func(ctx context.Context, q *query.CanReadAttachment) error {
				checks++
				q.Result, q.Version = visible, version
				return nil
			})
			bus.AddHandler(func(ctx context.Context, q *query.GetBlobByKey) error {
				reads++
				q.Result = &dto.Blob{ContentType: "image/webp", Content: []byte(version)}
				return nil
			})
			bus.AddHandler(func(ctx context.Context, q *query.GetMediaThumbnail) error {
				reads++
				q.Result = &dto.Blob{ContentType: "image/webp", Content: []byte(version)}
				return nil
			})

			request := func(etag string, wantStatus int, wantReads int) string {
				t.Helper()
				status, response := mock.NewServer().
					OnTenant(mock.DemoTenant).
					WithURL("http://localhost/static/images/attachments/cached" + size).
					AddHeader("Accept", "application/json").
					AddHeader("If-None-Match", etag).
					AddParam("bkey", "attachments/cached").
					Use(middlewares.ClientCache(30 * 24 * time.Hour)).
					Execute(handlers.ViewUploadedImage())
				if status != wantStatus || reads != wantReads {
					t.Fatalf("status=%d reads=%d, want status=%d reads=%d", status, reads, wantStatus, wantReads)
				}
				if status < 400 && response.Header().Get("Cache-Control") != "private, no-cache" {
					t.Fatalf("cache permits stale private images: %v", response.Header())
				}
				if status == http.StatusNotModified && response.Body.Len() != 0 {
					t.Fatal("304 returned image bytes")
				}
				return response.Header().Get("ETag")
			}

			etag := request("", http.StatusOK, 1)
			if etag == "" {
				t.Fatal("image response has no validator")
			}
			request(etag, http.StatusNotModified, 1)
			request(`"unrelated", `+strings.TrimPrefix(etag, "W/"), http.StatusNotModified, 1)

			visible = false
			request(etag, http.StatusNotFound, 1)

			visible, version = true, "replacement"
			replacement := request(etag, http.StatusOK, 2)
			if replacement == etag {
				t.Fatal("replacement reused the previous image validator")
			}
			request(replacement, http.StatusNotModified, 2)
			if checks != 6 {
				t.Fatalf("revalidation bypassed visibility: checks=%d", checks)
			}
		})
	}
}

func TestImageValidatorsDoNotSkipKeyValidation(t *testing.T) {
	bus.Reset()
	bus.AddHandler(func(ctx context.Context, q *query.CanReadAttachment) error {
		t.Fatal("invalid image key reached authorization")
		return nil
	})

	for _, handler := range []web.HandlerFunc{handlers.Favicon(), handlers.ViewUploadedImage()} {
		status, _ := mock.NewServer().
			OnTenant(mock.DemoTenant).
			WithURL("http://localhost/static/images/invalid?size=200").
			AddHeader("If-None-Match", "*").
			AddParam("bkey", "attachments/../invalid").
			Execute(handler)
		if status != http.StatusNotFound {
			t.Fatalf("invalid key returned status %d", status)
		}
	}
}

func TestAdminThumbnailPermissionAndInput(t *testing.T) {
	bus.Reset()
	reads := 0
	bus.AddHandler(func(ctx context.Context, q *query.GetMediaThumbnail) error {
		reads++
		if !q.AllowUnpublishedAvatar || q.Key != "avatars/pending" || q.Size != 200 {
			return app.ErrNotFound
		}
		q.Result = &dto.Blob{ContentType: "image/webp", Content: []byte("stored thumbnail")}
		return nil
	})

	for _, role := range []enum.Role{enum.RoleVisitor, enum.RoleHelper, enum.RoleModerator, enum.RoleCollaborator, enum.RoleAdministrator} {
		actor := &entity.User{ID: 1, Role: role, Status: enum.UserActive}
		status, response := mock.NewServer().
			OnTenant(mock.DemoTenant).
			AsUser(actor).
			WithURL("http://localhost/api/admin/files/thumbnail?key=avatars/pending&size=200").
			AddHeader("Accept", "application/json").
			Execute(handlers.AdminMediaThumbnail())
		want := http.StatusNotFound
		if role == enum.RoleAdministrator {
			want = http.StatusOK
		}
		if status != want || !strings.Contains(response.Header().Get("Cache-Control"), "no-store") {
			t.Fatalf("role=%s status=%d", role, status)
		}
	}

	for _, parameters := range []string{"key=../private&size=200", "key=avatars/pending&size=201", "key=avatars/pending&size=nope", "key=avatars/pending&size=0"} {
		status, _ := mock.NewServer().
			OnTenant(mock.DemoTenant).
			AsUser(&entity.User{ID: 1, Role: enum.RoleAdministrator, Status: enum.UserActive}).
			WithURL("http://localhost/api/admin/files/thumbnail?" + parameters).
			AddHeader("Accept", "application/json").
			Execute(handlers.AdminMediaThumbnail())
		if status != http.StatusBadRequest {
			t.Fatalf("invalid parameters %q: status=%d", parameters, status)
		}
	}
	if reads != 1 {
		t.Fatalf("unauthorized or invalid request reached thumbnail store: reads=%d", reads)
	}
}
