package logger

import (
	"fmt"
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
		rid := c.GetHeader("X-Request-ID")
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
