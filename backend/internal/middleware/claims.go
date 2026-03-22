package middleware

import (
	"fmt"
	"net/http"

	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/utils"
	"github.com/gin-gonic/gin"
)

// GetClaims 从上下文获取并校验 claims。
func GetClaims(c *gin.Context) (*utils.Claims, error) {
	claimsInterface, exists := c.Get(claimsContextKey)
	if !exists {
		return nil, fmt.Errorf("claims not found")
	}

	claims, ok := claimsInterface.(*utils.Claims)
	if !ok {
		return nil, fmt.Errorf("invalid claims type")
	}

	claims.Role = models.NormalizeRole(claims.Role)
	return claims, nil
}

// MustGetClaims 从上下文读取 claims，失败时自动返回 401 并终止请求。
func MustGetClaims(c *gin.Context) (*utils.Claims, bool) {
	claims, err := GetClaims(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的用户信息"})
		c.Abort()
		return nil, false
	}

	return claims, true
}
