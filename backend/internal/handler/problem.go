package handler

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ctf_exec/backend/internal/config"
	"github.com/ctf_exec/backend/internal/middleware"
	minio "github.com/ctf_exec/backend/internal/minio"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/ctf_exec/backend/internal/utils"
	"github.com/gin-gonic/gin"
)

type ProblemHandler struct {
	repo             *repository.ProblemRepository
	contestRepo      *repository.ContestRepository
	contestAdminRepo repository.ContestAdminRepository
	minioClient      *minio.Client
	cfg              *config.Config
}

func NewProblemHandler(repo *repository.ProblemRepository, contestRepo *repository.ContestRepository, contestAdminRepo repository.ContestAdminRepository, minioClient *minio.Client, cfg *config.Config) *ProblemHandler {
	return &ProblemHandler{repo: repo, contestRepo: contestRepo, contestAdminRepo: contestAdminRepo, minioClient: minioClient, cfg: cfg}
}

func (h *ProblemHandler) CreateProblemDraftGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}
	role := models.NormalizeRole(claims.Role)
	if role != models.RoleRoot && role != models.RoleSiteAdmin {
		writeForbidden(c, "仅 site_admin/root 可创建草稿")
		return
	}

	var req models.CreateProblemDraftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c, "请求体格式错误")
		return
	}
	req.Category = normalizeProblemCategory(req.Category)
	if req.Category == "" {
		writeBadRequest(c, "category 不能为空")
		return
	}
	req.Type = normalizeProblemType(req.Type)
	if req.Type == "" {
		writeBadRequest(c, "type 仅支持 static/container_web/container_tcp")
		return
	}
	id, err := h.repo.CreateProblemDraft(&req, claims.UserID)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"problem_id": id})
}

func (h *ProblemHandler) SaveProblemBasicGin(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		writeBadRequest(c, "无效的题目ID")
		return
	}
	var req models.UpdateProblemBasicRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c, "请求体格式错误")
		return
	}
	req.Type = normalizeProblemType(req.Type)
	req.DeployMode = normalizeDeployMode(req.DeployMode)
	req.Category = normalizeProblemCategory(req.Category)
	if req.Type == "" || req.DeployMode == "" || req.Category == "" {
		writeBadRequest(c, "type/deploy_mode 不合法")
		return
	}
	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Category) == "" || strings.TrimSpace(req.Difficulty) == "" {
		writeBadRequest(c, "title/category/difficulty 不能为空")
		return
	}
	difficulty := strings.ToLower(strings.TrimSpace(req.Difficulty))
	rng, ok := models.GetDifficultyRange(difficulty)
	if !ok {
		writeBadRequest(c, "difficulty 仅支持 easy/medium/hard/expert")
		return
	}
	req.Difficulty = difficulty
	if req.PlatformScore < rng.Min || req.PlatformScore > rng.Max {
		writeBadRequest(c, fmt.Sprintf("platform_score 必须在 [%d,%d] 区间内（难度=%s）", rng.Min, rng.Max, difficulty))
		return
	}
	if len(req.Tags) == 0 && req.Category != "" {
		req.Tags = []string{req.Category}
	}
	if err := h.repo.UpdateProblemBasic(id, &req); err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "基础信息保存成功"})
}

func (h *ProblemHandler) SaveProblemFlagGin(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		writeBadRequest(c, "无效的题目ID")
		return
	}
	var req models.SaveProblemFlagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c, "请求体格式错误")
		return
	}
	req.FlagRule = normalizeFlagRule(req.FlagRule)
	req.FlagType = normalizeFlagFlagType(req.FlagType)
	req.FlagInject = normalizeFlagInject(req.FlagInject)
	if req.FlagRule == "" || req.FlagType == "" || req.FlagInject == "" {
		writeBadRequest(c, "flag_rule/flag_type/flag_inject 不合法")
		return
	}
	// flag_mode_explain.md Part 4：dynamic 不允许 regex/contains。
	if req.FlagType == "dynamic" {
		if req.FlagRule == utils.FlagRuleRegex || req.FlagRule == utils.FlagRuleContains {
			writeBadRequest(c, "dynamic flag 不支持 regex/contains 规则")
			return
		}
	}
	flagValue := strings.TrimSpace(req.FlagValue)
	switch req.FlagType {
	case "static":
		if flagValue == "" {
			writeBadRequest(c, "static flag 必须提供 flag_value")
			return
		}
		// regex/contains 校验 pattern 长度 + 可编译。
		if req.FlagRule == utils.FlagRuleRegex || req.FlagRule == utils.FlagRuleContains {
			if err := utils.ValidateFlagPattern(req.FlagRule, flagValue); err != nil {
				writeBadRequest(c, err.Error())
				return
			}
		}
		req.FlagValue = flagValue
	case "dynamic":
		// dynamic 不在此层提供 flag_value（运行期由实例生成）。
		req.FlagValue = ""
	}
	if err := h.repo.UpsertProblemFlag(id, &req); err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Flag 配置保存成功"})
}

