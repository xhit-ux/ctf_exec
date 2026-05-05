package ws

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/ctf_exec/backend/internal/authsession"
	"github.com/ctf_exec/backend/internal/config"
	"github.com/gorilla/websocket"
)

// pongWaitMargin 是在心跳间隔基础上等待 pong 的额外时间，超时即认为连接已死。
const pongWaitMargin = 15 * time.Second

// writeWait 是单次写操作的最大耗时；超时则视为客户端断开。
const writeWait = 10 * time.Second

// maxMessageSize 限制客户端可发送的最大文本消息长度，防止恶意大包。
const maxMessageSize = 512

// client 表示一条单向心跳连接。本系统不允许客户端发起业务消息，readPump 仅丢弃数据。
type client struct {
	conn         *websocket.Conn
	userID       string
	connID       string
	sessionStore *authsession.Store
	cfg          config.WSConfig

	closeOnce sync.Once
	done      chan struct{}
}

func newClient(conn *websocket.Conn, userID, connID string, store *authsession.Store, cfg config.WSConfig) *client {
	return &client{
		conn:         conn,
		userID:       userID,
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
	c.readPump()
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
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("[ws] ping err user=%s conn=%s err=%v", c.userID, c.connID, err)
				return
			}
		}
	}
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
