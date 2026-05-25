package models

import (
	"encoding/json"
	"time"
)

// ============================================================
// 实体层 — 与 DB 表 1:1 对应
// ============================================================

// Problem 题目模板（platform scope）。
// v2.1 关键字段：platform_score 受难度区间约束；动态计分配置不在此层。
type Problem struct {
	ID              string     `json:"id"`
	Title           string     `json:"title"`
	Category        string     `json:"category"`
	Type            string     `json:"type"`
	Description     *string    `json:"description,omitempty"`
	Author          *string    `json:"author,omitempty"`
	PlatformScore   int        `json:"platform_score"`
	Difficulty      string     `json:"difficulty"`
	DeployMode      string     `json:"deploy_mode"`
	UseKata         bool       `json:"use_kata"`
	Status          string     `json:"status"`
	CreatedBy       *string    `json:"created_by,omitempty"`
	SourceProblemID *string    `json:"source_problem_id,omitempty"`
	CreatedAt       *time.Time `json:"created_at,omitempty"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`

	Tags          []string           `json:"tags,omitempty"`
	Attachment    *Attachment        `json:"attachment,omitempty"`
	Hints         []*ProblemHint     `json:"hints,omitempty"`
	Prerequisites []*PrerequisiteRef `json:"prerequisites,omitempty"`
	Flag          *ProblemFlag       `json:"flag,omitempty"`
	Container     *ProblemContainer  `json:"container,omitempty"`

	IsSolved    bool            `json:"is_solved"`
	IsLocked    bool            `json:"is_locked,omitempty"`
	SolvesCount int             `json:"solves_count,omitempty"`
	Instance    *InstanceView   `json:"instance,omitempty"`
	Connection  *ConnectionInfo `json:"connection_info,omitempty"`
	BuildStatus *string         `json:"build_status,omitempty"`
}

// ProblemFlag flag 配置（problem_flags 表）。
type ProblemFlag struct {
	ProblemID          string  `json:"problem_id"`
	FlagType           string  `json:"flag_type"`
	FlagRule           string  `json:"flag_rule"`
	FlagHash           *string `json:"-"`
	FlagPattern        *string `json:"flag_pattern,omitempty"`
	FlagExample        *string `json:"flag_example,omitempty"`
	FlagInject         string  `json:"flag_inject"`
	FlagEnvKey         string  `json:"flag_env_key"`
	FlagSidecarPattern *string `json:"flag_sidecar_pattern,omitempty"`
}

// ProblemContainer 容器配置（problem_containers 表）。sidecar 字段可选。
type ProblemContainer struct {
	ProblemID      string          `json:"problem_id"`
	ImageName      string          `json:"image_name"`
	ImageTag       string          `json:"image_tag"`
	ChallengePort  int             `json:"challenge_port"`
	SidecarImage   *string         `json:"sidecar_image,omitempty"`
	SidecarPort    *int            `json:"sidecar_port,omitempty"`
	CPULimit       string          `json:"cpu_limit"`
	MemoryLimit    string          `json:"memory_limit"`
	Env            json.RawMessage `json:"env,omitempty"`
	TTLSeconds     int             `json:"ttl_seconds"`
	IngressHostTpl *string         `json:"ingress_host_tpl,omitempty"`
}

// ProblemHint 提示。
type ProblemHint struct {
	ID         string    `json:"id"`
	ProblemID  string    `json:"problem_id"`
	Content    string    `json:"content,omitempty"`
	Cost       int       `json:"cost"`
	SortOrder  int       `json:"sort_order"`
	IsVisible  bool      `json:"is_visible"`
	IsUnlocked bool      `json:"is_unlocked"`
	CreatedAt  time.Time `json:"created_at,omitempty"`
}

// PrerequisiteRef 前置题目简要信息。
type PrerequisiteRef struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	IsSolved bool   `json:"is_solved"`
}

// InstanceView 实例状态视图（v3.0 Part 8.1）。
type InstanceView struct {
	Status           string  `json:"status"`
	AccessURL        *string `json:"access_url,omitempty"`
	StartedAt        *string `json:"started_at,omitempty"`
	ExpiresAt        *string `json:"expires_at,omitempty"`
	RemainingSeconds *int    `json:"remaining_seconds,omitempty"`
}

// ConnectionInfo HTTP / TCP 连接信息（v3.0 Part 3.2）。
type ConnectionInfo struct {
	HTTP *string `json:"http,omitempty"`
	TCP  *string `json:"tcp,omitempty"`
}

// BuildTaskView 构建任务视图。
type BuildTaskView struct {
	ID          string  `json:"id"`
	Status      string  `json:"status"`
	CurrentStep *string `json:"current_step,omitempty"`
	ErrorLog    *string `json:"error_log,omitempty"`
	StartedAt   string  `json:"started_at"`
	FinishedAt  *string `json:"finished_at,omitempty"`
}