func (h *ProblemHandler) SaveProblemContainerGin(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		writeBadRequest(c, "无效的题目ID")
		return
	}
	var req models.SaveProblemContainerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c, "请求体格式错误")
		return
	}
	if strings.TrimSpace(req.ImageName) == "" {
		writeBadRequest(c, "image_name 不能为空")
		return
	}
	if req.ChallengePort <= 0 {
		writeBadRequest(c, "challenge_port 必须大于 0")
		return
	}
	// sidecar 可选：要么两者都给，要么两者都为空。
	hasSidecarImage := req.SidecarImage != nil && strings.TrimSpace(*req.SidecarImage) != ""
	hasSidecarPort := req.SidecarPort != nil && *req.SidecarPort > 0
	if hasSidecarImage != hasSidecarPort {
		writeBadRequest(c, "sidecar 已默认注入 nginx:alpine，不再接受自定义配置")
		return
	}
	if err := h.repo.UpsertProblemContainer(id, &req); err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "容器配置保存成功"})
}

func (h *ProblemHandler) PublishProblemGin(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		writeBadRequest(c, "无效的题目ID")
		return
	}
	started, startErr := h.repo.StartImageBuildPipelineForProblem(id)
	if startErr != nil {
		writeError(c, startErr, http.StatusBadRequest)
		return
	}
	if started {
		c.JSON(http.StatusAccepted, gin.H{
			"message": "镜像构建任务已启动，请等待完成后再次点击发布",
			"status":  "building",
		})
		return
	}
	if err := h.repo.PublishProblem(id); err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "题目发布成功", "status": "published"})
}

func (h *ProblemHandler) SaveProblemImageGin(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		writeBadRequest(c, "无效的题目ID")
		return
	}

	cfg, cfgErr := config.LoadConfig()
	if cfgErr != nil {
		writeError(c, cfgErr, http.StatusInternalServerError)
		return
	}

	if err := c.Request.ParseMultipartForm(cfg.Upload.MaxImagePackageSize); err != nil {
		writeBadRequest(c, "解析表单失败")
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		writeBadRequest(c, "请上传构建文件，字段名为 file")
		return
	}
	if file.Size <= 0 {
		writeBadRequest(c, "构建文件不能为空")
		return
	}
	if cfg.Upload.MaxImagePackageSize > 0 && file.Size > cfg.Upload.MaxImagePackageSize {
		writeBadRequest(c, fmt.Sprintf("构建文件超过限制，最大允许 %d MB", cfg.Upload.MaxImagePackageSize/(1024*1024)))
		return
	}

	buildDir := filepath.Join(cfg.Upload.StaticPath, "images", "problems", id)
	if mkErr := os.MkdirAll(buildDir, 0o755); mkErr != nil {
		writeError(c, mkErr, http.StatusInternalServerError)
		return
	}
	dst := filepath.Join(buildDir, filepath.Base(file.Filename))
	if saveErr := c.SaveUploadedFile(file, dst); saveErr != nil {
		writeError(c, saveErr, http.StatusInternalServerError)
		return
	}

	imageRepo := strings.TrimSpace(c.PostForm("image_repo"))
	if imageRepo == "" {
		imageRepo = fmt.Sprintf("ctf-exec/%s", id)
	}
	imageTag := strings.TrimSpace(c.PostForm("image_tag"))
	uploadType := strings.TrimSpace(c.PostForm("upload_type"))
	if uploadType == "" {
		uploadType = "docker_save"
	}
	log.Printf("[image-upload] problem=%s file=%s size=%d upload_type=%s saved_to=%s", id, file.Filename, file.Size, uploadType, dst)

	task, repoErr := h.repo.CreateImageBuildTask(id, dst, file.Size, imageRepo, imageTag)
	if repoErr != nil {
		writeError(c, repoErr, http.StatusInternalServerError)
		return
	}
	if task == nil {
		writeInternal(c, "构建任务初始化失败")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "镜像包已暂存，发布时触发构建",
		"status":  "uploaded",
		"task":    task,
	})
}

