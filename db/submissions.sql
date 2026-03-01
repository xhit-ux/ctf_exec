-- Submissions: records of flag submissions
CREATE TABLE submissions (
    id CHAR(36) PRIMARY KEY,
    user_id CHAR(36),
    team_id CHAR(36),
    problem_id INT NOT NULL,
    flag VARCHAR(1024), -- The user's input
    is_correct TINYINT(1) NOT NULL DEFAULT 0,
    score_awarded INT NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ip_address VARCHAR(45),
    run_instance_id CHAR(36),
    meta JSON,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL,
    FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE SET NULL,
    FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE INDEX idx_submissions_user ON submissions(user_id);
CREATE INDEX idx_submissions_problem ON submissions(problem_id);

-- Demo submissions (Reference to demo users and problems)
INSERT INTO submissions (id, user_id, problem_id, flag, is_correct, score_awarded)
VALUES 
(UUID(), 'uuid-alice', 1, 'flag{demo_1}', 1, 100),
(UUID(), 'uuid-bob', 2, 'wrong_flag', 0, 0);