// SubmissionsSummary 题目提交统计摘要。
type SubmissionsSummary struct {
	Total           int `json:"total"`
	Correct         int `json:"correct"`
	DistinctSolvers int `json:"distinct_solvers"`
}

// ChallengeInstance 实例 DB 行（challenge_instances 表）。
type ChallengeInstance struct {
	ID               string  `json:"id"`
	UserID           string  `json:"user_id"`
	ProblemID        string  `json:"problem_id"`
	PodName          *string `json:"pod_name,omitempty"`
	Namespace        *string `json:"namespace,omitempty"`
	AccessURL        *string `json:"access_url,omitempty"`
	NodePort         *int    `json:"node_port,omitempty"`
	Status           string  `json:"status"`
	FlagValue        *string `json:"-"`
	StartedAt        *string `json:"started_at,omitempty"`
	ExpiredAt        *string `json:"expired_at,omitempty"`
	DestroyedAt      *string `json:"destroyed_at,omitempty"`
	RemainingSeconds *int    `json:"remaining_seconds,omitempty"`
}

// ImageBuildTask 镜像构建任务 DB 行。
type ImageBuildTask struct {
	ID          string  `json:"id"`
	ProblemID   string  `json:"problem_id"`
	FilePath    string  `json:"file_path"`
	FileSize    int64   `json:"file_size"`
	Status      string  `json:"status"`
	CurrentStep string  `json:"current_step"`
	ImageRepo   *string `json:"image_repo,omitempty"`
	ImageTag    *string `json:"image_tag,omitempty"`
	ErrorLog    *string `json:"error_log,omitempty"`
	RetryCount  int     `json:"retry_count"`
	MaxRetry    int     `json:"max_retry"`
	WorkerID    *string `json:"worker_id,omitempty"`
	StartedAt   string  `json:"started_at"`
	FinishedAt  *string `json:"finished_at,omitempty"`
}

// ProblemSubmissionMeta 提交评估所需的题目元数据。
type ProblemSubmissionMeta struct {
	ID            string
	Title         string
	PlatformScore int
	Status        string
	DeployMode    string
	FlagType      string
	FlagRule      string
	FlagHash      string
	FlagPattern   string
}

// ============================================================
// 请求 DTO — handler 入参
// ============================================================

// CreateProblemDraftRequest 创建草稿（仅需分类与类型）。
type CreateProblemDraftRequest struct {
	Category        string  `json:"category"`
	Type            string  `json:"type"`
	SourceProblemID *string `json:"source_problem_id,omitempty"`
}

// UpdateProblemBasicRequest Step1 基本信息保存。
// 注意：不再接受 contest_score —— 那是赛事 scope 的字段，由
// PUT /api/admin/contests/{cid}/challenges/{id} 配置。
type UpdateProblemBasicRequest struct {
	Title         string   `json:"title"`
	Author        string   `json:"author"`
	Category      string   `json:"category"`
	Type          string   `json:"type"`
	Description   string   `json:"description"`
	PlatformScore int      `json:"platform_score"`
	Difficulty    string   `json:"difficulty"`
	DeployMode    string   `json:"deploy_mode"`
	UseKata       bool     `json:"use_kata"`
	Tags          []string `json:"tags"`
}

// SaveProblemFlagRequest Step2 Flag 配置保存。
// flag_value 在 static + (exact|case_insensitive) 时为明文 flag；
// 在 static + (regex|contains) 时为正则/子串模式（写入 flag_pattern）；
// dynamic 时无需提供。
type SaveProblemFlagRequest struct {
	FlagType           string  `json:"flag_type"`
	FlagRule           string  `json:"flag_rule"`
	FlagValue          string  `json:"flag_value"`
	FlagExample        *string `json:"flag_example"`
	FlagInject         string  `json:"flag_inject"`
	FlagEnvKey         string  `json:"flag_env_key"`
	FlagSidecarPattern *string `json:"flag_sidecar_pattern"`
}

// SaveProblemContainerRequest Step4 容器配置保存。sidecar 可选。
type SaveProblemContainerRequest struct {
	ImageName      string          `json:"image_name"`
	ImageTag       string          `json:"image_tag"`
	ChallengePort  int             `json:"challenge_port"`
	SidecarImage   *string         `json:"sidecar_image"`
	SidecarPort    *int            `json:"sidecar_port"`
	CPULimit       string          `json:"cpu_limit"`
	MemoryLimit    string          `json:"memory_limit"`
	Env            json.RawMessage `json:"env"`
	TTLSeconds     int             `json:"ttl_seconds"`
	IngressHostTpl *string         `json:"ingress_host_tpl"`
}

