package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type requestBodyReader struct {
	remaining int64
	read      int64
	err       error
}

func (r *requestBodyReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		if r.err != nil {
			return 0, r.err
		}

		return 0, io.EOF
	}

	n := min(int64(len(p)), r.remaining)
	clear(p[:n])
	r.remaining -= n
	r.read += n
	return int(n), nil
}

func TestRequestBodyLimits(t *testing.T) {
	const bodyLimit = int64(72 * 1024 * 1024)

	for _, test := range []struct {
		name          string
		length, bytes int64
		status        int
		readError     error
	}{
		{"declared too large", bodyLimit + 1, bodyLimit + 1, http.StatusRequestEntityTooLarge, nil},
		{"exact boundary", -1, bodyLimit, http.StatusOK, nil},
		{"stream exceeds limit", -1, bodyLimit + 1, http.StatusRequestEntityTooLarge, nil},
		{"false small length", 1, bodyLimit + 1, http.StatusRequestEntityTooLarge, nil},
		{"interrupted body", 10, 1, http.StatusBadRequest, io.ErrUnexpectedEOF},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := &requestBodyReader{remaining: test.bytes, err: test.readError}
			request := httptest.NewRequest(http.MethodPost, "/upload", body)
			request.ContentLength = test.length
			response := httptest.NewRecorder()
			engine := New()
			called := false
			engine.Post("/upload", func(c *Context) error {
				called = true
				if len(c.Request.Body) != int(test.bytes) || c.Request.ContentLength != test.bytes {
					t.Fatal("accepted request lost body bytes")
				}

				return c.Ok(nil)
			})

			engine.mux.ServeHTTP(response, request)
			if response.Code != test.status || called != (test.status == http.StatusOK) {
				t.Fatalf("status=%d handler called=%v", response.Code, called)
			}

			if body.read > bodyLimit+1 || (test.length > bodyLimit && body.read != 0) {
				t.Fatalf("read %d bytes before rejecting upload", body.read)
			}

			if test.status != http.StatusOK {
				var failure struct {
					Message string `json:"message"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &failure); err != nil || failure.Message == "" {
					t.Fatalf("missing JSON error: body=%s error=%v", response.Body, err)
				}

				if response.Header().Get("Content-Type") != UTF8JSONContentType || response.Header().Get("Cache-Control") != "no-store" {
					t.Fatalf("unexpected error headers: %v", response.Header())
				}
			}
		})
	}
}

func TestRequestBodyFailureRecovery(t *testing.T) {
	engine := New()
	engine.NotFound(func(c *Context) error {
		return c.String(http.StatusNotFound, "missing")
	})

	server := httptest.NewServer(engine.mux)
	defer server.Close()
	client := server.Client()
	client.Transport.(*http.Transport).MaxConnsPerHost = 1

	oversized := &requestBodyReader{remaining: 72*1024*1024 + 1}
	response, err := client.Post(server.URL+"/missing", "application/json", oversized)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()

	if response.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized stream reached not-found handler: %d", response.StatusCode)
	}

	response, err = client.Get(server.URL + "/missing")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusNotFound || string(body) != "missing" {
		t.Fatalf("healthy request after rejection: status=%d body=%s error=%v", response.StatusCode, body, err)
	}
}

func TestChunkedRequestBinding(t *testing.T) {
	engine := New()
	engine.Post("/upload", func(c *Context) error {
		var input struct {
			Name string `json:"name"`
		}

		if c.Request.GetHeader("Content-Type") == "application/x-www-form-urlencoded" {
			form, err := url.ParseQuery(string(c.Request.Body))
			if err != nil {
				return err
			}

			input.Name = form.Get("name")
		} else {
			if err := NewDefaultBinder().Bind(&input, c); err != nil {
				return err
			}
		}

		return c.String(http.StatusOK, input.Name)
	})

	server := httptest.NewServer(engine.mux)
	defer server.Close()
	for _, test := range []struct {
		contentType string
		body        string
	}{
		{"application/json", `{"name":"chunked upload + café"}`},
		{"application/x-www-form-urlencoded", "name=chunked+upload+%2B+caf%C3%A9"},
	} {
		t.Run(test.contentType, func(t *testing.T) {
			reader := io.NopCloser(strings.NewReader(test.body))
			request, err := http.NewRequest(http.MethodPost, server.URL+"/upload", reader)
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Content-Type", test.contentType)

			response, err := server.Client().Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()

			body, err := io.ReadAll(response.Body)
			if err != nil || response.StatusCode != http.StatusOK || string(body) != "chunked upload + café" {
				t.Fatalf("chunked request: status=%d body=%q error=%v", response.StatusCode, body, err)
			}
		})
	}
}

func BenchmarkRequestBody(b *testing.B) {
	for _, size := range []int64{1024, 4 * 1024 * 1024, 72 * 1024 * 1024} {
		b.Run(fmt.Sprintf("%d_bytes", size), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(size)

			for i := 0; i < b.N; i++ {
				request := httptest.NewRequest(http.MethodPost, "/upload", &requestBodyReader{remaining: size})
				ctx, err := NewContext(nil, request, httptest.NewRecorder(), nil)
				if err != nil || int64(len(ctx.Request.Body)) != size {
					b.Fatalf("request body size=%d error=%v", size, err)
				}
			}
		})
	}
}
