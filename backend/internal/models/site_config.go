package models

// SiteConfig 站点全局配置（root 可管理）。
type SiteConfig struct {
	SiteName                  string `json:"site_name"`
	SiteNotice                string `json:"site_notice"`
	RegistrationEnabled       bool   `json:"registration_enabled"`
	EmailVerificationRequired bool   `json:"email_verification_required"`
	DefaultContestVisibility  string `json:"default_contest_visibility"`
}

// UpdateSiteConfigRequest 更新站点配置请求。
type UpdateSiteConfigRequest struct {
	SiteName                  string `json:"site_name"`
	SiteNotice                string `json:"site_notice"`
	RegistrationEnabled       bool   `json:"registration_enabled"`
	EmailVerificationRequired bool   `json:"email_verification_required"`
	DefaultContestVisibility  string `json:"default_contest_visibility"`
}
