package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/ctf_exec/backend/internal/models"
	"github.com/google/uuid"
)

type SiteConfigRepository struct {
	db *sql.DB
}

func NewSiteConfigRepository(db *sql.DB) *SiteConfigRepository {
	return &SiteConfigRepository{db: db}
}

func (r *SiteConfigRepository) ensureTable() error {
	query := `
		CREATE TABLE IF NOT EXISTS site_settings (
			config_key VARCHAR(64) PRIMARY KEY,
			config_value TEXT NOT NULL,
			updated_by CHAR(36),
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			FOREIGN KEY (updated_by) REFERENCES users(id) ON DELETE SET NULL
		) ENGINE=InnoDB;
	`
	if _, err := r.db.Exec(query); err != nil {
		return fmt.Errorf("初始化站点配置表失败: %w", err)
	}

	return nil
}

func parseBool(raw string, defaultValue bool) bool {
	if raw == "" {
		return defaultValue
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return defaultValue
	}
	return parsed
}

// GetSiteConfig 查询站点配置。
func (r *SiteConfigRepository) GetSiteConfig() (*models.SiteConfig, error) {
	if err := r.ensureTable(); err != nil {
		return nil, err
	}

	cfg := &models.SiteConfig{
		SiteName:                  "CTF Platform",
		SiteNotice:                "",
		RegistrationEnabled:       true,
		EmailVerificationRequired: true,
		DefaultContestVisibility:  "public",
	}

	rows, err := r.db.Query(`SELECT config_key, config_value FROM site_settings`)
	if err != nil {
		return nil, fmt.Errorf("查询站点配置失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var key, value string
		if scanErr := rows.Scan(&key, &value); scanErr != nil {
			return nil, fmt.Errorf("读取站点配置失败: %w", scanErr)
		}

		switch key {
		case "site_name":
			cfg.SiteName = value
		case "site_notice":
			cfg.SiteNotice = value
		case "registration_enabled":
			cfg.RegistrationEnabled = parseBool(value, true)
		case "email_verification_required":
			cfg.EmailVerificationRequired = parseBool(value, true)
		case "default_contest_visibility":
			cfg.DefaultContestVisibility = value
		}
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历站点配置失败: %w", err)
	}

	return cfg, nil
}

// UpdateSiteConfig 更新站点配置并写入审计日志。
func (r *SiteConfigRepository) UpdateSiteConfig(operatorID string, req *models.UpdateSiteConfigRequest, ipAddress string) error {
	if err := r.ensureTable(); err != nil {
		return err
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	upsert := `
		INSERT INTO site_settings (config_key, config_value, updated_by, updated_at)
		VALUES (?, ?, ?, NOW())
		ON DUPLICATE KEY UPDATE config_value = VALUES(config_value), updated_by = VALUES(updated_by), updated_at = NOW()
	`

	entries := map[string]string{
		"site_name":                   req.SiteName,
		"site_notice":                 req.SiteNotice,
		"registration_enabled":        strconv.FormatBool(req.RegistrationEnabled),
		"email_verification_required": strconv.FormatBool(req.EmailVerificationRequired),
		"default_contest_visibility":  req.DefaultContestVisibility,
	}

	for key, value := range entries {
		if _, execErr := tx.Exec(upsert, key, value, operatorID); execErr != nil {
			err = fmt.Errorf("更新站点配置失败: %w", execErr)
			return err
		}
	}

	details, marshalErr := json.Marshal(entries)
	if marshalErr != nil {
		err = fmt.Errorf("序列化站点配置审计详情失败: %w", marshalErr)
		return err
	}

	insertLog := `
		INSERT INTO user_activity_logs (id, user_id, action_type, target_id, details, ip_address, created_at)
		VALUES (?, ?, 'site_config_update', NULL, ?, ?, ?)
	`
	if _, execErr := tx.Exec(insertLog, uuid.New().String(), operatorID, string(details), ipAddress, time.Now()); execErr != nil {
		err = fmt.Errorf("写入站点配置审计日志失败: %w", execErr)
		return err
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return fmt.Errorf("提交事务失败: %w", commitErr)
	}

	return nil
}
