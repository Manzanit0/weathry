package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/exp/slog"
)

func Logging() gin.HandlerFunc {
	return func(c *gin.Context) {
		t0 := time.Now()

		c.Next()

		// Ofuscate access_key if it exists.
		q := c.Request.URL.Query()
		if q.Has("access_key") {
			q.Set("access_key", "*****")
		}

		slog.Info("inbound request",
			"http.request.duration_ms", time.Since(t0).Milliseconds(),
			"http.request.method", c.Request.Method,
			"http.request.url.scheme", c.Request.URL.Scheme,
			"http.request.url.host", c.Request.URL.Host,
			"http.request.url.path", c.Request.URL.Path,
			"http.request.url.query_params", q,
			"http.request.content_length", c.Request.ContentLength,
			"http.request.headers", c.Request.Header,
			"http.response.status_code", c.Request.Response.StatusCode,
			"http.response.headers", c.Request.Response.Header,
			"http.response.content_length", c.Request.Response.ContentLength,
			"http.response.uncompressed", c.Request.Response.Uncompressed,
			"http.response.protocol", c.Request.Response.Proto)
	}
}
