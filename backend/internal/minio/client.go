package minio

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/ctf_exec/backend/internal/config"
	minio "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Client MinIO 客户端封装，提供题目附件上传/下载/删除的便利方法。
type Client struct {
	client        *minio.Client
	bucketPrefix  string
	presignExpiry time.Duration
}

// NewClient 从配置初始化 MinIO 客户端。
func NewClient(cfg config.MinIOConfig) (*Client, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("初始化 MinIO 客户端失败: %w", err)
	}
	return &Client{
		client:        client,
		bucketPrefix:  cfg.BucketPrefix,
		presignExpiry: time.Duration(cfg.PresignExpiry) * time.Second,
	}, nil
}

// EnsureBuckets 确保所有 category 对应的 bucket 已存在，不存在则创建。
func (c *Client) EnsureBuckets(ctx context.Context) error {
	for _, category := range []string{"web", "pwn", "crypto", "reverse", "misc"} {
		bucket := c.BucketName(category)
		exists, err := c.client.BucketExists(ctx, bucket)
		if err != nil {
			return fmt.Errorf("检查 bucket %s 是否存在失败: %w", bucket, err)
		}
		if !exists {
			if err := c.client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
				return fmt.Errorf("创建 bucket %s 失败: %w", bucket, err)
			}
		}
	}
	return nil
}

// BucketName 按 category 拼接 bucket 名称。
func (c *Client) BucketName(category string) string {
	return fmt.Sprintf("%s-attachments-%s", c.bucketPrefix, category)
}

// Upload 上传附件到指定 category 的 bucket。
func (c *Client) Upload(ctx context.Context, category, objectName string, reader io.Reader, size int64, contentType string) error {
	bucket := c.BucketName(category)
	_, err := c.client.PutObject(ctx, bucket, objectName, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("上传到 %s/%s 失败: %w", bucket, objectName, err)
	}
	return nil
}

// PresignedGetURL 生成附件预签名下载 URL。
func (c *Client) PresignedGetURL(ctx context.Context, category, objectName string) (string, error) {
	bucket := c.BucketName(category)
	reqParams := make(url.Values)
	u, err := c.client.PresignedGetObject(ctx, bucket, objectName, c.presignExpiry, reqParams)
	if err != nil {
		return "", fmt.Errorf("生成预签名 URL 失败 (%s/%s): %w", bucket, objectName, err)
	}
	return u.String(), nil
}

// Remove 删除指定 category bucket 中的附件。
func (c *Client) Remove(ctx context.Context, category, objectName string) error {
	bucket := c.BucketName(category)
	if err := c.client.RemoveObject(ctx, bucket, objectName, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("删除 %s/%s 失败: %w", bucket, objectName, err)
	}
	return nil
}
