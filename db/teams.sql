-- Teams and membership
CREATE TABLE teams (
    id CHAR(36) PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    invite_code VARCHAR(32) NOT NULL UNIQUE,
    leader_id CHAR(36),
    score INT NOT NULL DEFAULT 0,
    solved_count INT NOT NULL DEFAULT 0,
    rank_cache INT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (leader_id) REFERENCES users(id) ON DELETE SET NULL
) ENGINE=InnoDB;

CREATE TABLE team_members (
    team_id CHAR(36) NOT NULL,
    user_id CHAR(36) NOT NULL,
    role_in_team VARCHAR(16) NOT NULL DEFAULT 'member', -- leader/member
    joined_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY(team_id, user_id),
    FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- Demo teams
INSERT INTO teams (id, name, invite_code, leader_id, score, solved_count)
VALUES 
('team1-uuid', 'Team Phoenix', 'PHOENIX8', NULL, 0, 0),
('team2-uuid', 'Red Team', 'REDTEAM1', NULL, 0, 0);
