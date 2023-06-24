package middleware

import (
	"net/http/httputil"

	"github.com/gin-gonic/gin"
	"golang.org/x/exp/slog"
)

func Logging() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqDump, err := httputil.DumpRequest(c.Request, true)
		if err == nil {
			slog.Info(string(reqDump))
		}

		c.Next()
	}
}
