package logger

import (
	"context"
	"log/slog"
	"os"
)


type contextKey string

const requestIDKey contextKey = "request_id"
const jobIDKey     contextKey = "job_id"

// Init sets up the global slog logger.
// Call once at startup in main().
func Init(level, format string) {
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

	opts := &slog.HandlerOptions{
		Level: lvl,
	}

	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}

// WithRequestID stores a request ID in the context.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// WithJobID stores a job ID in the context.
func WithJobID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, jobIDKey, id)
}

// FromContext returns a logger with request_id and job_id
// automatically included if they exist in the context.
func FromContext(ctx context.Context) *slog.Logger {
	l := slog.Default()

	if id, ok := ctx.Value(requestIDKey).(string); ok && id != "" {
		l = l.With("request_id", id)
	}

	if id, ok := ctx.Value(jobIDKey).(string); ok && id != "" {
		l = l.With("job_id", id)
	}

	return l
}