// CreateHintRequest / UpdateHintRequest 提示 CRUD 入参。
type CreateHintRequest struct {
	Content   string `json:"content"`
	Cost      int    `json:"cost"`
	SortOrder int    `json:"sort_order"`
	IsVisible *bool  `json:"is_visible"`
}

type UpdateHintRequest struct {
	Content   string `json:"content"`
	Cost      int    `json:"cost"`
	SortOrder int    `json:"sort_order"`
	IsVisible *bool  `json:"is_visible"`
}

// SetPrerequisitesRequest 批量设置前置题目。
type SetPrerequisitesRequest struct {
	PrerequisiteIDs []string `json:"prerequisite_ids"`
}

// ============================================================
// 难度分值区间（v2.1 Part 1.4 / 3.1）
// ============================================================

// DifficultyScoreRange easy:[1,200] medium:[200,500] hard:[500,1000] expert:[1000,2000]
type DifficultyScoreRange struct {
	Min int
	Max int
}

var difficultyRanges = map[string]DifficultyScoreRange{
	"easy":   {Min: 1, Max: 200},
	"medium": {Min: 200, Max: 500},
	"hard":   {Min: 500, Max: 1000},
	"expert": {Min: 1000, Max: 2000},
}

// GetDifficultyRange 获取难度对应的平台分值区间。
func GetDifficultyRange(difficulty string) (DifficultyScoreRange, bool) {
	r, ok := difficultyRanges[difficulty]
	return r, ok
}

// ============================================================
// connection_info 构造（v3.0 Part 3.2 / 8.1）
// ============================================================
//
// 规则：
//   - type=static                                                     → nil
//   - type=container_web   且 instance.status=running 且 access_url   → {http: access_url}
//   - type=container_tcp   且 instance.status=running                 → {tcp: nc <host> <port>}
//
// container_tcp 优先级：access_url > "host:port"（含 schema 时去掉 schema），
//
//	否则用 instance.node_port + access_url 中的 host。
//	提取失败时返回 nil（避免半残数据）。
func BuildConnectionInfo(problemType string, instance *ChallengeInstance, container *ProblemContainer) *ConnectionInfo {
	if instance == nil || instance.Status != "running" {
		return nil
	}
	switch problemType {
	case "container_web":
		if instance.AccessURL == nil || *instance.AccessURL == "" {
			return nil
		}
		url := *instance.AccessURL
		return &ConnectionInfo{HTTP: &url}
	case "container_tcp":
		host, port := extractTCPHostPort(instance, container)
		if host == "" || port == 0 {
			return nil
		}
		s := tcpConnectionString(host, port)
		return &ConnectionInfo{TCP: &s}
	default:
		return nil
	}
}

// extractTCPHostPort 从 access_url / node_port / container.challenge_port 中提取 host+port。
func extractTCPHostPort(instance *ChallengeInstance, container *ProblemContainer) (string, int) {
	host := ""
	port := 0
	if instance.AccessURL != nil {
		raw := *instance.AccessURL
		raw = stripSchema(raw)
		if h, p, ok := splitHostPort(raw); ok {
			host = h
			if p > 0 {
				port = p
			}
		} else if raw != "" {
			host = raw
		}
	}
	if port == 0 && instance.NodePort != nil && *instance.NodePort > 0 {
		port = *instance.NodePort
	}
	if port == 0 && container != nil && container.ChallengePort > 0 {
		port = container.ChallengePort
	}
	return host, port
}

func stripSchema(raw string) string {
	for _, schema := range []string{"http://", "https://", "tcp://"} {
		if len(raw) >= len(schema) && equalFoldPrefix(raw, schema) {
			return raw[len(schema):]
		}
	}
	return raw
}

func equalFoldPrefix(s, p string) bool {
	if len(s) < len(p) {
		return false
	}
	for i := 0; i < len(p); i++ {
		a, b := s[i], p[i]
		if 'A' <= a && a <= 'Z' {
			a += 'a' - 'A'
		}
		if 'A' <= b && b <= 'Z' {
			b += 'a' - 'A'
		}
		if a != b {
			return false
		}
	}
	return true
}

// splitHostPort 简易解析 host:port，path 部分会被丢弃。
func splitHostPort(raw string) (string, int, bool) {
	cut := -1
	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case '/':
			raw = raw[:i]
			i = len(raw)
		case ':':
			cut = i
		}
	}
	if cut < 0 {
		return raw, 0, false
	}
	host := raw[:cut]
	portStr := raw[cut+1:]
	port := 0
	for i := 0; i < len(portStr); i++ {
		c := portStr[i]
		if c < '0' || c > '9' {
			return host, 0, false
		}
		port = port*10 + int(c-'0')
	}
	return host, port, port > 0
}

func tcpConnectionString(host string, port int) string {
	return "nc " + host + " " + itoa(port)
}

// itoa 避免引入 strconv。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
