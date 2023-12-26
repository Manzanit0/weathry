package middleware

import (
	"bytes"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (r responseBodyWriter) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

func Logger(debug bool) gin.HandlerFunc {
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

		body := "<redacted>"
		if debug {
			body = w.body.String()
		}

		logFields := []any{
			slog.Group("http",
				slog.Group("request",
					"duration_ms", time.Since(t0).Milliseconds(),
					"method", c.Request.Method,
					"content_length", c.Request.ContentLength,
					"headers", c.Request.Header,
					slog.Group("url",
						"scheme", c.Request.URL.Scheme,
						"host", c.Request.URL.Host,
						"path", c.Request.URL.Path,
						"query_params", q,
					),
				),
				slog.Group("response",
					"status", c.Writer.Status(),
					"size", c.Writer.Size(),
					"body", body,
				),
			),
		}

		slog.InfoContext(c.Request.Context(), "inbound request", logFields...)
	}
}
