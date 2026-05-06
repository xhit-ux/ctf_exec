package middleware

import (
	"net/http"
	"strings"

	"github.com/ctf_exec/backend/internal/apierror"
	"github.com/ctf_exec/backend/internal/apperr"
	"github.com/gin-gonic/gin"
)

// stateChangingMethods 定义需要 CSRF 校验的 HTTP 方法。
var stateChangingMethods = map[string]bool{
	http.MethodPost:   true,
	http.MethodPut:    true,
	http.MethodPatch:  true,
	http.MethodDelete: true,
}

// GinCSRFMiddleware 校验状态变更请求的 Origin / Referer，防御跨站请求伪造。
//
// 校验规则：
//   - GET/HEAD/OPTIONS 请求直接放行（无副作用，不需要校验）。
//   - 状态变更请求若携带 Origin 头，必须匹配白名单。
//   - 若 Origin 缺失但 Referer 存在，从 Referer 提取 origin 部分并校验。
//   - 两者均缺失：视为非浏览器请求（如 curl、脚本调用），放行。
//
// 注意：本中间件作为纵深防线，真正的 CSRF 防线是 Cookie 的 SameSite 属性 +
// CORS 校验 + 本中间件的三重组合。
func GinCSRFMiddleware(allowedOrigins []string) gin.HandlerFunc {
	allowedSet := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		normalized := strings.TrimSpace(origin)
		if normalized == "" {
			continue
		}
		allowedSet[strings.ToLower(normalized)] = struct{}{}
	}

	return func(c *gin.Context) {
		if !stateChangingMethods[c.Request.Method] {
			c.Next()
			return
		}

		if len(allowedSet) == 0 {
			c.Next()
			return
		}

		origin := normalizeOrigin(c.GetHeader("Origin"))
		if origin != "" {
			if _, ok := allowedSet[origin]; !ok {
				apierror.Write(c, apperr.New(apperr.ErrForbidden, "不允许的跨站请求来源"), http.StatusForbidden)
				c.Abort()
				return
			}
			c.Next()
			return
		}

		referer := normalizeOrigin(extractRefererOrigin(c.GetHeader("Referer")))
		if referer != "" {
			if _, ok := allowedSet[referer]; !ok {
				apierror.Write(c, apperr.New(apperr.ErrForbidden, "不允许的跨站请求来源"), http.StatusForbidden)
				c.Abort()
				return
			}
		}

		// 无 Origin 且无 Referer → 非浏览器请求，放行
		c.Next()
	}
}

// normalizeOrigin 统一 Origin 格式为小写、去尾斜杠。
func normalizeOrigin(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" || s == "null" {
		return ""
	}
	s = strings.TrimRight(s, "/")
	return strings.ToLower(s)
}

// extractRefererOrigin 从 Referer URL 中提取 scheme://host:port 部分。
func extractRefererOrigin(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	// 找到第三个 / 的位置，截取到那里即为 origin 部分
	// e.g. https://example.com/path → https://example.com
	idx := 0
	slashCount := 0
	for i, ch := range s {
		if ch == '/' {
			slashCount++
			if slashCount == 3 { // https:// has two slashes, then the third / starts the path
				idx = i
				break
			}
		}
	}
	if idx > 0 {
		return normalizeOrigin(s[:idx])
	}
	return normalizeOrigin(s)
}
