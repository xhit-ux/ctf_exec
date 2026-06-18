package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"sync"
	"time"

	"github.com/ctf_exec/backend/internal/config"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Client 封装 RabbitMQ 连接与 channel 管理。
// 支持自动重连、发布确认、QoS 预取。
type Client struct {
	cfg       config.RabbitMQConfig
	conn      *amqp.Connection
	connMu    sync.RWMutex
	closeCh   chan *amqp.Error
	doneCh    chan struct{}
	reconnect bool
}

// New 创建 RabbitMQ 客户端并建立连接。
func New(ctx context.Context, cfg config.RabbitMQConfig) (*Client, error) {
	c := &Client{
		cfg:       cfg,
		closeCh:   make(chan *amqp.Error, 1),
		doneCh:    make(chan struct{}),
		reconnect: true,
	}
	if err := c.connect(ctx); err != nil {
		return nil, err
	}
	go c.reconnectLoop()
	return c, nil
}

func (c *Client) connect(ctx context.Context) error {
	dsn := fmt.Sprintf("amqp://%s:%s@%s:%d/%s",
		c.cfg.User, c.cfg.Password, c.cfg.Host, c.cfg.Port, c.cfg.VHost)

	conn, err := amqp.Dial(dsn)
	if err != nil {
		return fmt.Errorf("RabbitMQ 连接失败: %w", err)
	}
	conn.NotifyClose(c.closeCh)

	c.connMu.Lock()
	if c.conn != nil {
		_ = c.conn.Close()
	}
	c.conn = conn
	c.connMu.Unlock()

	hostname, _ := os.Hostname()
	log.Printf("[RabbitMQ] 连接成功 host=%s:%d vhost=%s worker=%s",
		c.cfg.Host, c.cfg.Port, c.cfg.VHost, hostname)
	return nil
}

func (c *Client) reconnectLoop() {
	baseDelay := time.Duration(c.cfg.ReconnectDelay) * time.Second
	if baseDelay <= 0 {
		baseDelay = 3 * time.Second
	}
	attempt := 0

	for {
		select {
		case <-c.doneCh:
			return
		case err, ok := <-c.closeCh:
			if !ok {
				return
			}
			log.Printf("[RabbitMQ] 连接断开: %v — 准备重连", err)

			for c.reconnect {
				attempt++
				delay := baseDelay * time.Duration(math.Pow(2, float64(attempt)))
				if delay > 60*time.Second {
					delay = 60 * time.Second
				}
				log.Printf("[RabbitMQ] 第 %d 次重连尝试，等待 %v", attempt, delay)

				select {
				case <-c.doneCh:
					return
				case <-time.After(delay):
				}

				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				if err := c.connect(ctx); err == nil {
					cancel()
					attempt = 0
					log.Printf("[RabbitMQ] 重连成功")
					break
				}
				cancel()
			}
		}
	}
}

// Channel 返回一个新 channel（非线程安全，调用方管理生命周期）。
func (c *Client) Channel() (*amqp.Channel, error) {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	if conn == nil || conn.IsClosed() {
		return nil, fmt.Errorf("RabbitMQ 连接不可用")
	}
	return conn.Channel()
}

// DeclareQueue 声明持久化队列。
func (c *Client) DeclareQueue(ch *amqp.Channel, name string) (amqp.Queue, error) {
	return ch.QueueDeclare(
		name,  // name
		true,  // durable — 重启不丢消息
		false, // autoDelete
		false, // exclusive
		false, // noWait
		nil,
	)
}

// PublishTask 发布任务消息到指定队列（带发布确认）。
func (c *Client) PublishTask(ctx context.Context, queueName string, payload map[string]string) error {
	ch, err := c.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	if err := ch.Confirm(false); err != nil {
		return fmt.Errorf("启用发布确认失败: %w", err)
	}
	confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 1))

	if _, err := c.DeclareQueue(ch, queueName); err != nil {
		return fmt.Errorf("声明队列失败: %w", err)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化任务消息失败: %w", err)
	}

	if err := ch.PublishWithContext(ctx,
		"",
		queueName,
		true,  // mandatory
		false, // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Body:         body,
			Timestamp:    time.Now(),
		},
	); err != nil {
		return fmt.Errorf("发布消息失败: %w", err)
	}

	select {
	case confirm := <-confirms:
		if !confirm.Ack {
			return fmt.Errorf("消息未被 Broker 确认")
		}
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

// ConsumeTasks 开始消费指定队列。返回 delivery channel。
func (c *Client) ConsumeTasks(ctx context.Context, queueName string, prefetch int) (<-chan amqp.Delivery, error) {
	ch, err := c.Channel()
	if err != nil {
		return nil, err
	}

	if prefetch <= 0 {
		prefetch = 1
	}
	if err := ch.Qos(prefetch, 0, false); err != nil {
		ch.Close()
		return nil, fmt.Errorf("设置 QoS 失败: %w", err)
	}

	if _, err := c.DeclareQueue(ch, queueName); err != nil {
		ch.Close()
		return nil, fmt.Errorf("声明队列失败: %w", err)
	}

	deliveries, err := ch.Consume(
		queueName,
		"",    // consumer tag
		false, // autoAck — Worker 手动 ACK
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil,
	)
	if err != nil {
		ch.Close()
		return nil, fmt.Errorf("开始消费失败: %w", err)
	}
	return deliveries, nil
}

// Shutdown 优雅关闭。
func (c *Client) Shutdown() {
	c.reconnect = false
	close(c.doneCh)

	c.connMu.Lock()
	defer c.connMu.Unlock()
	if c.conn != nil && !c.conn.IsClosed() {
		if err := c.conn.Close(); err != nil {
			log.Printf("[RabbitMQ] 关闭连接时出错: %v", err)
		}
	}
	log.Printf("[RabbitMQ] 已关闭")
}
