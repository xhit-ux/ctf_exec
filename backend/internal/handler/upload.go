package handler

import (
	"path/filepath"

	"github.com/ctf_exec/backend/internal/middleware"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/ctf_exec/backend/internal/utils"
	"github.com/gin-gonic/gin"
)

// UploadHandler 文件上传处理器
type UploadHandler struct {
	userRepo *repository.UserRepository
}

// NewUploadHandler 创建上传处理器实例
func NewUploadHandler(userRepo *repository.UserRepository) *UploadHandler {
	return &UploadHandler{userRepo: userRepo}
}

// UploadAvatarGin Gin 版本的上传用户头像
func (h *UploadHandler) UploadAvatarGin(c *gin.Context) {
	// 从 Gin 上下文中获取 claims
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}

	// 解析 multipart form，限制最大内存为 10MB
	if err := c.Request.ParseMultipartForm(10 << 20); err != nil {
		c.JSON(400, gin.H{"error": "解析表单失败"})
		return
	}

	// 获取上传的文件
	file, header, err := c.Request.FormFile("avatar")
	if err != nil {
		c.JSON(400, gin.H{"error": "获取文件失败"})
		return
	}
	defer file.Close()

	// 验证图片文件
	if err := utils.ValidateImageFile(file, header, utils.MaxAvatarSize); err != nil {
		writeError(c, err, 400)
		return
	}

	// 确定保存目录：static/avatars/{user_id}/
	destDir := filepath.Join("../static/avatars", claims.UserID)

	// 保存文件
	filePath, err := utils.SaveUploadedFile(file, header, destDir)
	if err != nil {
		writeError(c, err, 500)
		return
	}

	// 修正存储路径：移除 ../ 前缀点（如果有）
	if len(filePath) >= 3 && filePath[:3] == "../" {
		filePath = filePath[3:]
	}

	// 更新数据库中的头像 URL
	if err := h.userRepo.UpdateAvatarURL(claims.UserID, filePath); err != nil {
		// 如果数据库更新失败，尝试删除已上传的文件
		utils.DeleteFile(filePath)
		writeError(c, err, 500)
		return
	}

	// 返回成功响应
	c.JSON(200, gin.H{
		"message":    "头像上传成功",
		"avatar_url": filePath,
	})
}

// UploadContestCoverGin Gin 版本的上传比赛封面
func (h *UploadHandler) UploadContestCoverGin(c *gin.Context) {
	// 从 Gin 上下文中获取 claims
	_, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}

	// 解析 multipart form
	if err := c.Request.ParseMultipartForm(10 << 20); err != nil {
		c.JSON(400, gin.H{"error": "解析表单失败"})
		return
	}

	file, header, err := c.Request.FormFile("cover")
	if err != nil {
		c.JSON(400, gin.H{"error": "获取文件失败"})
		return
	}
	defer file.Close()

	if err := utils.ValidateImageFile(file, header, utils.MaxCoverSize); err != nil {
		writeError(c, err, 400)
		return
	}

	destDir := "../static/contests"
	filePath, err := utils.SaveUploadedFile(file, header, destDir)
	if err != nil {
		writeError(c, err, 500)
		return
	}

	// 修正存储路径
	if len(filePath) >= 3 && filePath[:3] == "../" {
		filePath = filePath[3:]
	}

	// 返回成功响应
	c.JSON(200, gin.H{
		"message":   "封面上传成功",
		"cover_url": filePath,
	})
}
