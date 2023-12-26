package logger

import (
	"context"
	"io"
	"log/slog"
	"os"

	"github.com/manzanit0/weathry/pkg/middleware"
)

func InitGlobalSlog(service string) {
	handler := NewContextJSONHandler(os.Stdout, nil)
	logger := slog.New(handler)
	logger = logger.With("service", service)
	slog.SetDefault(logger)
}

type ContextJSONHandler struct {
	jsonHandler slog.Handler
}

func NewContextJSONHandler(w io.Writer, opts *slog.HandlerOptions) *ContextJSONHandler {
	return &ContextJSONHandler{slog.NewJSONHandler(w, opts)}
}

func (h *ContextJSONHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.jsonHandler.Enabled(ctx, level)
}

func (h *ContextJSONHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ContextJSONHandler{jsonHandler: h.jsonHandler.WithAttrs(attrs)}
}

func (h *ContextJSONHandler) WithGroup(name string) slog.Handler {
	return &ContextJSONHandler{jsonHandler: h.jsonHandler.WithGroup(name)}
}

func (h *ContextJSONHandler) Handle(ctx context.Context, r slog.Record) error {
	trace_id := ctx.Value(middleware.CtxKeyTraceID)
	if str, ok := trace_id.(string); ok {
		r.AddAttrs(slog.String(string(middleware.CtxKeyTraceID), str))
	}

	return h.jsonHandler.Handle(ctx, r)
}
