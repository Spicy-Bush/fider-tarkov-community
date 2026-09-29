package handlers

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image/color"
	"image/png"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/assets"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/readlimit"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/crypto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/imagic"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/log"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/blob"
	"github.com/goenning/letteravatar"
)

func isValidBlobKey(key string) bool {
	return blob.ValidateKey(key) == nil
}

func LetterAvatar() web.HandlerFunc {
	return func(c *web.Context) error {
		id := c.Param("id")
		name := c.Param("name")
		if name == "" {
			name = "?"
		}

		size, err := c.QueryParamAsInt("size")
		if err != nil {
			return c.BadRequest(web.Map{})
		}
		size = between(size, 50, 200)

		img, err := letteravatar.Draw(size, strings.ToUpper(letteravatar.Extract(name)), &letteravatar.Options{
			PaletteKey: fmt.Sprintf("%s:%s", id, name),
		})
		if err != nil {
			return c.Failure(err)
		}

		buf := new(bytes.Buffer)
		err = png.Encode(buf, img)
		if err != nil {
			return c.Failure(err)
		}

		return c.Image("image/png", buf.Bytes())
	}
}

func Gravatar() web.HandlerFunc {
	gravatarClient := &http.Client{Timeout: 10 * time.Second}

	return func(c *web.Context) error {
		id, err := c.ParamAsInt("id")
		if err != nil {
			return c.NotFound()
		}

		size, err := c.QueryParamAsInt("size")
		if err != nil {
			return c.BadRequest(web.Map{})
		}

		size = between(size, 50, 200)

		if id > 0 {
			userByID := &query.GetUserByID{UserID: id}
			err := bus.Dispatch(c, userByID)
			if err == nil && userByID.Result.Tenant.ID == c.Tenant().ID {
				if userByID.Result.Email != "" {
					url := fmt.Sprintf("https://www.gravatar.com/avatar/%s?s=%d&d=404", crypto.MD5(strings.ToLower(userByID.Result.Email)), size)
					cacheKey := fmt.Sprintf("gravatar:%s", url)

					if image, found := c.Engine().Cache().Get(cacheKey); found {
						log.Debugf(c, "Gravatar found in cache: @{GravatarURL}", dto.Props{
							"GravatarURL": cacheKey,
						})
						imageInBytes := image.([]byte)
						return c.Image(http.DetectContentType(imageInBytes), imageInBytes)
					}

					log.Debugf(c, "Requesting gravatar: @{GravatarURL}", dto.Props{
						"GravatarURL": url,
					})

					resp, err := gravatarClient.Get(url)
					if err == nil {
						defer resp.Body.Close()
						if resp.StatusCode == http.StatusOK {
							bytes, err := readlimit.ReadAll(resp.Body, imagic.MaxImageBytes)
							if err == nil {
								c.Engine().Cache().Set(cacheKey, bytes, 24*time.Hour)
								return c.Image(http.DetectContentType(bytes), bytes)
							}
						}
					}
				}
			}
		}

		return LetterAvatar()(c)
	}
}

var faviconSizeBuckets = []int{64, 100, 200, 512}

func authorizeImage(c *web.Context, key string) error {
	ownerImage := strings.HasPrefix(key, "attachments/") || strings.HasPrefix(key, "pages/")
	if strings.HasPrefix(key, "files/") {
		c.Response.Header().Set("Cache-Control", "private, no-store")
	}

	if !ownerImage {
		return nil
	}
	c.Response.Header().Set("Cache-Control", "private, no-cache")

	access := &query.CanReadAttachment{Key: key}
	if err := bus.Dispatch(c, access); err != nil {
		return err
	}

	if access.Result {
		if access.Version != "" {
			identity := fmt.Sprintf("%d:%s:%s", c.Tenant().ID, access.Version, c.Request.URL.RequestURI())
			etag := sha256.Sum256([]byte(identity))
			c.Response.Header().Set("ETag", fmt.Sprintf(`W/"%x"`, etag))
		}
		return nil
	}

	if entity.Can(c.User(), c.Tenant(), entity.ManageFiles) {
		file := &query.GetMediaFile{BlobKey: key}
		if err := bus.Dispatch(c, file); err != nil {
			return err
		}

		if file.Result.State != "ready" {
			return app.ErrNotFound
		}

		return nil
	}

	return app.ErrNotFound
}

func imageNotModified(c *web.Context) bool {
	etag := c.Response.Header().Get("ETag")
	if etag == "" {
		return false
	}

	for _, candidate := range strings.Split(c.Request.GetHeader("If-None-Match"), ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || strings.TrimPrefix(candidate, "W/") == strings.TrimPrefix(etag, "W/") {
			return true
		}
	}
	return false
}

