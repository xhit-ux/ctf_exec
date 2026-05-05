// Package ws 提供 WebSocket 心跳与在线状态上报能力。
//
// 架构说明：
//   - 客户端建立 WS 连接时，HTTP 升级请求会先经过 GinAuthMiddlewareWithRepo 完成 JWT 鉴权。
//   - 升级成功后服务端通过定时 ping 维持连接活性；浏览器自动回复 pong（无需 JS 介入）。
//   - 每次收到 pong 即刷新 Redis 中的 presence/conn 元数据 TTL，作为「在线」判定依据。
//   - 本模块只承担心跳和在线状态上报，不传输业务消息；业务推送如需引入再扩展。
package ws

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/ctf_exec/backend/internal/apierror"
	"github.com/ctf_exec/backend/internal/apperr"
	"github.com/ctf_exec/backend/internal/authsession"
	"github.com/ctf_exec/backend/internal/config"
	"github.com/ctf_exec/backend/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Handler 处理 WebSocket 升级与生命周期。
type Handler struct {
	cfg          config.WSConfig
	sessionStore *authsession.Store
	upgrader     websocket.Upgrader
}

// NewHandler 构造 WS Handler。allowedOrigins 复用全站 CORS 白名单。
func NewHandler(cfg config.WSConfig, sessionStore *authsession.Store, allowedOrigins []string) *Handler {
	originSet := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		o = strings.TrimSpace(o)
		if o != "" {
			originSet[strings.ToLower(o)] = struct{}{}
		}
	}

	return &Handler{
		cfg:          cfg,
		sessionStore: sessionStore,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				origin := strings.TrimSpace(strings.ToLower(r.Header.Get("Origin")))
				if origin == "" {
					// 同源请求允许放行。
					return true
				}
				if len(originSet) == 0 {
					return true
				}
				_, ok := originSet[origin]
				return ok
			},
		},
	}
}

// HandleConnect 升级 HTTP 请求为 WebSocket 并接管心跳生命周期。
//
// 前置条件：必须经过 GinAuthMiddlewareWithRepo，使 claims 可从 gin.Context 取出。
func (h *Handler) HandleConnect(c *gin.Context) {
	if !h.cfg.Enabled {
		apierror.Write(c, apperr.New(apperr.ErrForbidden, "WebSocket 服务未启用"), http.StatusForbidden)
		return
	}

	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// gorilla 在升级失败时会自动写入响应，无需再写 JSON。
		log.Printf("[ws] upgrade failed user=%s err=%v", claims.UserID, err)
		return
	}

	connID := uuid.NewString()
	ctx := c.Request.Context()
	if err := h.sessionStore.RegisterWSConnection(ctx, claims.UserID, connID, h.cfg.MaxConnectionsPerUser, h.cfg.PresenceTimeout); err != nil {
		if errors.Is(err, authsession.ErrWSConnectionLimitExceeded) {
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(
				websocket.ClosePolicyViolation,
				"connection limit exceeded",
			))
		} else {
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(
				websocket.CloseInternalServerErr,
				"register failed",
			))
			log.Printf("[ws] register conn failed user=%s err=%v", claims.UserID, err)
		}
		_ = conn.Close()
		return
	}

	client := newClient(conn, claims.UserID, connID, h.sessionStore, h.cfg)
	client.start()
}
