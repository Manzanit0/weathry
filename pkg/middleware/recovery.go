package middleware

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/manzanit0/weathry/pkg/tgram"
	"golang.org/x/exp/slog"
)

func Recovery(t tgram.Client, reportChat int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			Recover(t, reportChat)
			c.AbortWithStatus(http.StatusInternalServerError)
		}()

		c.Next()
	}
}

func Recover(t tgram.Client, reportChat int64) {
	if r := recover(); r != nil {
		callstack := getCallstack()
		slog.Error(fmt.Sprint("recovered from panic", callstack))

		if t != nil {
			_ = t.SendMessage(tgram.SendMessageRequest{
				ParseMode: tgram.ParseModeHTML,
				ChatID:    reportChat,
				Text: fmt.Sprintf(`<b>Recovered from panic: %v</b>
<code>%s</code>`, r, callstack),
			})
		}

	}
}

func getCallstack() string {
	pcs := make([]uintptr, 20)
	depth := runtime.Callers(3, pcs)
	frames := runtime.CallersFrames(pcs[:depth])

	var sb strings.Builder
	for f, more := frames.Next(); more; f, more = frames.Next() {
		sb.WriteString(fmt.Sprintf("%s: %d\n", f.Function, f.Line))
	}

	return sb.String()
}
