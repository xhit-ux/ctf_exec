package utils

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ctf_exec/backend/internal/apperr"
	"github.com/google/uuid"
)

// 文件大小限制
const (
	MaxAvatarSize     = 2 * 1024 * 1024  // 2MB
	MaxCoverSize      = 5 * 1024 * 1024  // 5MB
	MaxAttachmentSize = 50 * 1024 * 1024 // 50MB
)

// 允许的图片格式
var AllowedImageExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".webp": true,
}

var allowedImageMIMEs = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
}

// ValidateImageFile 验证图片文件
func ValidateImageFile(file multipart.File, header *multipart.FileHeader, maxSize int64) error {
	// 检查文件大小
	if header.Size > maxSize {
		return apperr.New(apperr.ErrBadRequest, fmt.Sprintf("文件大小超过限制，最大允许 %d MB", maxSize/(1024*1024)))
	}

	// 检查文件扩展名
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !AllowedImageExtensions[ext] {
		return apperr.New(apperr.ErrBadRequest, "不支持的文件格式，仅支持: jpg, jpeg, png, gif, webp")
	}

	// 检查请求头 MIME 类型（弱校验）
	contentType := header.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		return apperr.New(apperr.ErrBadRequest, "文件类型必须是图片")
	}

	// 基于 magic bytes 做强校验，防止伪造 Content-Type 绕过。
	probe := make([]byte, 512)
	n, err := io.ReadFull(file, probe)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return apperr.New(apperr.ErrBadRequest, "读取文件头失败")
	}

	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return apperr.New(apperr.ErrBadRequest, "重置文件指针失败")
	}

	detectedType := http.DetectContentType(probe[:n])
	if !allowedImageMIMEs[detectedType] {
		return apperr.New(apperr.ErrBadRequest, "文件内容不是受支持的图片格式")
	}

	return nil
}

// SaveUploadedFile 保存上传的文件
// destDir: 目标目录，如 "static/avatars/user-id"
// 返回相对路径，如 "static/avatars/user-id/timestamp_filename.jpg"
func SaveUploadedFile(file multipart.File, header *multipart.FileHeader, destDir string) (string, error) {
	// 确保目标目录存在
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}

	// 生成唯一文件名：时间戳_uuid_原始文件名
	timestamp := time.Now().Unix()
	ext := filepath.Ext(header.Filename)
	uniqueID := uuid.New().String()[:8]
	filename := fmt.Sprintf("%d_%s%s", timestamp, uniqueID, ext)

	// 清理路径，防止路径遍历攻击
	destPath := filepath.Join(destDir, filename)
	destPath = filepath.Clean(destPath)

	// 创建目标文件
	dst, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("创建文件失败: %w", err)
	}
	defer dst.Close()

	// 复制文件内容
	if _, err := io.Copy(dst, file); err != nil {
		return "", fmt.Errorf("保存文件失败: %w", err)
	}

	// 返回相对路径（用于存储到数据库）
	return destPath, nil
}

// DeleteFile 删除文件
func DeleteFile(filePath string) error {
	// 清理路径
	filePath = filepath.Clean(filePath)

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil // 文件不存在，视为成功
	}

	// 删除文件
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("删除文件失败: %w", err)
	}

	return nil
}
