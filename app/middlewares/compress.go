package middlewares

import (
	"bufio"
	"compress/gzip"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
	gzipWriter *gzip.Writer
}

func Compress() web.MiddlewareFunc {
	return func(next web.HandlerFunc) web.HandlerFunc {
		return func(c *web.Context) error {
			if strings.Contains(c.Request.GetHeader("Accept"), "text/event-stream") {
				return next(c)
			}

			if strings.Contains(c.Request.GetHeader("Accept-Encoding"), "gzip") {
				res := c.Response
				res.Header().Set("Content-Encoding", "gzip")
				res.Header().Del("Accept-Encoding")
				res.Header().Add("Vary", "Accept-Encoding")

				gw, _ := gzip.NewWriterLevel(res.Writer, gzip.DefaultCompression)

				c.Response.Writer = &gzipResponseWriter{
					Writer:         gw,
					ResponseWriter: c.Response.Writer,
					gzipWriter:     gw,
				}

				err := next(c)
				gw.Close()
				return err
			}
			return next(c)
		}
	}
}

func (w *gzipResponseWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	w.ResponseWriter.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(code)

}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	header := w.ResponseWriter.Header()
	if header.Get("Content-Type") == "" {
		header.Set("Content-Type", http.DetectContentType(b))
	}
	header.Del("Content-Length")

	return w.Writer.Write(b)
}

func (w *gzipResponseWriter) Flush() {
	w.gzipWriter.Flush()
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *gzipResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.ResponseWriter.(http.Hijacker).Hijack()
}
