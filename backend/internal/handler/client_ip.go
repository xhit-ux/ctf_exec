package handler

import (
	"net"
	"strings"

	"github.com/gin-gonic/gin"
)

// realClientIP 优先使用 Gin 的 ClientIP（配合 TrustedProxies），并保留头部兜底解析。
func realClientIP(c *gin.Context) string {
	if ip := strings.TrimSpace(c.ClientIP()); ip != "" {
		return ip
	}

	if forwardedFor := strings.TrimSpace(c.GetHeader("X-Forwarded-For")); forwardedFor != "" {
		parts := strings.Split(forwardedFor, ",")
		for _, part := range parts {
			candidate := strings.TrimSpace(part)
			if parsed := net.ParseIP(candidate); parsed != nil {
				return parsed.String()
			}
		}
	}

	if realIP := strings.TrimSpace(c.GetHeader("X-Real-IP")); realIP != "" {
		if parsed := net.ParseIP(realIP); parsed != nil {
			return parsed.String()
		}
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(c.Request.RemoteAddr))
	if err == nil {
		if parsed := net.ParseIP(host); parsed != nil {
			return parsed.String()
		}
	}

	return strings.TrimSpace(c.Request.RemoteAddr)
}