func (h *ProblemHandler) SaveProblemAttachmentGin(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		writeBadRequest(c, "无效的题目ID")
		return
	}

	detail, err := h.repo.GetAdminProblemByID(id)
	if err != nil {
		writeError(c, err, http.StatusNotFound)
		return
	}

	if err := c.Request.ParseMultipartForm(h.cfg.Upload.MaxAttachmentSize); err != nil {
		writeBadRequest(c, "解析表单失败")
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		writeBadRequest(c, "获取附件失败")
		return
	}
	defer file.Close()

	if err := utils.ValidateAttachmentArchive(header, h.cfg.Upload.MaxAttachmentSize); err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}
	ext := utils.AttachmentExtension(header.Filename)
	if ext == "" {
		writeBadRequest(c, "仅支持上传压缩包文件")
		return
	}

	// 清理旧附件（双路：本地或 MinIO）
	if existing, lookupErr := h.repo.GetProblemAttachment(id); lookupErr != nil {
		writeError(c, lookupErr, http.StatusInternalServerError)
		return
	} else if existing != nil {
		if existing.BucketName == "local" {
			oldPath := resolveAttachmentFSPath(h.cfg.Upload.StaticPath, existing.ObjectKey)
			if oldPath != "" {
				_ = utils.DeleteFile(oldPath)
			}
		} else if h.cfg.MinIO.Enabled && h.minioClient != nil {
			cat, objName := parseMinIOObjectKey(existing.ObjectKey)
			_ = h.minioClient.Remove(c.Request.Context(), cat, objName)
		}
	}

	category := normalizeAttachmentCategory(detail.Category)
	fileName := fmt.Sprintf("%s%s", id, ext)

	var bucketName, objectKey string
	var written int64
	var sha string

	if h.cfg.MinIO.Enabled && h.minioClient != nil {
		// MinIO 上传
		objectKey = filepath.ToSlash(filepath.Join(category, fileName))
		bucketName = h.minioClient.BucketName(category)

		hash256 := sha256.New()
		teeReader := io.TeeReader(file, hash256)

		if err := h.minioClient.Upload(c.Request.Context(), category, fileName, teeReader, header.Size, header.Header.Get("Content-Type")); err != nil {
			writeError(c, err, http.StatusInternalServerError)
			return
		}

		// MinIO: TeeReader 边上传边计算 SHA256
		written = header.Size
		sha = fmt.Sprintf("%x", hash256.Sum(nil))
	} else {
		// 本地存储
		storagePath := filepath.ToSlash(filepath.Join("static", "attachments", category, fileName))
		fsPath := filepath.Join(h.cfg.Upload.StaticPath, "attachments", category, fileName)

		var shaErr error
		written, sha, shaErr = utils.SaveUploadedFileAs(file, fsPath)
		if shaErr != nil {
			writeError(c, shaErr, http.StatusInternalServerError)
			return
		}
		bucketName = "local"
		objectKey = storagePath
	}

	item, err := h.repo.ReplaceProblemAttachment(id, header.Filename, bucketName, objectKey, written, sha)
	if err != nil {
		if bucketName == "local" {
			_ = utils.DeleteFile(filepath.Join(h.cfg.Upload.StaticPath, "attachments", category, fileName))
		} else {
			_ = h.minioClient.Remove(c.Request.Context(), category, fileName)
		}
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "附件上传成功",
		"attachment": gin.H{
			"id":           item.ID,
			"file_name":    item.FileName,
			"file_size":    item.FileSize,
			"sha256":       item.Sha256,
			"file_path":    item.ObjectKey,
			"download_url": fmt.Sprintf("/api/problems/%s/attachment", id),
		},
	})
}

