package whttp

import (
	"net/http/httputil"

	"github.com/gin-gonic/gin"
)

type Logger interface {
	Println(v ...interface{})
}

func Logging(l Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if l == nil {
			c.Next()
			return
		}

		reqDump, err := httputil.DumpRequest(c.Request, true)
		if err == nil {
			l.Println(string(reqDump))
		}

		c.Next()
	}
}
