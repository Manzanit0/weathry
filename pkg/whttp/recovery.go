package whttp

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/manzanit0/weathry/pkg/alert"
)

func Recovery(n alert.Notifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		// This funky mechanic of re-panicking allows leveraging the notifier's
		// recover while at the same time aborting the request upon panic.
		defer n.Recover(c.Request.Context())
		defer func() {
			if r := recover(); r != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				panic(r)
			}
		}()

		c.Next()
	}
}
