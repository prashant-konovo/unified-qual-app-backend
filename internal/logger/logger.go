package logger

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
)

// Setup initialises the global slog logger.
// level: "debug", "info", "warn", "error" (default "info").
func Setup(level string) {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	slog.SetDefault(slog.New(h))
}

// Middleware logs every HTTP request with method, path, status, duration and request ID.
// It replaces chi's default text Logger with structured JSON output.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
		reqID := chimw.GetReqID(r.Context())

		next.ServeHTTP(ww, r)

		if r.URL.Path == "/health" {
			return // skip noise
		}

		slog.InfoContext(r.Context(), "http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"bytes", ww.BytesWritten(),
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", reqID,
			"remote", r.RemoteAddr,
		)
	})
}

// FromCtx returns a logger enriched with the request ID from context.
func FromCtx(ctx context.Context) *slog.Logger {
	if reqID := chimw.GetReqID(ctx); reqID != "" {
		return slog.With("request_id", reqID)
	}
	return slog.Default()
}
