package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

type contextKey string

const requestIDKey contextKey = "request_id"

var defaultLogger *slog.Logger

func Init(enabled bool, level, format, filePath string, maxSize, maxBackups, maxAge int, compress bool) {
	var l slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		l = slog.LevelDebug
	case "info", "":
		l = slog.LevelInfo
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     l,
		AddSource: l == slog.LevelDebug,
	}

	if !enabled {
		defaultLogger = slog.New(slog.NewTextHandler(os.Stderr, opts))
		return
	}

	consoleHandler := slog.NewTextHandler(os.Stdout, opts)

	var handler slog.Handler
	if filePath == "" {
		handler = consoleHandler
	} else {
		rotator := &lumberjack.Logger{
			Filename:   filePath,
			MaxSize:    maxSize,
			MaxBackups: maxBackups,
			MaxAge:     maxAge,
			Compress:   compress,
		}
		closers = append(closers, rotator)

		var fileHandler slog.Handler
		if strings.ToLower(strings.TrimSpace(format)) == "json" {
			fileHandler = slog.NewJSONHandler(rotator, opts)
		} else {
			fileHandler = slog.NewTextHandler(rotator, opts)
		}
		handler = newMultiHandler(consoleHandler, fileHandler, rotator)
	}

	defaultLogger = slog.New(handler)
}

// multiHandler 将日志同时写入多个 handler（console + file）。
type multiHandler struct {
	handlers []slog.Handler
	mu       sync.Mutex
	closers  []io.Closer
}

func newMultiHandler(console, file slog.Handler, closers ...io.Closer) *multiHandler {
	return &multiHandler{handlers: []slog.Handler{console, file}, closers: closers}
}

func (h *multiHandler) Enabled(ctx context.Context, l slog.Level) bool {
	for _, hh := range h.handlers {
		if hh.Enabled(ctx, l) {
			return true
		}
	}
	return false
}

func (h *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, hh := range h.handlers {
		if hh.Enabled(ctx, r.Level) {
			if err := hh.Handle(ctx, r.Clone()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, hh := range h.handlers {
		handlers[i] = hh.WithAttrs(attrs)
	}
	return &multiHandler{handlers: handlers}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, hh := range h.handlers {
		handlers[i] = hh.WithGroup(name)
	}
	return &multiHandler{handlers: handlers}
}

var closers []io.Closer

func Close() {
	for _, c := range closers {
		_ = c.Close()
	}
	closers = nil
}

func Default() *slog.Logger {
	if defaultLogger == nil {
		defaultLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
	return defaultLogger
}

func NewTestLogger(w io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func RequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

func logArgs(ctx context.Context, msg string, args ...any) (string, []any) {
	if rid := RequestID(ctx); rid != "" {
		extra := make([]any, 0, len(args)+2)
		extra = append(extra, "request_id", rid)
		extra = append(extra, args...)
		return msg, extra
	}
	return msg, args
}

func Debug(msg string, args ...any) {
	Default().Debug(msg, args...)
}

func DebugContext(ctx context.Context, msg string, args ...any) {
	msg, args = logArgs(ctx, msg, args...)
	Default().Debug(msg, args...)
}

func Info(msg string, args ...any) {
	Default().Info(msg, args...)
}

func InfoContext(ctx context.Context, msg string, args ...any) {
	msg, args = logArgs(ctx, msg, args...)
	Default().Info(msg, args...)
}

func Warn(msg string, args ...any) {
	Default().Warn(msg, args...)
}

func WarnContext(ctx context.Context, msg string, args ...any) {
	msg, args = logArgs(ctx, msg, args...)
	Default().Warn(msg, args...)
}

func Error(msg string, args ...any) {
	Default().Error(msg, args...)
}

func ErrorContext(ctx context.Context, msg string, args ...any) {
	msg, args = logArgs(ctx, msg, args...)
	Default().Error(msg, args...)
}

func Fatal(msg string, args ...any) {
	Default().Error(msg, args...)
	fmt.Fprintf(os.Stderr, "FATAL: "+msg+"\n", args...)
	os.Exit(1)
}

func Now() string {
	return time.Now().Format(time.RFC3339)
}

// Console 直接写 stdout，不经过 slog 编码，保留 ANSI 颜色。
// 用于启动横幅、里程碑提示等需要终端色彩的消息。
func Console(format string, args ...any) {
	fmt.Fprintf(os.Stdout, format+"\n", args...)
}