// GetProblemAttachmentDownloadGin 附件下载端点：后端流式转发，不暴露 MinIO 地址给客户端。
func (h *ProblemHandler) GetProblemAttachmentDownloadGin(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		writeBadRequest(c, "无效的题目ID")
		return
	}

	attachment, err := h.repo.GetProblemAttachment(id)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}
	if attachment == nil {
		writeNotFound(c, "该题目没有附件")
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, attachment.FileName))
	c.Header("Content-Length", fmt.Sprintf("%d", attachment.FileSize))

	if attachment.BucketName == "local" {
		fsPath := resolveAttachmentFSPath(h.cfg.Upload.StaticPath, attachment.ObjectKey)
		if fsPath == "" {
			writeInternal(c, "无法解析本地附件路径")
			return
		}
		c.File(fsPath)
		return
	}

	if h.minioClient == nil {
		writeInternal(c, "MinIO 客户端未初始化")
		return
	}
	category, objectName := parseMinIOObjectKey(attachment.ObjectKey)
	obj, err := h.minioClient.GetObject(c.Request.Context(), category, objectName)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}
	defer obj.Close()

	info, statErr := obj.Stat()
	if statErr == nil {
		if info.ContentType != "" {
			c.Header("Content-Type", info.ContentType)
		}
		if info.Size > 0 {
			c.Header("Content-Length", fmt.Sprintf("%d", info.Size))
		}
	}
	if c.GetHeader("Content-Type") == "" {
		c.Header("Content-Type", "application/octet-stream")
	}

	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, obj)
}

// parseMinIOObjectKey 将 object_key（格式: "category/filename"）拆分为 category 和文件名。
func parseMinIOObjectKey(objectKey string) (string, string) {
	parts := strings.SplitN(objectKey, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "misc", objectKey
}

func (h *ProblemHandler) GetProblemBuildTaskGin(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		writeBadRequest(c, "无效的题目ID")
		return
	}

	task, err := h.repo.GetLatestImageBuildTask(id)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	if task == nil {
		c.JSON(http.StatusOK, gin.H{
			"task":   nil,
			"status": "not_found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"task":   task,
		"status": task.Status,
	})
}

func (h *ProblemHandler) GetProblemInstanceGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}
	problemID := strings.TrimSpace(c.Param("id"))
	if problemID == "" {
		writeBadRequest(c, "无效的题目ID")
		return
	}

	instance, err := h.repo.GetChallengeInstance(problemID, claims.UserID)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}
	if instance == nil {
		c.JSON(http.StatusOK, gin.H{"status": "idle", "instance": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": instance.Status, "instance": instance})
}

func (h *ProblemHandler) StartProblemInstanceGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}
	problemID := strings.TrimSpace(c.Param("id"))
	if problemID == "" {
		writeBadRequest(c, "无效的题目ID")
		return
	}

	instance, err := h.repo.StartChallengeInstance(c.Request.Context(), problemID, claims.UserID)
	if err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "实例启动成功",
		"status":   instance.Status,
		"instance": instance,
	})
}

func (h *ProblemHandler) DestroyProblemInstanceGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}
	problemID := strings.TrimSpace(c.Param("id"))
	if problemID == "" {
		writeBadRequest(c, "无效的题目ID")
		return
	}

	if err := h.repo.DestroyChallengeInstance(problemID, claims.UserID); err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "实例销毁成功", "status": "destroyed"})
}

func (h *ProblemHandler) ExtendProblemInstanceGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}
	problemID := strings.TrimSpace(c.Param("id"))
	if problemID == "" {
		writeBadRequest(c, "无效的题目ID")
		return
	}
	instance, err := h.repo.ExtendChallengeInstance(c.Request.Context(), problemID, claims.UserID)
	if err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "实例已延期", "instance": instance})
}

