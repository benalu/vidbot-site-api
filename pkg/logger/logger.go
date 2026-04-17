package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Level — log level type
type Level = slog.Level

const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

// contextKey untuk request_id
type contextKey string

const requestIDKey contextKey = "request_id"

var defaultLogger *slog.Logger

// Init — inisialisasi logger. Dipanggil sekali dari main.go.
// format: "json" untuk production, "text" untuk development
func Init(format string, level slog.Level) {
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// persingkat source path — tampilkan hanya 2 level terakhir
			if a.Key == slog.SourceKey {
				if src, ok := a.Value.Any().(*slog.Source); ok {
					parts := strings.Split(filepath.ToSlash(src.File), "/")
					if len(parts) > 2 {
						src.File = strings.Join(parts[len(parts)-2:], "/")
					}
					a.Value = slog.AnyValue(fmt.Sprintf("%s:%d", src.File, src.Line))
				}
			}
			// format waktu lebih ringkas
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(a.Value.Time().Format("2006-01-02 15:04:05.000"))
			}
			return a
		},
	}

	switch strings.ToLower(format) {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	default:
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)
}

// InitWithWriter — untuk testing atau custom output
func InitWithWriter(w io.Writer, format string, level slog.Level) {
	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(w, opts)
	} else {
		handler = slog.NewTextHandler(w, opts)
	}
	defaultLogger = slog.New(handler)
}

// WithContext — embed request_id dari context
func WithContext(ctx context.Context) *slog.Logger {
	if id, ok := ctx.Value(requestIDKey).(string); ok && id != "" {
		return defaultLogger.With("request_id", id)
	}
	return defaultLogger
}

// WithRequestID — buat context dengan request_id
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// ─── Shorthand functions ──────────────────────────────────────────────────────

// Info — log level INFO
func Info(msg string, args ...any) {
	logWithCaller(LevelInfo, msg, args...)
}

// Warn — log level WARN
func Warn(msg string, args ...any) {
	logWithCaller(LevelWarn, msg, args...)
}

// Error — log level ERROR
func Error(msg string, args ...any) {
	logWithCaller(LevelError, msg, args...)
}

// Debug — log level DEBUG
func Debug(msg string, args ...any) {
	logWithCaller(LevelDebug, msg, args...)
}

// InfoCtx — log INFO dengan request context
func InfoCtx(ctx context.Context, msg string, args ...any) {
	WithContext(ctx).Info(msg, args...)
}

// WarnCtx — log WARN dengan request context
func WarnCtx(ctx context.Context, msg string, args ...any) {
	WithContext(ctx).Warn(msg, args...)
}

// ErrorCtx — log ERROR dengan request context
func ErrorCtx(ctx context.Context, msg string, args ...any) {
	WithContext(ctx).Error(msg, args...)
}

// ─── Service-specific loggers ─────────────────────────────────────────────────

// Service — logger dengan service dan platform tag
// Contoh: logger.Service("vidhub", "videb").Info("extract started")
func Service(group, platform string) *slog.Logger {
	if platform == "" {
		return defaultLogger.With("group", group)
	}
	return defaultLogger.With("group", group, "platform", platform)
}

// ─── Timing helper ────────────────────────────────────────────────────────────

// Timer — helper untuk log durasi operasi
// Usage:
//
//	done := logger.Timer("vidhub", "videb", "extract")
//	defer done()
func Timer(group, platform, op string) func() {
	start := time.Now()
	return func() {
		elapsed := time.Since(start)
		Service(group, platform).Debug("operation completed",
			"op", op,
			"elapsed_ms", elapsed.Milliseconds(),
		)
	}
}

// ─── Caller info ──────────────────────────────────────────────────────────────

func logWithCaller(level slog.Level, msg string, args ...any) {
	if defaultLogger == nil {
		// fallback kalau Init belum dipanggil
		fmt.Printf("[%s] %s\n", level, msg)
		return
	}
	// skip 2 frame: logWithCaller → Info/Warn/Error → caller
	_, file, line, ok := runtime.Caller(2)
	if ok {
		parts := strings.Split(filepath.ToSlash(file), "/")
		if len(parts) > 2 {
			file = strings.Join(parts[len(parts)-2:], "/")
		}
		defaultLogger.With("src", fmt.Sprintf("%s:%d", file, line)).Log(context.Background(), level, msg, args...)
		return
	}
	defaultLogger.Log(context.Background(), level, msg, args...)
}
