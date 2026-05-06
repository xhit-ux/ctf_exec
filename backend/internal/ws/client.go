package ws

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/ctf_exec/backend/internal/authsession"
	"github.com/ctf_exec/backend/internal/config"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/gorilla/websocket"
)

// pongWaitMargin 是在心跳间隔基础上等待 pong 的额外时间，超时即认为连接已死。
const pongWaitMargin = 15 * time.Second

// writeWait 是单次写操作的最大耗时；超时则视为客户端断开。
const writeWait = 10 * time.Second

// maxMessageSize 限制客户端可发送的最大文本消息长度，防止恶意大包。
const maxMessageSize = 512

// client 表示一条单向心跳连接。本系统不允许客户端发起业务消息，readPump 仅丢弃数据。
//
// 鉴权再校验：当 cfg.SessionRecheckInterval > 0 时，会启动独立 goroutine 周期性
// 校验 access 会话是否仍有效（被踢下线 / 主动登出 / 单会话指针置换），失效则关闭连接。
type client struct {
	conn         *websocket.Conn
	userID       string
	jti          string
	role         string
	connID       string
	sessionStore *authsession.Store
	cfg          config.WSConfig

	writeMu   sync.Mutex
	closeOnce sync.Once
	done      chan struct{}
}

func newClient(conn *websocket.Conn, userID, jti, role, connID string, store *authsession.Store, cfg config.WSConfig) *client {
	return &client{
		conn:         conn,
		userID:       userID,
		jti:          jti,
		role:         models.NormalizeRole(role),
		connID:       connID,
		sessionStore: store,
		cfg:          cfg,
		done:         make(chan struct{}),
	}
}

func (c *client) start() {
	pongWait := c.cfg.ClientHeartbeatInterval + pongWaitMargin
	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))

	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := c.sessionStore.RefreshWSHeartbeat(ctx, c.userID, c.connID, c.cfg.PresenceTimeout); err != nil {
			log.Printf("[ws] refresh heartbeat failed user=%s conn=%s err=%v", c.userID, c.connID, err)
		}
		return nil
	})

	go c.writePump()
	if c.cfg.SessionRecheckInterval > 0 {
		go c.sessionRecheckPump()
	}
	c.readPump()
}

// writeMessage 串行化所有 websocket 写操作，避免 ping / close 帧并发写入造成数据错乱。
func (c *client) writeMessage(messageType int, data []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	return c.conn.WriteMessage(messageType, data)
}

// readPump 阻塞读取直到连接关闭。本服务不消费业务消息，但必须读取以触发 pong handler。
func (c *client) readPump() {
	defer c.cleanup()

	for {
		if _, _, err := c.conn.NextReader(); err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Printf("[ws] read err user=%s conn=%s err=%v", c.userID, c.connID, err)
			}
			return
		}
	}
}

// writePump 周期性发送 ping；任意一次写入失败即关闭连接。
func (c *client) writePump() {
	ticker := time.NewTicker(c.cfg.ClientHeartbeatInterval)
	defer func() {
		ticker.Stop()
		c.cleanup()
	}()

	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			if err := c.writeMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("[ws] ping err user=%s conn=%s err=%v", c.userID, c.connID, err)
				return
			}
		}
	}
}

// sessionRecheckPump 周期性校验 access 会话与单会话指针；失效即主动关闭连接。
//
// 触发场景：
//   - 用户主动登出 / 角色变更 / 被封禁后调用 RevokeAllUserSessions；
//   - admin 用户在另一处登录，导致 user_current 指针指向新 jti；
//   - 中间件层因密码重置等高风险操作下发会话撤销。
//
// 网络/Redis 抖动产生的 transient error 不会立即关闭连接，仅记录日志。
func (c *client) sessionRecheckPump() {
	ticker := time.NewTicker(c.cfg.SessionRecheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			if !c.checkSessionStillValid() {
				c.closeWithReason(websocket.ClosePolicyViolation, "session revoked")
				return
			}
		}
	}
}

// checkSessionStillValid 返回 false 表示明确判定会话已失效（应当关闭连接）；
// transient 错误（Redis 抖动等）保守地返回 true 以避免误踢。
func (c *client) checkSessionStillValid() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	active, err := c.sessionStore.IsSessionActive(ctx, c.userID, c.jti)
	if err != nil {
		log.Printf("[ws] session recheck err user=%s conn=%s err=%v", c.userID, c.connID, err)
		return true
	}
	if !active {
		log.Printf("[ws] session revoked, closing user=%s conn=%s jti=%s", c.userID, c.connID, c.jti)
		return false
	}

	if c.role == models.RoleRoot || c.role == models.RoleSiteAdmin {
		isCurrent, err := c.sessionStore.IsUserCurrentSession(ctx, c.userID, c.jti)
		if err != nil {
			log.Printf("[ws] current session recheck err user=%s conn=%s err=%v", c.userID, c.connID, err)
			return true
		}
		if !isCurrent {
			log.Printf("[ws] kicked by single_session policy user=%s conn=%s jti=%s", c.userID, c.connID, c.jti)
			return false
		}
	}
	return true
}

// closeWithReason 发送 Close 帧并触发清理；写失败时静默吞掉（连接可能已经断开）。
func (c *client) closeWithReason(code int, reason string) {
	_ = c.writeMessage(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason))
	c.cleanup()
}

func (c *client) cleanup() {
	c.closeOnce.Do(func() {
		close(c.done)
		_ = c.conn.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := c.sessionStore.UnregisterWSConnection(ctx, c.userID, c.connID); err != nil {
			log.Printf("[ws] unregister conn failed user=%s conn=%s err=%v", c.userID, c.connID, err)
		}
	})
}
