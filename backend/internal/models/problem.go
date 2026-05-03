package models

import "encoding/json"

type Problem struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Category    string            `json:"category"`
	Type        string            `json:"type"`
	Description *string           `json:"description,omitempty"`
	BaseScore   int               `json:"base_score"`
	Difficulty  string            `json:"difficulty"`
	DeployMode  string            `json:"deploy_mode"`
	UseKata     bool              `json:"use_kata"`
	Status      string            `json:"status"`
	CreatedBy   *string           `json:"created_by,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Attachments []*Attachment     `json:"attachments,omitempty"`
	IsSolved    bool              `json:"is_solved,omitempty"`
	Flag        *ProblemFlag      `json:"flag,omitempty"`
	Container   *ProblemContainer `json:"container,omitempty"`
}

type ProblemFlag struct {
	ProblemID          string  `json:"problem_id"`
	FlagRule           string  `json:"flag_rule"`
	FlagHash           *string `json:"-"`
	FlagExample        *string `json:"flag_example,omitempty"`
	FlagType           string  `json:"flag_type"`
	FlagInject         string  `json:"flag_inject"`
	FlagEnvKey         string  `json:"flag_env_key"`
	FlagSidecarPattern *string `json:"flag_sidecar_pattern,omitempty"`
}

type ProblemContainer struct {
	ProblemID      string          `json:"problem_id"`
	ImageName      string          `json:"image_name"`
	ImageTag       string          `json:"image_tag"`
	ChallengePort  int             `json:"challenge_port"`
	SidecarImage   string          `json:"sidecar_image"`
	SidecarPort    int             `json:"sidecar_port"`
	CPULimit       string          `json:"cpu_limit"`
	MemoryLimit    string          `json:"memory_limit"`
	Env            json.RawMessage `json:"env,omitempty"`
	TTLSeconds     int             `json:"ttl_seconds"`
	IngressHostTpl *string         `json:"ingress_host_tpl,omitempty"`
}

type CreateProblemRequest struct {
	Title       string                  `json:"title"`
	Category    string                  `json:"category"`
	Type        string                  `json:"type"`
	Description string                  `json:"description"`
	BaseScore   int                     `json:"base_score"`
	Difficulty  string                  `json:"difficulty"`
	DeployMode  string                  `json:"deploy_mode"`
	UseKata     bool                    `json:"use_kata"`
	Status      string                  `json:"status"`
	Tags        []string                `json:"tags"`
	Flag        CreateFlagRequest       `json:"flag"`
	Container   *CreateContainerRequest `json:"container,omitempty"`
	CreatedBy   string                  `json:"-"`
}

type UpdateProblemRequest struct {
	Title       string                  `json:"title"`
	Category    string                  `json:"category"`
	Type        string                  `json:"type"`
	Description string                  `json:"description"`
	BaseScore   int                     `json:"base_score"`
	Difficulty  string                  `json:"difficulty"`
	DeployMode  string                  `json:"deploy_mode"`
	UseKata     bool                    `json:"use_kata"`
	Status      string                  `json:"status"`
	Tags        []string                `json:"tags"`
	Flag        CreateFlagRequest       `json:"flag"`
	Container   *CreateContainerRequest `json:"container,omitempty"`
}

type CreateFlagRequest struct {
	FlagPlain          string  `json:"flag_plain"`
	FlagRule           string  `json:"flag_rule"`
	FlagExample        *string `json:"flag_example"`
	FlagType           string  `json:"flag_type"`
	FlagInject         string  `json:"flag_inject"`
	FlagEnvKey         string  `json:"flag_env_key"`
	FlagSidecarPattern *string `json:"flag_sidecar_pattern"`
}

type CreateContainerRequest struct {
	ImageName      string          `json:"image_name"`
	ImageTag       string          `json:"image_tag"`
	ChallengePort  int             `json:"challenge_port"`
	SidecarImage   string          `json:"sidecar_image"`
	SidecarPort    int             `json:"sidecar_port"`
	CPULimit       string          `json:"cpu_limit"`
	MemoryLimit    string          `json:"memory_limit"`
	Env            json.RawMessage `json:"env"`
	TTLSeconds     int             `json:"ttl_seconds"`
	IngressHostTpl *string         `json:"ingress_host_tpl"`
}

type CreateProblemDraftRequest struct {
	Category string `json:"category"`
	Type     string `json:"type"`
}

type UpdateProblemBasicRequest struct {
	Title       string `json:"title"`
	Category    string `json:"category"`
	Type        string `json:"type"`
	Description string `json:"description"`
	BaseScore   int    `json:"base_score"`
	Difficulty  string `json:"difficulty"`
	DeployMode  string `json:"deploy_mode"`
	UseKata     bool   `json:"use_kata"`
}

type ProblemSubmissionMeta struct {
	ID       string
	Title    string
	Points   int
	Status   string
	FlagHash string
	FlagRule string
	FlagType string
}

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

type ChallengeInstance struct {
	ID          string  `json:"id"`
	UserID      string  `json:"user_id"`
	ProblemID   string  `json:"problem_id"`
	PodName     *string `json:"pod_name,omitempty"`
	Namespace   *string `json:"namespace,omitempty"`
	AccessURL   *string `json:"access_url,omitempty"`
	NodePort    *int    `json:"node_port,omitempty"`
	Status      string  `json:"status"`
	StartedAt   *string `json:"started_at,omitempty"`
	ExpiredAt   *string `json:"expired_at,omitempty"`
	DestroyedAt *string `json:"destroyed_at,omitempty"`
}
