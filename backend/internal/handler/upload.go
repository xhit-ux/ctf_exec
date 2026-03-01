package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/ctf_exec/backend/internal/repository"
	"github.com/ctf_exec/backend/internal/utils"
)

// UploadHandler 文件上传处理器
type UploadHandler struct {
	userRepo *repository.UserRepository
}

// NewUploadHandler 创建上传处理器实例
func NewUploadHandler(userRepo *repository.UserRepository) *UploadHandler {
	return &UploadHandler{userRepo: userRepo}
}

// respondWithError 返回 JSON 格式的错误响应
func respondWithError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

// UploadAvatar 上传用户头像
func (h *UploadHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	// 仅允许 POST 请求
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "仅支持 POST 请求")
		return
	}

	// 从上下文获取用户信息（由 AuthMiddleware 设置）
	claims, ok := r.Context().Value("claims").(*utils.Claims)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "未授权")
		return
	}

	// 解析 multipart form，限制最大内存为 10MB
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		respondWithError(w, http.StatusBadRequest, "解析表单失败")
		return
	}

	// 获取上传的文件
	file, header, err := r.FormFile("avatar")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "获取文件失败")
		return
	}
	defer file.Close()

	// 验证图片文件
	if err := utils.ValidateImageFile(header, utils.MaxAvatarSize); err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 确定保存目录：static/avatars/{user_id}/
	destDir := filepath.Join("../static/avatars", claims.UserID)

	// 保存文件
	filePath, err := utils.SaveUploadedFile(file, header, destDir)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("保存文件失败: %v", err))
		return
	}

	// 修正存储路径：移除 ../ 前缀点（如果有）
	// 确保路径以 static/ 开头，这样前端拼接 localhost:8080/static/... 就能正确访问
	if len(filePath) >= 3 && filePath[:3] == "../" {
		filePath = filePath[3:]
	}

	// 更新数据库中的头像 URL
	if err := h.userRepo.UpdateAvatarURL(claims.UserID, filePath); err != nil {
		// 如果数据库更新失败，尝试删除已上传的文件
		utils.DeleteFile(filePath)
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("更新头像失败: %v", err))
		return
	}

	// 返回成功响应
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message":    "头像上传成功",
		"avatar_url": filePath,
	})
}

// UploadContestCover 上传比赛封面
func (h *UploadHandler) UploadContestCover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "仅支持 POST 请求")
		return
	}

	// 验证管理员权限（简单起见，这里假设已经在 AuthMiddleware 中验证了 userID）
	claims, ok := r.Context().Value("claims").(*utils.Claims)
	if !ok || claims.Role != "admin" {
		respondWithError(w, http.StatusForbidden, "仅管理员可上传比赛封面")
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		respondWithError(w, http.StatusBadRequest, "解析表单失败")
		return
	}

	file, header, err := r.FormFile("cover")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "获取文件失败")
		return
	}
	defer file.Close()

	if err := utils.ValidateImageFile(header, utils.MaxCoverSize); err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	destDir := "../static/contests"
	filePath, err := utils.SaveUploadedFile(file, header, destDir)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("保存文件失败: %v", err))
		return
	}

	// 修正存储路径
	if len(filePath) >= 3 && filePath[:3] == "../" {
		filePath = filePath[3:]
	}

	// 返回成功响应
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message":   "封面上传成功",
		"cover_url": filePath,
	})
}
