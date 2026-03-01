-- Activity / audit logs
CREATE TABLE user_activity_logs (
    id CHAR(36) PRIMARY KEY,
    user_id CHAR(36),
    action_type VARCHAR(64) NOT NULL,
    target_id CHAR(36),
    details JSON,
    ip_address VARCHAR(45),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
) ENGINE=InnoDB;

CREATE INDEX idx_activity_user ON user_activity_logs(user_id);
