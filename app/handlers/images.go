package handlers

import (
	"bytes"
	"fmt"
	"image/color"
	"image/png"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/assets"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/crypto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/imagic"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/log"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"github.com/goenning/letteravatar"
)

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

		if err == nil && id > 0 {
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

					req := &cmd.HTTPRequest{
						URL:    url,
						Method: "GET",
					}
					err := bus.Dispatch(c, req)
					if err == nil && req.ResponseStatusCode == http.StatusOK {
						bytes := req.ResponseBody
						c.Engine().Cache().Set(cacheKey, bytes, 24*time.Hour)
						return c.Image(http.DetectContentType(bytes), bytes)
					}
				}
			}
		}

		return LetterAvatar()(c)
	}
}

var faviconSizeBuckets = []int{64, 100, 200, 512}

func Favicon() web.HandlerFunc {
	defaultFavicon, _ := fs.ReadFile(assets.FS, "favicon.png")

	return func(c *web.Context) error {
		bkey := c.Param("bkey")
		bg := c.QueryParam("bg")

		requestedSize, err := c.QueryParamAsInt("size")
		if err != nil {
			return c.BadRequest(web.Map{})
		}

		canonicalSize := snapToSize(requestedSize, faviconSizeBuckets)
		if canonicalSize == 0 {
			canonicalSize = 64
		}

		if requestedSize != canonicalSize {
			redirectURL := "/static/favicon"
			if bkey != "" {
				redirectURL = fmt.Sprintf("/static/favicon/%s", bkey)
			}
			redirectURL = fmt.Sprintf("%s?size=%d", redirectURL, canonicalSize)
			if bg != "" {
				redirectURL += "&bg=white"
			}
			return c.PermanentRedirect(redirectURL)
		}

		var (
			faviconBytes []byte
			contentType  string
		)

		if bkey != "" {
			q := &query.GetBlobByKey{Key: bkey}
			err := bus.Dispatch(c, q)
			if err != nil {
				return c.Failure(err)
			}
			faviconBytes = q.Result.Content
			contentType = q.Result.ContentType
		} else {
			faviconBytes = defaultFavicon
			contentType = "image/png"
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

		return c.Image(contentType, faviconBytes)
	}
}

func ViewUploadedImage() web.HandlerFunc {
	return func(c *web.Context) error {
		bkey := c.Param("bkey")

		requestedSize, err := c.QueryParamAsInt("size")
		if err != nil {
			return c.BadRequest(web.Map{})
		}

		canonicalSize := snapToSize(requestedSize, imageSizeBuckets)
		if requestedSize != canonicalSize {
			if canonicalSize == 0 {
				return c.PermanentRedirect(fmt.Sprintf("/static/images/%s", bkey))
			}
			return c.PermanentRedirect(fmt.Sprintf("/static/images/%s?size=%d", bkey, canonicalSize))
		}

		q := &query.GetBlobByKey{Key: bkey}
		err = bus.Dispatch(c, q)
		if err != nil {
			return c.Failure(err)
		}

		imgBytes := q.Result.Content
		if canonicalSize > 0 {
			imgBytes, err = imagic.Apply(imgBytes, imagic.Resize(canonicalSize))
			if err != nil {
				return c.Failure(err)
			}
		}

		return c.Image(q.Result.ContentType, imgBytes)
	}
}
