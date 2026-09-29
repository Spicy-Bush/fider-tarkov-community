package handlers_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/crypto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/mock"
)

type avatarTransport func(*http.Request) (*http.Response, error)

func (transport avatarTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func useAvatarTransport(t *testing.T, transport avatarTransport) {
	t.Helper()
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() {
		http.DefaultTransport = previous
	})
}

func TestGravatarHandler(t *testing.T) {
	server := mock.NewServer().OnTenant(mock.DemoTenant)
	user := &entity.User{ID: 3, Email: "DarthVader.Fider@gmail.com", Tenant: mock.DemoTenant}
	bus.AddHandler(func(ctx context.Context, q *query.GetUserByID) error {
		q.Result = user
		return nil
	})

	avatar := image.NewRGBA(image.Rect(0, 0, 64, 64))
	avatar.Set(32, 32, color.White)
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, avatar); err != nil {
		t.Fatal(err)
	}

	requests := 0
	useAvatarTransport(t, func(request *http.Request) (*http.Response, error) {
		requests++
		wantPath := "/avatar/" + crypto.MD5("darthvader.fider@gmail.com")
		if request.URL.Host != "www.gravatar.com" || request.URL.Path != wantPath || request.URL.Query().Get("s") != "64" || request.URL.Query().Get("d") != "404" {
			t.Fatalf("unexpected avatar request: %s", request.URL)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(encoded.Bytes())),
			Header:     make(http.Header),
		}, nil
	})

	code, response := server.WithURL("https://demo.test.fider.io/?size=64").
		AddParam("id", user.ID).AddParam("name", "Darth Vader").Execute(handlers.Gravatar())
	if code != http.StatusOK || response.Header().Get("Content-Type") != "image/png" || !bytes.Equal(response.Body.Bytes(), encoded.Bytes()) {
		t.Fatalf("avatar response: status=%d type=%s bytes=%d", code, response.Header().Get("Content-Type"), response.Body.Len())
	}

	if requests != 1 {
		t.Fatalf("provider requests = %d, want 1", requests)
	}

	response.Body.Reset()
	code, response = server.Execute(handlers.Gravatar())
	if code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), encoded.Bytes()) || requests != 1 {
		t.Fatalf("cached response: status=%d bytes=%d provider requests=%d", code, response.Body.Len(), requests)
	}
}

func TestGravatarFallback(t *testing.T) {
	for _, scenario := range []struct {
		name           string
		userID         int
		lookupError    error
		tenant         *entity.Tenant
		email          string
		upstreamStatus int
		upstreamError  error
		wantRequests   int
	}{
		{
			name:           "not registered",
			userID:         3,
			tenant:         mock.DemoTenant,
			email:          "missing@example.com",
			upstreamStatus: http.StatusNotFound,
			wantRequests:   1,
		},
		{
			name:           "provider unavailable",
			userID:         3,
			tenant:         mock.DemoTenant,
			email:          "missing@example.com",
			upstreamStatus: http.StatusServiceUnavailable,
			wantRequests:   1,
		},
		{
			name:          "transport unavailable",
			userID:        3,
			tenant:        mock.DemoTenant,
			email:         "missing@example.com",
			upstreamError: errors.New("connection lost"),
			wantRequests:  1,
		},
		{name: "anonymous", userID: 0},
		{name: "missing user", userID: 3, lookupError: app.ErrNotFound},
		{
			name:   "another tenant",
			userID: 3,
			tenant: &entity.Tenant{ID: 999},
			email:  "private@example.com",
		},
		{name: "no email", userID: 3, tenant: mock.DemoTenant},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			server := mock.NewServer().OnTenant(mock.DemoTenant)
			bus.AddHandler(func(ctx context.Context, q *query.GetUserByID) error {
				q.Result = &entity.User{ID: scenario.userID, Email: scenario.email, Tenant: scenario.tenant}
				return scenario.lookupError
			})

			requests := 0
			useAvatarTransport(t, func(request *http.Request) (*http.Response, error) {
				requests++
				if scenario.upstreamError != nil {
					return nil, scenario.upstreamError
				}

				return &http.Response{
					StatusCode: scenario.upstreamStatus,
					Body:       http.NoBody,
					Header:     make(http.Header),
				}, nil
			})

			code, response := server.WithURL("https://demo.test.fider.io/?size=64").
				AddParam("id", scenario.userID).AddParam("name", "Jon Snow").Execute(handlers.Gravatar())
			if code != http.StatusOK || response.Header().Get("Content-Type") != "image/png" {
				t.Fatalf("fallback: status=%d content type=%s", code, response.Header().Get("Content-Type"))
			}

			avatar, err := png.Decode(response.Body)
			if err != nil {
				t.Fatal(err)
			}

			if avatar.Bounds().Dx() != 64 || avatar.Bounds().Dy() != 64 {
				t.Fatalf("fallback size = %v, want 64×64", avatar.Bounds())
			}

			if requests != scenario.wantRequests {
				t.Fatalf("provider requests = %d, want %d", requests, scenario.wantRequests)
			}
		})
	}
}

func TestLetterAvatarSizeAndGlyph(t *testing.T) {
	for _, size := range []struct {
		requested string
		pixels    int
	}{
		{requested: "", pixels: 50},
		{requested: "1", pixels: 50},
		{requested: "64", pixels: 64},
		{requested: "5000", pixels: 200},
	} {
		t.Run(fmt.Sprintf("size=%s", size.requested), func(t *testing.T) {
			code, response := mock.NewServer().WithURL("https://demo.test.fider.io/?size="+size.requested).
				AddParam("id", 1).AddParam("name", "Jon Snow").Execute(handlers.LetterAvatar())
			if code != http.StatusOK {
				t.Fatalf("response = %d", code)
			}

			avatar, err := png.Decode(response.Body)
			if err != nil {
				t.Fatal(err)
			}

			if avatar.Bounds() != image.Rect(0, 0, size.pixels, size.pixels) {
				t.Fatalf("avatar bounds = %v, want %d×%d", avatar.Bounds(), size.pixels, size.pixels)
			}

			background := color.RGBAModel.Convert(avatar.At(0, 0))
			for y := 0; y < size.pixels; y++ {
				for x := 0; x < size.pixels; x++ {
					if color.RGBAModel.Convert(avatar.At(x, y)) != background {
						return
					}
				}
			}
			t.Fatal("avatar contains no visible glyph")
		})
	}
}