func (h *ProblemHandler) DeleteProblemGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}
	role := models.NormalizeRole(claims.Role)
	if role != models.RoleRoot && role != models.RoleSiteAdmin {
		writeForbidden(c, "仅 site_admin/root 可删除题目")
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		writeBadRequest(c, "无效的题目ID")
		return
	}
	if err := h.repo.DeleteProblem(id); err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "题目删除成功"})
}

func (h *ProblemHandler) ListAdminProblemsGin(c *gin.Context) {
	claims, _ := middleware.GetClaims(c)
	createdBy := ""
	if claims != nil && claims.Role == "contest_admin" {
		createdBy = claims.UserID
	}
	items, err := h.repo.ListAdminProblems(
		strings.TrimSpace(c.Query("keyword")),
		strings.TrimSpace(c.Query("scope")),
		strings.TrimSpace(c.Query("status")),
		createdBy,
	)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *ProblemHandler) GetAdminProblemDetailGin(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		writeBadRequest(c, "无效的题目ID")
		return
	}
	item, err := h.repo.GetAdminProblemByID(id)
	if err != nil {
		writeError(c, err, http.StatusNotFound)
		return
	}
	if item.Attachment != nil {
		item.Attachment.DownloadURL = fmt.Sprintf("/api/problems/%s/attachment", id)
	}
	c.JSON(http.StatusOK, item)
}

func (h *ProblemHandler) GetProblemsGin(c *gin.Context) {
	if claims, err := middleware.GetClaims(c); err == nil {
		items, qErr := h.repo.GetProblemsWithStatus(claims.UserID)
		if qErr != nil {
			writeInternal(c, "获取题目列表失败")
			return
		}
		c.JSON(http.StatusOK, items)
		return
	}

	items, err := h.repo.GetProblems()
	if err != nil {
		writeInternal(c, "获取题目列表失败")
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *ProblemHandler) GetProblemDetailGin(c *gin.Context) {
	id := strings.TrimSpace(c.Query("id"))
	if id == "" {
		id = strings.TrimSpace(c.Param("id"))
	}
	if id == "" {
		writeBadRequest(c, "缺少 id 参数")
		return
	}

	userID := ""
	if claims, err := middleware.GetClaims(c); err == nil {
		userID = claims.UserID
	}

	item, err := h.repo.GetProblemByIDForUser(id, userID)
	if err != nil {
		writeError(c, err, http.StatusNotFound)
		return
	}
	if item.Attachment != nil {
		item.Attachment.DownloadURL = fmt.Sprintf("/api/problems/%s/attachment", id)
	}
	c.JSON(http.StatusOK, item)
}

// ============================================================
// 提示子系统 handler
// ============================================================

// ListProblemHintsGin 列出该题目的提示（用户视角，含 is_unlocked，未解锁不返回 content）。
func (h *ProblemHandler) ListProblemHintsGin(c *gin.Context) {
	problemID := strings.TrimSpace(c.Param("id"))
	if problemID == "" {
		writeBadRequest(c, "无效的题目ID")
		return
	}
	userID := ""
	if claims, err := middleware.GetClaims(c); err == nil {
		userID = claims.UserID
	}
	var contestID *string
	if cid := strings.TrimSpace(c.Query("contest_id")); cid != "" {
		contestID = &cid
	}
	hints, err := h.repo.ListProblemHints(problemID, userID, contestID)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}
	for _, h := range hints {
		if !h.IsUnlocked {
			h.Content = ""
		}
	}
	c.JSON(http.StatusOK, gin.H{"hints": hints})
}

// UnlockHintGin 用户主动解锁某条提示。
func (h *ProblemHandler) UnlockHintGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}
	hintID := strings.TrimSpace(c.Param("hintID"))
	if hintID == "" {
		writeBadRequest(c, "无效的提示ID")
		return
	}
	var contestID *string
	if cid := strings.TrimSpace(c.Query("contest_id")); cid != "" {
		contestID = &cid
	}
	hint, _, err := h.repo.UnlockHint(claims.UserID, hintID, contestID)
	if err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}
	c.JSON(http.StatusOK, hint)
}

