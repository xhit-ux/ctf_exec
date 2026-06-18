package logger

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type responseWriter struct {
	gin.ResponseWriter
	written int
}

func (w *responseWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.written += n
	return n, err
}

func (w *responseWriter) WriteString(s string) (int, error) {
	n, err := w.ResponseWriter.WriteString(s)
	w.written += n
	return n, err
}

func GinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := sanitizeRequestID(c.GetHeader("X-Request-ID"))
		if rid == "" {
			rid = fmt.Sprintf("req_%s", uuid.New().String()[:8])
		}
		c.Set(string(requestIDKey), rid)
		c.Writer.Header().Set("X-Request-ID", rid)

		ctx := WithRequestID(c.Request.Context(), rid)
		c.Request = c.Request.WithContext(ctx)

		rw := &responseWriter{ResponseWriter: c.Writer}
		c.Writer = rw

		start := time.Now()
		c.Next()
		latency := time.Since(start)

		status := c.Writer.Status()
		ua := c.Request.UserAgent()
		if len(ua) > 128 {
			ua = ua[:128]
		}

		attrs := []any{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", status,
			"latency_ms", latency.Milliseconds(),
			"bytes", rw.written,
			"client_ip", c.ClientIP(),
			"ua", ua,
		}

		if msg, exists := c.Get("log_error"); exists {
			if s, ok := msg.(string); ok && s != "" {
				attrs = append(attrs, "error", s)
			}
		}

		msg := fmt.Sprintf("%s %s %d %dms %dB", c.Request.Method, c.Request.URL.Path, status, latency.Milliseconds(), rw.written)

		if status >= 500 {
			Default().ErrorContext(ctx, msg, attrs...)
		} else if status >= 400 {
			Default().WarnContext(ctx, msg, attrs...)
		} else {
			Default().InfoContext(ctx, msg, attrs...)
		}
	}
}

// sanitizeRequestID 校验客户端传入的 X-Request-ID，拒绝不安全的值。
// 仅允许 [a-zA-Z0-9._-]，最大 64 字符。
func sanitizeRequestID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 64 {
		return ""
	}
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-' {
			continue
		}
		return ""
	}
	return raw
}
