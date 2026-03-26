package handler

import "github.com/gin-gonic/gin"

// realClientIP 依赖 Gin 的 ClientIP 与 TrustedProxies 配置。
func realClientIP(c *gin.Context) string {
	return c.ClientIP()
}
