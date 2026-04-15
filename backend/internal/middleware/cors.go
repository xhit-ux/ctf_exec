package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// GinCORSMiddleware Gin CORS 中间件。
func GinCORSMiddleware(allowedOrigins []string) gin.HandlerFunc {
	allowedSet := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		normalized := strings.TrimSpace(origin)
		if normalized == "" {
			continue
		}
		allowedSet[normalized] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin != "" {
			if _, ok := allowedSet[origin]; !ok {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Vary", "Origin")
		}
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "X-Access-Expires-In-Secs")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