// CreateProblemHintGin 管理端创建提示。
func (h *ProblemHandler) CreateProblemHintGin(c *gin.Context) {
	problemID := strings.TrimSpace(c.Param("id"))
	if problemID == "" {
		writeBadRequest(c, "无效的题目ID")
		return
	}
	var req models.CreateHintRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c, "请求体格式错误")
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		writeBadRequest(c, "content 不能为空")
		return
	}
	if req.Cost < 0 {
		writeBadRequest(c, "cost 不能为负数")
		return
	}
	hint, err := h.repo.CreateHint(problemID, &req)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusCreated, hint)
}

// UpdateProblemHintGin 管理端更新提示。
func (h *ProblemHandler) UpdateProblemHintGin(c *gin.Context) {
	hintID := strings.TrimSpace(c.Param("hintID"))
	if hintID == "" {
		writeBadRequest(c, "无效的提示ID")
		return
	}
	var req models.UpdateHintRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c, "请求体格式错误")
		return
	}
	if req.Cost < 0 {
		writeBadRequest(c, "cost 不能为负数")
		return
	}
	if err := h.repo.UpdateHint(hintID, &req); err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "提示更新成功"})
}

// DeleteProblemHintGin 管理端删除提示。
func (h *ProblemHandler) DeleteProblemHintGin(c *gin.Context) {
	hintID := strings.TrimSpace(c.Param("hintID"))
	if hintID == "" {
		writeBadRequest(c, "无效的提示ID")
		return
	}
	if err := h.repo.DeleteHint(hintID); err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "提示删除成功"})
}

// ============================================================
// 前置依赖子系统 handler
// ============================================================

// SetProblemPrerequisitesGin 全量替换某题目的前置依赖列表（管理端）。
func (h *ProblemHandler) SetProblemPrerequisitesGin(c *gin.Context) {
	problemID := strings.TrimSpace(c.Param("id"))
	if problemID == "" {
		writeBadRequest(c, "无效的题目ID")
		return
	}
	var req models.SetPrerequisitesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c, "请求体格式错误")
		return
	}
	if err := h.repo.SetProblemPrerequisites(problemID, req.PrerequisiteIDs); err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "前置依赖设置成功"})
}

// ListProblemPrerequisitesGin 列出某题目的前置依赖（用户视角，含 is_solved）。
func (h *ProblemHandler) ListProblemPrerequisitesGin(c *gin.Context) {
	problemID := strings.TrimSpace(c.Param("id"))
	if problemID == "" {
		writeBadRequest(c, "无效的题目ID")
		return
	}
	userID := ""
	if claims, err := middleware.GetClaims(c); err == nil {
		userID = claims.UserID
	}
	items, err := h.repo.ListProblemPrerequisites(problemID, userID)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, gin.H{"prerequisites": items})
}

func normalizeProblemType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "static", "container_web", "container_tcp":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func normalizeProblemCategory(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case "web", "pwn", "crypto", "reverse", "misc":
		return v
	case "rev":
		return "reverse"
	default:
		return ""
	}
}

func normalizeAttachmentCategory(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case "web":
		return "web"
	case "pwn":
		return "pwn"
	case "crypto":
		return "crypto"
	case "reverse", "rev":
		return "reverse"
	case "misc":
		return "misc"
	default:
		return "misc"
	}
}

func resolveAttachmentFSPath(staticRoot, objectKey string) string {
	cleanKey := strings.TrimSpace(objectKey)
	if cleanKey == "" {
		return ""
	}
	if strings.HasPrefix(cleanKey, "static/") {
		cleanKey = strings.TrimPrefix(cleanKey, "static/")
	}
	cleanKey = filepath.Clean(cleanKey)
	if cleanKey == "." || strings.HasPrefix(cleanKey, "..") || filepath.IsAbs(cleanKey) {
		return ""
	}
	return filepath.Join(staticRoot, cleanKey)
}

func normalizeDeployMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "shared", "per_user":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func normalizeFlagRule(raw string) string {
	return utils.NormalizeFlagRule(raw)
}

func normalizeFlagFlagType(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return "static"
	}
	if v == "static" || v == "dynamic" {
		return v
	}
	return ""
}

func normalizeFlagInject(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return "env"
	}
	if v == "env" || v == "sidecar" {
		return v
	}
	return ""
}

func normalizeProblemStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "draft", "ready", "hidden":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}
