package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/segmentio/ksuid"
)

type CtxKey string

const CtxKeyTraceID CtxKey = "trace_id"

func TraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), CtxKeyTraceID, ksuid.New().String())
		c.Request = c.Request.Clone(ctx)

		c.Next()
	}
}
