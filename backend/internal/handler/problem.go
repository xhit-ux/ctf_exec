package handler

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ctf_exec/backend/internal/config"
	"github.com/ctf_exec/backend/internal/middleware"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/ctf_exec/backend/internal/utils"
	"github.com/gin-gonic/gin"
)

type ProblemHandler struct {
	repo             *repository.ProblemRepository
	contestRepo      *repository.ContestRepository
	contestAdminRepo repository.ContestAdminRepository
}

func NewProblemHandler(repo *repository.ProblemRepository, contestRepo *repository.ContestRepository, contestAdminRepo repository.ContestAdminRepository) *ProblemHandler {
	return &ProblemHandler{repo: repo, contestRepo: contestRepo, contestAdminRepo: contestAdminRepo}
}

func (h *ProblemHandler) CreateProblemGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}

	role := models.NormalizeRole(claims.Role)
	if role != models.RoleRoot && role != models.RoleSiteAdmin {
		writeForbidden(c, "仅 site_admin/root 可创建题目")
		return
	}

	var req models.CreateProblemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c, "请求体格式错误")
		return
	}

	if err := validateProblemCreateUpdate(&req); err != nil {
		writeBadRequest(c, err.Error())
		return
	}
	req.CreatedBy = claims.UserID

	item, err := h.repo.CreateProblem(&req)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusCreated, item)
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
	if strings.TrimSpace(req.Category) == "" {
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
	if req.Type == "" || req.DeployMode == "" {
		writeBadRequest(c, "type/deploy_mode 不合法")
		return
	}
	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Category) == "" || strings.TrimSpace(req.Difficulty) == "" {
		writeBadRequest(c, "title/category/difficulty 不能为空")
		return
	}
	if req.BaseScore < 0 {
		writeBadRequest(c, "base_score 不能为负数")
		return
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
	var req models.CreateFlagRequest
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
	if req.FlagType == "static" && strings.TrimSpace(req.FlagPlain) == "" {
		writeBadRequest(c, "static flag 不能为空")
		return
	}
	if err := h.repo.UpsertProblemFlag(id, &req); err != nil {
		writeError(c, err, http.StatusInternalServerError)
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
	var req models.CreateContainerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c, "请求体格式错误")
		return
	}
	if strings.TrimSpace(req.ImageName) == "" || strings.TrimSpace(req.SidecarImage) == "" {
		writeBadRequest(c, "image_name/sidecar_image 不能为空")
		return
	}
	if req.ChallengePort <= 0 || req.SidecarPort <= 0 {
		writeBadRequest(c, "challenge_port/sidecar_port 必须大于 0")
		return
	}
	if err := h.repo.UpsertProblemContainer(id, &req); err != nil {
		writeError(c, err, http.StatusInternalServerError)
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

	cfg, cfgErr := config.LoadConfig()
	if cfgErr != nil {
		writeError(c, cfgErr, http.StatusInternalServerError)
		return
	}
	if cfg.MinIO.Enabled {
		writeBadRequest(c, "MinIO 附件上传暂未启用")
		return
	}

	detail, err := h.repo.GetAdminProblemByID(id)
	if err != nil {
		writeError(c, err, http.StatusNotFound)
		return
	}

	if err := c.Request.ParseMultipartForm(cfg.Upload.MaxAttachmentSize); err != nil {
		writeBadRequest(c, "解析表单失败")
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		writeBadRequest(c, "获取附件失败")
		return
	}
	defer file.Close()

	if err := utils.ValidateAttachmentArchive(header, cfg.Upload.MaxAttachmentSize); err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}
	ext := utils.AttachmentExtension(header.Filename)
	if ext == "" {
		writeBadRequest(c, "仅支持上传压缩包文件")
		return
	}

	attachments, err := h.repo.ListProblemAttachments(id)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}
	for _, attachment := range attachments {
		if attachment == nil {
			continue
		}
		oldPath := resolveAttachmentFSPath(cfg.Upload.StaticPath, attachment.ObjectKey)
		if oldPath != "" {
			_ = utils.DeleteFile(oldPath)
		}
	}

	category := normalizeAttachmentCategory(detail.Category)
	fileName := fmt.Sprintf("%s%s", id, ext)
	storagePath := filepath.ToSlash(filepath.Join("static", "attachments", category, fileName))
	fsPath := filepath.Join(cfg.Upload.StaticPath, "attachments", category, fileName)

	written, sha, err := utils.SaveUploadedFileAs(file, fsPath)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	item, err := h.repo.ReplaceProblemAttachment(id, header.Filename, "local", storagePath, written, sha)
	if err != nil {
		_ = utils.DeleteFile(fsPath)
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "附件上传成功",
		"attachment": gin.H{
			"id":        item.ID,
			"file_name": item.FileName,
			"file_size": item.FileSize,
			"sha256":    item.Sha256,
			"file_path": item.ObjectKey,
		},
	})
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

	instance, err := h.repo.StartChallengeInstance(problemID, claims.UserID)
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

