-- Dynamic challenge instances (k3s / container instances)
CREATE TABLE challenge_instances (
    id CHAR(36) PRIMARY KEY,
    problem_id INT NOT NULL,
    owner_user_id CHAR(36),
    owner_team_id CHAR(36),
    pod_name VARCHAR(255),
    job_id VARCHAR(255),
    status VARCHAR(32) NOT NULL DEFAULT 'running', -- running / finished / expired
    address VARCHAR(255), -- ip:port or host
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    destroyed_at DATETIME,
    meta JSON,
    FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE CASCADE,
    FOREIGN KEY (owner_user_id) REFERENCES users(id) ON DELETE SET NULL,
    FOREIGN KEY (owner_team_id) REFERENCES teams(id) ON DELETE SET NULL
) ENGINE=InnoDB;

CREATE INDEX idx_instances_problem ON challenge_instances(problem_id);

-- No demo instances by default