func Favicon() web.HandlerFunc {
	defaultFavicon, _ := fs.ReadFile(assets.FS, "favicon.png")

	return func(c *web.Context) error {
		bkey := c.Param("bkey")
		bg := c.QueryParam("bg")
		if bkey != "" && !isValidBlobKey(bkey) {
			return c.NotFound()
		}

		if err := authorizeImage(c, bkey); err != nil {
			return c.Failure(err)
		}

		requestedSize, err := c.QueryParamAsInt("size")
		if err != nil {
			return c.BadRequest(web.Map{})
		}

		canonicalSize := snapToSize(requestedSize, faviconSizeBuckets)
		if canonicalSize == 0 {
			canonicalSize = 64
		}

		if requestedSize != canonicalSize {
			redirectURL := url.URL{Path: "/static/favicon"}
			if bkey != "" {
				redirectURL.Path += "/" + bkey
			}
			redirectURL.RawQuery = fmt.Sprintf("size=%d", canonicalSize)
			if bg != "" {
				redirectURL.RawQuery += "&bg=white"
			}
			return c.PermanentRedirect(redirectURL.String())
		}
		if imageNotModified(c) {
			return c.NoContent(http.StatusNotModified)
		}

		var faviconBytes []byte

		if bkey != "" {
			q := &query.GetBlobByKey{Key: bkey, MaxBytes: imagic.MaxImageBytes}
			err := bus.Dispatch(c, q)
			if err != nil {
				return c.Failure(err)
			}
			faviconBytes = q.Result.Content
		} else {
			faviconBytes = defaultFavicon
		}

		opts := []imagic.ImageOperation{
			imagic.Padding(canonicalSize * 10 / 100),
			imagic.Resize(canonicalSize),
		}

		if bg != "" {
			opts = append(opts, imagic.ChangeBackground(color.White))
		}

		faviconBytes, err = imagic.Apply(faviconBytes, opts...)
		if err != nil {
			return c.Failure(err)
		}

		return c.Image("image/webp", faviconBytes)
	}
}

func ViewUploadedImage() web.HandlerFunc {
	return func(c *web.Context) error {
		bkey := c.Param("bkey")

		if !isValidBlobKey(bkey) {
			return c.NotFound()
		}

		if err := authorizeImage(c, bkey); err != nil {
			return c.Failure(err)
		}

		requestedSize, err := c.QueryParamAsInt("size")
		if err != nil {
			return c.BadRequest(web.Map{})
		}

		canonicalSize := snapToSize(requestedSize, imageSizeBuckets)
		if requestedSize != canonicalSize {
			redirectURL := url.URL{Path: "/static/images/" + bkey}
			if canonicalSize > 0 {
				redirectURL.RawQuery = fmt.Sprintf("size=%d", canonicalSize)
			}
			return c.PermanentRedirect(redirectURL.String())
		}
		if imageNotModified(c) {
			return c.NoContent(http.StatusNotModified)
		}

		if canonicalSize == 200 || canonicalSize == 512 {
			thumbnail := &query.GetMediaThumbnail{Key: bkey, Size: canonicalSize}
			if err := bus.Dispatch(c, thumbnail); err != nil {
				return c.Failure(err)
			}
			return c.Image(thumbnail.Result.ContentType, thumbnail.Result.Content)
		}

		q := &query.GetBlobByKey{Key: bkey, MaxBytes: imagic.MaxImageBytes}
		err = bus.Dispatch(c, q)
		if err != nil {
			return c.Failure(err)
		}

		imgBytes := q.Result.Content
		if canonicalSize > 0 {
			metadata, err := imagic.Parse(imgBytes)
			if err != nil {
				return c.Failure(err)
			}

			if metadata.Width > canonicalSize || metadata.Height > canonicalSize {
				imgBytes, err = imagic.Apply(imgBytes, imagic.Resize(canonicalSize))
				if err != nil {
					return c.Failure(err)
				}

				return c.Image("image/webp", imgBytes)
			}
		}

		return c.Image(q.Result.ContentType, imgBytes)
	}
}

func AdminMediaThumbnail() web.HandlerFunc {
	return func(c *web.Context) error {
		c.Response.Header().Set("Cache-Control", "private, no-store")
		if !entity.Can(c.User(), c.Tenant(), entity.ManageFiles) {
			return c.NotFound()
		}
		key := c.QueryParam("key")
		size, err := c.QueryParamAsInt("size")
		if err != nil || (size != 200 && size != 512) || !isValidBlobKey(key) {
			return c.BadRequest(web.Map{})
		}
		thumbnail := &query.GetMediaThumbnail{
			Key:                    key,
			Size:                   size,
			AllowUnpublishedAvatar: true,
		}
		if err := bus.Dispatch(c, thumbnail); err != nil {
			return c.Failure(err)
		}
		return c.Image(thumbnail.Result.ContentType, thumbnail.Result.Content)
	}
}
