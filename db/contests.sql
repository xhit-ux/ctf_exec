-- Contests / competitions
CREATE TABLE contests (
    id CHAR(36) PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    start_time DATETIME,
    end_time DATETIME,
    scoring_type VARCHAR(32) DEFAULT 'jeopardy', -- jeopardy / attack-defense / dynamic
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB;

CREATE TABLE contest_problems (
    contest_id CHAR(36) NOT NULL,
    problem_id INT NOT NULL,
    score_override INT,
    PRIMARY KEY (contest_id, problem_id),
    FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE CASCADE,
    FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE contest_participants (
    contest_id CHAR(36) NOT NULL,
    user_id CHAR(36),
    team_id CHAR(36),
    registered_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (contest_id, user_id, team_id),
    FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL,
    FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE SET NULL
) ENGINE=InnoDB;

-- Demo contest
INSERT INTO contests (id, title, description, start_time, end_time)
VALUES (UUID(), 'Demo Contest', 'A short demo contest for local development', NOW(), DATE_ADD(NOW(), INTERVAL 2 HOUR));