func (h *ProblemHandler) UpdateProblemGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}
	role := models.NormalizeRole(claims.Role)
	if role != models.RoleRoot && role != models.RoleSiteAdmin {
		writeForbidden(c, "仅 site_admin/root 可更新题目")
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		writeBadRequest(c, "无效的题目ID")
		return
	}

	var req models.UpdateProblemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c, "请求体格式错误")
		return
	}
	if err := validateProblemCreateUpdate((*models.CreateProblemRequest)(&models.CreateProblemRequest{
		Title:       req.Title,
		Category:    req.Category,
		Type:        req.Type,
		Description: req.Description,
		BaseScore:   req.BaseScore,
		Difficulty:  req.Difficulty,
		DeployMode:  req.DeployMode,
		UseKata:     req.UseKata,
		Status:      req.Status,
		Tags:        req.Tags,
		Flag:        req.Flag,
		Container:   req.Container,
	})); err != nil {
		writeBadRequest(c, err.Error())
		return
	}

	if err := h.repo.UpdateProblem(id, &req); err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "题目更新成功"})
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
	items, err := h.repo.ListAdminProblems(
		strings.TrimSpace(c.Query("keyword")),
		strings.TrimSpace(c.Query("scope")),
		strings.TrimSpace(c.Query("status")),
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

	item, err := h.repo.GetProblemByID(id)
	if err != nil {
		writeError(c, err, http.StatusNotFound)
		return
	}
	c.JSON(http.StatusOK, item)
}

func validateProblemCreateUpdate(req *models.CreateProblemRequest) error {
	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Category) == "" || strings.TrimSpace(req.Difficulty) == "" {
		return apperrErr("title/category/difficulty 不能为空")
	}
	if req.BaseScore < 0 {
		return apperrErr("base_score 不能为负数")
	}
	t := normalizeProblemType(req.Type)
	if t == "" {
		return apperrErr("type 仅支持 static/container_web/container_tcp")
	}
	req.Type = t
	dm := normalizeDeployMode(req.DeployMode)
	if dm == "" {
		return apperrErr("deploy_mode 仅支持 shared/per_user")
	}
	req.DeployMode = dm

	status := normalizeProblemStatus(req.Status)
	if status == "" {
		return apperrErr("status 仅支持 draft/ready/hidden")
	}
	req.Status = status

	req.Flag.FlagRule = normalizeFlagRule(req.Flag.FlagRule)
	req.Flag.FlagType = normalizeFlagFlagType(req.Flag.FlagType)
	req.Flag.FlagInject = normalizeFlagInject(req.Flag.FlagInject)
	if req.Flag.FlagRule == "" || req.Flag.FlagType == "" || req.Flag.FlagInject == "" {
		return apperrErr("flag_rule/flag_type/flag_inject 不合法")
	}

	if req.DeployMode == "shared" && req.Flag.FlagType == "dynamic" {
		return apperrErr("shared 模式下不允许 dynamic flag")
	}
	if req.Flag.FlagType == "static" && strings.TrimSpace(req.Flag.FlagPlain) == "" {
		return apperrErr("static flag 不能为空")
	}

	if req.Container != nil {
		if strings.TrimSpace(req.Container.ImageName) == "" || strings.TrimSpace(req.Container.SidecarImage) == "" {
			return apperrErr("容器配置 image_name/sidecar_image 不能为空")
		}
		if req.Container.ChallengePort <= 0 || req.Container.SidecarPort <= 0 {
			return apperrErr("容器配置端口必须大于 0")
		}
		if req.Type == "container_web" && (req.Container.IngressHostTpl == nil || strings.TrimSpace(*req.Container.IngressHostTpl) == "") {
			return apperrErr("container_web 必须配置 ingress_host_tpl")
		}
	}

	return nil
}

type simpleErr string

func (e simpleErr) Error() string { return string(e) }

func apperrErr(msg string) error { return simpleErr(msg) }

func normalizeProblemType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "static", "container_web", "container_tcp":
		return strings.ToLower(strings.TrimSpace(raw))
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
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return "exact"
	}
	if v == "exact" || v == "case_insensitive" {
		return v
	}
	return ""
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
