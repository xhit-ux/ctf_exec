package handler

import (
	"net/http"
	"strings"

	"github.com/ctf_exec/backend/internal/middleware"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/gin-gonic/gin"
)

type SiteConfigHandler struct {
	repo *repository.SiteConfigRepository
}

func NewSiteConfigHandler(repo *repository.SiteConfigRepository) *SiteConfigHandler {
	return &SiteConfigHandler{repo: repo}
}

// GetSiteConfigGin root 查看站点配置。
func (h *SiteConfigHandler) GetSiteConfigGin(c *gin.Context) {
	cfg, err := h.repo.GetSiteConfig()
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, cfg)
}

// UpdateSiteConfigGin root 更新站点配置。
func (h *SiteConfigHandler) UpdateSiteConfigGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}

	var req models.UpdateSiteConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体格式错误"})
		return
	}

	if strings.TrimSpace(req.SiteName) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "site_name 不能为空"})
		return
	}

	visibility := strings.ToLower(strings.TrimSpace(req.DefaultContestVisibility))
	if visibility != "public" && visibility != "invite_only" && visibility != "private" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "default_contest_visibility 仅支持 public/invite_only/private"})
		return
	}
	req.DefaultContestVisibility = visibility

	if err := h.repo.UpdateSiteConfig(claims.UserID, &req, realClientIP(c)); err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "站点配置更新成功"})
}
