package middleware

import (
	"bytes"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/exp/slog"
)

type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (r responseBodyWriter) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

func Logging(debug bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		w := &responseBodyWriter{body: &bytes.Buffer{}, ResponseWriter: c.Writer}
		c.Writer = w

		t0 := time.Now()

		c.Next()

		// Ofuscate access_key if it exists.
		q := c.Request.URL.Query()
		if q.Has("access_key") {
			q.Set("access_key", "*****")
		}

		logFields := []any{
			"http.response.status", c.Writer.Status(),
			"http.response.size", c.Writer.Size(),
			"http.request.duration_ms", time.Since(t0).Milliseconds(),
			"http.request.method", c.Request.Method,
			"http.request.url.scheme", c.Request.URL.Scheme,
			"http.request.url.host", c.Request.URL.Host,
			"http.request.url.path", c.Request.URL.Path,
			"http.request.url.query_params", q,
			"http.request.content_length", c.Request.ContentLength,
			"http.request.headers", c.Request.Header,
		}

		if debug {
			logFields = append(logFields, "http.response.body", w.body.String())
		}

		slog.Info("inbound request", logFields...)
	}
}
