package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

type compressWriter struct {
	w              http.ResponseWriter
	zw             *gzip.Writer
	wroteHeader    bool
	shouldCompress bool
}

func newCompressWriter(w http.ResponseWriter) *compressWriter {
	return &compressWriter{
		w:  w,
		zw: gzip.NewWriter(w),
	}
}

func (c *compressWriter) Header() http.Header {
	return c.w.Header()
}

func (c *compressWriter) Write(p []byte) (int, error) {
	if !c.wroteHeader {
		c.WriteHeader(http.StatusOK)
	}
	if !c.shouldCompress {
		return c.w.Write(p)
	}
	return c.zw.Write(p)
}

func (c *compressWriter) WriteHeader(statusCode int) {
	if !c.wroteHeader {
		contentType := c.w.Header().Get("Content-Type")
		if strings.Contains(contentType, "application/json") ||
			strings.Contains(contentType, "text/html") {
			c.shouldCompress = true
			c.w.Header().Set("Content-Encoding", "gzip")
			c.w.Header().Del("Content-Length")
		}
		c.w.WriteHeader(statusCode)
		c.wroteHeader = true
	}
}

type compressReader struct {
	r  io.ReadCloser
	zr *gzip.Reader
}

func newCompressReader(r io.ReadCloser) (*compressReader, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	return &compressReader{r: r, zr: zr}, nil
}

func (c *compressReader) Read(p []byte) (n int, err error) {
	return c.zr.Read(p)
}

func (c *compressReader) Close() error {
	if err := c.r.Close(); err != nil {
		return err
	}
	return c.zr.Close()
}
func GzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ow := w

		if r.Header.Get("Content-Encoding") == "gzip" {
			reader, err := newCompressReader(r.Body)
			if err != nil {
				http.Error(w, "Invalid gzip data", http.StatusBadRequest)
				return
			}
			defer reader.Close()
			r.Body = reader
			r.Header.Del("Content-Length")
			r.ContentLength = -1
		}

		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			cw := newCompressWriter(w)
			defer func() {
				if cw.shouldCompress {
					cw.zw.Close()
				}
			}()
			ow = cw
		}

		next.ServeHTTP(ow, r)
	})
}
