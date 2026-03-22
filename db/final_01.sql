-- ==========================================================
-- CTF Platform - Consolidated Initialization Script (Revised)
-- Target: MySQL / MariaDB
-- ==========================================================

CREATE DATABASE IF NOT EXISTS ctf_exec CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE ctf_exec;

SET FOREIGN_KEY_CHECKS = 0;

DROP TABLE IF EXISTS user_activity_logs;
DROP TABLE IF EXISTS contest_leaderboards;
DROP TABLE IF EXISTS leaderboard_global;
DROP TABLE IF EXISTS contest_scores;
DROP TABLE IF EXISTS contest_participants;
DROP TABLE IF EXISTS contest_challenges;
DROP TABLE IF EXISTS contests;
DROP TABLE IF EXISTS docker_images;
DROP TABLE IF EXISTS attachments;
DROP TABLE IF EXISTS challenge_instances;
DROP TABLE IF EXISTS challenge_tags;
DROP TABLE IF EXISTS submissions;
DROP TABLE IF EXISTS problems;
DROP TABLE IF EXISTS team_members;
DROP TABLE IF EXISTS teams;
DROP TABLE IF EXISTS users;

SET FOREIGN_KEY_CHECKS = 1;

-- ----------------------------------------------------------
-- 1. Users Table
-- ----------------------------------------------------------

CREATE TABLE users (
    id CHAR(36) PRIMARY KEY,
    username VARCHAR(32) NOT NULL UNIQUE,
    email VARCHAR(254) UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(16) NOT NULL DEFAULT 'user',

    display_name VARCHAR(64),
    avatar_url VARCHAR(1024),
    bio TEXT,
    website_url VARCHAR(512),
    github_username VARCHAR(100),
    twitter_username VARCHAR(100),

    real_name VARCHAR(100),
    student_id VARCHAR(50),
    organization VARCHAR(100),
    country CHAR(2),

    -- [MOD] 删除 users.team_id
    -- 原设计存在 team_members 表，users.team_id 会导致关系冗余和不一致

    score INT NOT NULL DEFAULT 0,
    solved_count INT NOT NULL DEFAULT 0,
    rank_num INT,

    is_active TINYINT(1) NOT NULL DEFAULT 1,
    is_verified TINYINT(1) NOT NULL DEFAULT 0,

    twofa_enabled TINYINT(1) NOT NULL DEFAULT 0,
    twofa_secret VARCHAR(100),
    twofa_recovery_codes TEXT,

    auth_provider VARCHAR(32) DEFAULT 'local',
    external_id VARCHAR(255),

    password_reset_token VARCHAR(128),
    password_reset_expires DATETIME,

    last_login_at DATETIME,
    last_login_ip VARCHAR(45),
    registration_ip VARCHAR(45),

    preferences JSON DEFAULT (JSON_OBJECT()),

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    CONSTRAINT username_format CHECK (username REGEXP '^[a-zA-Z0-9_]{3,32}$'),
    CONSTRAINT role_valid CHECK (role IN ('root', 'admin', 'user')),
    CONSTRAINT score_non_negative CHECK (score >= 0)
) ENGINE=InnoDB;

CREATE INDEX idx_users_score ON users(score DESC, last_login_at ASC);
CREATE INDEX idx_users_role ON users(role);

-- ----------------------------------------------------------
-- 1.1 Global Leaderboard Snapshot
-- ----------------------------------------------------------

CREATE TABLE leaderboard_global (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    board_type VARCHAR(16) NOT NULL,

    rank_num INT NOT NULL,
    ref_id CHAR(36) NOT NULL,

    username VARCHAR(32),
    display_name VARCHAR(64),
    avatar_url VARCHAR(1024),

    team_name VARCHAR(100),

    score INT NOT NULL DEFAULT 0,
    solved_count INT NOT NULL DEFAULT 0,

    total_score INT NOT NULL DEFAULT 0,
    avg_score DECIMAL(10,2) NOT NULL DEFAULT 0,
    member_count INT NOT NULL DEFAULT 0,

    snapshot_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT chk_board_type CHECK (board_type IN ('user', 'team')),
    UNIQUE KEY uniq_board_rank (board_type, rank_num),
    UNIQUE KEY uniq_board_ref (board_type, ref_id)
) ENGINE=InnoDB;

CREATE INDEX idx_leaderboard_board_rank ON leaderboard_global(board_type, rank_num);
CREATE INDEX idx_leaderboard_board_user ON leaderboard_global(board_type, username);

-- ----------------------------------------------------------
-- 2. Teams
-- ----------------------------------------------------------

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

    role_in_team VARCHAR(16) NOT NULL DEFAULT 'member',

    joined_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY(team_id, user_id),

    FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- ----------------------------------------------------------
-- 3. Problems (Challenges)
-- ----------------------------------------------------------

CREATE TABLE problems (
    id INT PRIMARY KEY AUTO_INCREMENT,

    short_id INT UNIQUE,

    title VARCHAR(255) NOT NULL,
    category VARCHAR(32) NOT NULL,
    type VARCHAR(32) NOT NULL DEFAULT 'static',

    points INT NOT NULL DEFAULT 0,
    difficulty VARCHAR(16) NOT NULL,

    -- [MOD] 删除 tags JSON
    -- 原脚本中同时存在 challenge_tags 表，JSON 会造成冗余和无法索引

    description TEXT,

    -- [MOD] flag -> flag_hash
    -- 避免数据库泄露导致所有题目 flag 暴露
    flag_hash VARCHAR(255) NOT NULL,

    flag_rule VARCHAR(255),
    flag_example VARCHAR(255),

    docker_image VARCHAR(255),
    run_command TEXT,

    visible_from DATETIME,
    visible_to DATETIME,

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB;

CREATE TABLE challenge_tags (
    challenge_id INT NOT NULL,
    tag VARCHAR(64) NOT NULL,

    PRIMARY KEY (challenge_id, tag),

    FOREIGN KEY (challenge_id) REFERENCES problems(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- ----------------------------------------------------------
-- 4. Submissions
-- ----------------------------------------------------------

CREATE TABLE submissions (
    id CHAR(36) PRIMARY KEY,

    user_id CHAR(36),
    team_id CHAR(36),

    problem_id INT NOT NULL,

    -- 赛事上下文（可空：非赛事提交）
    contest_id CHAR(36),

    flag VARCHAR(1024),

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

-- [MOD] 新增复合索引
-- 用于快速判断用户是否解过某题
CREATE INDEX idx_sub_user_problem ON submissions(user_id, problem_id);
-- contest first blood 查询优化
CREATE INDEX idx_sub_problem_correct ON submissions(problem_id, is_correct);
-- contest scoreboard 查询优化
CREATE INDEX idx_sub_user_time ON submissions(user_id, created_at);
-- 赛事提交判重与查询优化
CREATE INDEX idx_sub_contest_user_problem_correct ON submissions(contest_id, user_id, problem_id, is_correct);

-- ----------------------------------------------------------
-- 5. Challenge Instances
-- ----------------------------------------------------------

CREATE TABLE challenge_instances (
    id CHAR(36) PRIMARY KEY,

    challenge_id INT NOT NULL,

    owner_user_id CHAR(36),
    owner_team_id CHAR(36),

    pod_name VARCHAR(255),
    job_id VARCHAR(255),

    status VARCHAR(32) NOT NULL DEFAULT 'running',

    address VARCHAR(255),

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    destroyed_at DATETIME,

    meta JSON,

    FOREIGN KEY (challenge_id) REFERENCES problems(id) ON DELETE CASCADE,
    FOREIGN KEY (owner_user_id) REFERENCES users(id) ON DELETE SET NULL,
    FOREIGN KEY (owner_team_id) REFERENCES teams(id) ON DELETE SET NULL
) ENGINE=InnoDB;

-- ----------------------------------------------------------
-- Attachments
-- ----------------------------------------------------------

CREATE TABLE attachments (
    id CHAR(36) PRIMARY KEY,
    challenge_id INT NOT NULL,

    file_name VARCHAR(255),
    storage_path VARCHAR(1024),
    file_type VARCHAR(50),

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (challenge_id) REFERENCES problems(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- ----------------------------------------------------------
-- Docker Images
-- ----------------------------------------------------------

CREATE TABLE docker_images (
    id CHAR(36) PRIMARY KEY,

    challenge_id INT NOT NULL,

    image_name VARCHAR(255) NOT NULL,
    image_tag VARCHAR(128),

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (challenge_id) REFERENCES problems(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- ----------------------------------------------------------
-- Contests
-- ----------------------------------------------------------

CREATE TABLE contests (
    id CHAR(36) PRIMARY KEY,

    title VARCHAR(255) NOT NULL,
    description TEXT,

    cover_image VARCHAR(1024),

    start_time DATETIME,
    end_time DATETIME,

    scoring_type VARCHAR(32) DEFAULT 'jeopardy',

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB;

-- ----------------------------------------------------------
-- Contest Challenges
-- ----------------------------------------------------------

CREATE TABLE contest_challenges (
    contest_id CHAR(36) NOT NULL,
    challenge_id INT NOT NULL,

    score_override INT,

    PRIMARY KEY (contest_id, challenge_id),

    FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE CASCADE,
    FOREIGN KEY (challenge_id) REFERENCES problems(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- ----------------------------------------------------------
-- Contest Participants
-- ----------------------------------------------------------

CREATE TABLE contest_participants (

    -- [MOD] 新增 id 主键
    id CHAR(36) PRIMARY KEY,

    contest_id CHAR(36) NOT NULL,

    user_id CHAR(36),
    team_id CHAR(36),

    registered_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE CASCADE

) ENGINE=InnoDB;

-- [MOD] 个人参赛唯一约束
CREATE UNIQUE INDEX uniq_contest_user
ON contest_participants(contest_id, user_id);

-- [MOD] 团队参赛唯一约束
CREATE UNIQUE INDEX uniq_contest_team
ON contest_participants(contest_id, team_id);

-- ----------------------------------------------------------
-- Contest Scores
-- ----------------------------------------------------------

CREATE TABLE contest_scores (

    id CHAR(36) PRIMARY KEY,

    contest_id CHAR(36) NOT NULL,

    user_id CHAR(36),
    team_id CHAR(36),

    score INT NOT NULL DEFAULT 0,

    solved_count INT DEFAULT 0,

    last_solve_time DATETIME,

    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP 
        ON UPDATE CURRENT_TIMESTAMP,

    UNIQUE KEY uniq_contest_user (contest_id, user_id),

    UNIQUE KEY uniq_contest_team (contest_id, team_id),

    FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE

) ENGINE=InnoDB;

-- ----------------------------------------------------------
-- Contest Realtime Leaderboard Metadata
-- ----------------------------------------------------------

CREATE TABLE contest_leaderboards (
    contest_id CHAR(36) PRIMARY KEY,

    mode VARCHAR(32) NOT NULL DEFAULT 'redis_realtime',
    redis_user_key VARCHAR(255) NOT NULL,
    redis_team_key VARCHAR(255) NOT NULL,

    is_active TINYINT(1) NOT NULL DEFAULT 1,
    last_synced_at DATETIME,

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    CONSTRAINT chk_contest_lb_mode CHECK (mode IN ('redis_realtime', 'snapshot')),
    FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- ----------------------------------------------------------
-- User Activity Logs
-- ----------------------------------------------------------

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

CREATE INDEX idx_activity_action_time ON user_activity_logs(action_type, created_at);

-- [MOD] 角色变更审计约定（最小方案）
-- action_type: 'role_change'
-- details JSON: {
--   "operator_id": "<root_user_id>",
--   "old_role": "user",
--   "new_role": "admin",
--   "reason": "..."
-- }

-- ----------------------------------------------------------
-- Demo Users
-- ----------------------------------------------------------

INSERT INTO users
(id, username, email, password_hash, role, display_name, bio, real_name, organization, country, is_active, is_verified)
VALUES
('00000000-0000-0000-0000-000000000001','root','root@ctf.local','$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe','root','Root','Default root account','System Root','CTF Platform','CN',1,1),
('11111111-1111-1111-1111-111111111111','alice','alice@example.com','$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe','user','Alice','Demo user Alice','Alice L','University A','CN',1,1),
('22222222-2222-2222-2222-222222222222','bob','bob@example.com','$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe','admin','Bob','Demo admin Bob','Bob K','University B','US',1,1);

-- 补齐测试用户到 20 条（不含 root）
INSERT INTO users
(id, username, email, password_hash, role, display_name, bio, real_name, organization, country, is_active, is_verified)
VALUES
('30000000-0000-0000-0000-000000000003','user03','user03@example.com','$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe','user','User03','Demo user 03','User 03','University C','CN',1,1),
('30000000-0000-0000-0000-000000000004','user04','user04@example.com','$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe','user','User04','Demo user 04','User 04','University C','CN',1,1),
('30000000-0000-0000-0000-000000000005','user05','user05@example.com','$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe','user','User05','Demo user 05','User 05','University D','CN',1,1),
('30000000-0000-0000-0000-000000000006','user06','user06@example.com','$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe','user','User06','Demo user 06','User 06','University D','CN',1,1),
('30000000-0000-0000-0000-000000000007','user07','user07@example.com','$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe','user','User07','Demo user 07','User 07','University E','CN',1,1),
('30000000-0000-0000-0000-000000000008','user08','user08@example.com','$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe','user','User08','Demo user 08','User 08','University E','CN',1,1),
('30000000-0000-0000-0000-000000000009','user09','user09@example.com','$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe','user','User09','Demo user 09','User 09','University F','CN',1,1),
('30000000-0000-0000-0000-000000000010','user10','user10@example.com','$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe','user','User10','Demo user 10','User 10','University F','CN',1,1),
('30000000-0000-0000-0000-000000000011','user11','user11@example.com','$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe','user','User11','Demo user 11','User 11','University G','CN',1,1),
('30000000-0000-0000-0000-000000000012','user12','user12@example.com','$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe','user','User12','Demo user 12','User 12','University G','CN',1,1),
('30000000-0000-0000-0000-000000000013','user13','user13@example.com','$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe','user','User13','Demo user 13','User 13','University H','CN',1,1),
('30000000-0000-0000-0000-000000000014','user14','user14@example.com','$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe','user','User14','Demo user 14','User 14','University H','CN',1,1),
('30000000-0000-0000-0000-000000000015','user15','user15@example.com','$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe','user','User15','Demo user 15','User 15','University I','CN',1,1),
('30000000-0000-0000-0000-000000000016','user16','user16@example.com','$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe','user','User16','Demo user 16','User 16','University I','CN',1,1),
('30000000-0000-0000-0000-000000000017','user17','user17@example.com','$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe','user','User17','Demo user 17','User 17','University J','CN',1,1),
('30000000-0000-0000-0000-000000000018','user18','user18@example.com','$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe','user','User18','Demo user 18','User 18','University J','CN',1,1),
('30000000-0000-0000-0000-000000000019','user19','user19@example.com','$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe','user','User19','Demo user 19','User 19','University K','CN',1,1),
('30000000-0000-0000-0000-000000000020','user20','user20@example.com','$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe','user','User20','Demo user 20','User 20','University K','CN',1,1);

-- ----------------------------------------------------------
-- Demo Teams
-- ----------------------------------------------------------

INSERT INTO teams (id,name,invite_code,leader_id)
VALUES
('5be521ef-1d35-11f1-a099-da2caeb72bcd','Team Phoenix','PHOENIX8','11111111-1111-1111-1111-111111111111'),
('6be521ef-1d35-11f1-a099-da2caeb72bcd','Red Team','REDTEAM1','22222222-2222-2222-2222-222222222222');

-- 补齐测试团队到 20 条
INSERT INTO teams (id,name,invite_code,leader_id)
VALUES
('70000000-0000-0000-0000-000000000003','Team 03','TEAM0003','30000000-0000-0000-0000-000000000003'),
('70000000-0000-0000-0000-000000000004','Team 04','TEAM0004','30000000-0000-0000-0000-000000000004'),
('70000000-0000-0000-0000-000000000005','Team 05','TEAM0005','30000000-0000-0000-0000-000000000005'),
('70000000-0000-0000-0000-000000000006','Team 06','TEAM0006','30000000-0000-0000-0000-000000000006'),
('70000000-0000-0000-0000-000000000007','Team 07','TEAM0007','30000000-0000-0000-0000-000000000007'),
('70000000-0000-0000-0000-000000000008','Team 08','TEAM0008','30000000-0000-0000-0000-000000000008'),
('70000000-0000-0000-0000-000000000009','Team 09','TEAM0009','30000000-0000-0000-0000-000000000009'),
('70000000-0000-0000-0000-000000000010','Team 10','TEAM0010','30000000-0000-0000-0000-000000000010'),
('70000000-0000-0000-0000-000000000011','Team 11','TEAM0011','30000000-0000-0000-0000-000000000011'),
('70000000-0000-0000-0000-000000000012','Team 12','TEAM0012','30000000-0000-0000-0000-000000000012'),
('70000000-0000-0000-0000-000000000013','Team 13','TEAM0013','30000000-0000-0000-0000-000000000013'),
('70000000-0000-0000-0000-000000000014','Team 14','TEAM0014','30000000-0000-0000-0000-000000000014'),
('70000000-0000-0000-0000-000000000015','Team 15','TEAM0015','30000000-0000-0000-0000-000000000015'),
('70000000-0000-0000-0000-000000000016','Team 16','TEAM0016','30000000-0000-0000-0000-000000000016'),
('70000000-0000-0000-0000-000000000017','Team 17','TEAM0017','30000000-0000-0000-0000-000000000017'),
('70000000-0000-0000-0000-000000000018','Team 18','TEAM0018','30000000-0000-0000-0000-000000000018'),
('70000000-0000-0000-0000-000000000019','Team 19','TEAM0019','30000000-0000-0000-0000-000000000019'),
('70000000-0000-0000-0000-000000000020','Team 20','TEAM0020','30000000-0000-0000-0000-000000000020');


INSERT INTO team_members (team_id, user_id, role_in_team, joined_at)
VALUES
('5be521ef-1d35-11f1-a099-da2caeb72bcd','11111111-1111-1111-1111-111111111111','leader',CURRENT_TIMESTAMP),
('6be521ef-1d35-11f1-a099-da2caeb72bcd','22222222-2222-2222-2222-222222222222','leader',CURRENT_TIMESTAMP);

INSERT INTO team_members (team_id, user_id, role_in_team, joined_at)
VALUES
('70000000-0000-0000-0000-000000000003','30000000-0000-0000-0000-000000000003','leader',CURRENT_TIMESTAMP),
('70000000-0000-0000-0000-000000000004','30000000-0000-0000-0000-000000000004','leader',CURRENT_TIMESTAMP),
('70000000-0000-0000-0000-000000000005','30000000-0000-0000-0000-000000000005','leader',CURRENT_TIMESTAMP),
('70000000-0000-0000-0000-000000000006','30000000-0000-0000-0000-000000000006','leader',CURRENT_TIMESTAMP),
('70000000-0000-0000-0000-000000000007','30000000-0000-0000-0000-000000000007','leader',CURRENT_TIMESTAMP),
('70000000-0000-0000-0000-000000000008','30000000-0000-0000-0000-000000000008','leader',CURRENT_TIMESTAMP),
('70000000-0000-0000-0000-000000000009','30000000-0000-0000-0000-000000000009','leader',CURRENT_TIMESTAMP),
('70000000-0000-0000-0000-000000000010','30000000-0000-0000-0000-000000000010','leader',CURRENT_TIMESTAMP),
('70000000-0000-0000-0000-000000000011','30000000-0000-0000-0000-000000000011','leader',CURRENT_TIMESTAMP),
('70000000-0000-0000-0000-000000000012','30000000-0000-0000-0000-000000000012','leader',CURRENT_TIMESTAMP),
('70000000-0000-0000-0000-000000000013','30000000-0000-0000-0000-000000000013','leader',CURRENT_TIMESTAMP),
('70000000-0000-0000-0000-000000000014','30000000-0000-0000-0000-000000000014','leader',CURRENT_TIMESTAMP),
('70000000-0000-0000-0000-000000000015','30000000-0000-0000-0000-000000000015','leader',CURRENT_TIMESTAMP),
('70000000-0000-0000-0000-000000000016','30000000-0000-0000-0000-000000000016','leader',CURRENT_TIMESTAMP),
('70000000-0000-0000-0000-000000000017','30000000-0000-0000-0000-000000000017','leader',CURRENT_TIMESTAMP),
('70000000-0000-0000-0000-000000000018','30000000-0000-0000-0000-000000000018','leader',CURRENT_TIMESTAMP),
('70000000-0000-0000-0000-000000000019','30000000-0000-0000-0000-000000000019','leader',CURRENT_TIMESTAMP),
('70000000-0000-0000-0000-000000000020','30000000-0000-0000-0000-000000000020','leader',CURRENT_TIMESTAMP);

-- ----------------------------------------------------------
-- Demo Problems
-- 所有题目 flag 为 flag{demo}
-- flag_hash 为 flag{demo} 的 SHA256
-- ----------------------------------------------------------

INSERT INTO problems
(id, short_id, title, category, points, difficulty, description, flag_hash)
VALUES
(1, 1, 'Hello Web', 'web', 100, '简单', '一个简单的 Web 入门题', '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4'),
(2, 2, 'babyheap', 'pwn', 300, '困难', '经典 heap 漏洞利用', '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4'),
(3, 3, 'easycrypto', 'crypto', 150, '中等', 'RSA 入门题', '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4'),
(4, 4, 'reverse-me', 'reverse', 200, '中等', '简单逆向分析', '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4'),
(5, 5, 'misc-challenge', 'misc', 120, '简单', '基础 Misc 题', '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4');

-- 补齐测试题目到 20 条
INSERT INTO problems
(id, short_id, title, category, points, difficulty, description, flag_hash)
VALUES
(6, 6, 'web-login-bypass', 'web', 130, '简单', '登录绕过基础题', '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4'),
(7, 7, 'web-template-injection', 'web', 220, '中等', '模板注入利用', '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4'),
(8, 8, 'pwn-ret2libc', 'pwn', 260, '中等', 'ret2libc 基础利用', '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4'),
(9, 9, 'pwn-uaf-training', 'pwn', 340, '困难', 'Use-After-Free 训练题', '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4'),
(10, 10, 'crypto-shift', 'crypto', 110, '简单', '古典密码位移分析', '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4'),
(11, 11, 'crypto-rsa-common-mod', 'crypto', 240, '中等', 'RSA 共模攻击', '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4'),
(12, 12, 'crypto-lcg-break', 'crypto', 320, '困难', '线性同余生成器预测', '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4'),
(13, 13, 'reverse-crackme-1', 'reverse', 140, '简单', 'CrackMe 入门', '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4'),
(14, 14, 'reverse-vm-disasm', 'reverse', 280, '中等', '虚拟机指令逆向', '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4'),
(15, 15, 'reverse-anti-debug', 'reverse', 360, '困难', '反调试绕过分析', '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4'),
(16, 16, 'misc-forensics-disk', 'misc', 160, '中等', '磁盘取证基础题', '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4'),
(17, 17, 'misc-stego-audio', 'misc', 180, '中等', '音频隐写分析', '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4'),
(18, 18, 'misc-pcap-hunt', 'misc', 210, '中等', '流量包线索提取', '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4'),
(19, 19, 'web-race-condition', 'web', 310, '困难', '竞态条件漏洞利用', '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4'),
(20, 20, 'pwn-srop-lab', 'pwn', 400, '困难', 'SROP 技术实验题', '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4');

-- ----------------------------------------------------------
-- Demo Challenge Tags
-- ----------------------------------------------------------

INSERT INTO challenge_tags (challenge_id, tag)
VALUES
-- Hello Web
(1,'web'),
(1,'http'),
(1,'beginner'),

-- babyheap
(2,'pwn'),
(2,'heap'),
(2,'glibc'),

-- easycrypto
(3,'crypto'),
(3,'rsa'),
(3,'number-theory'),

-- reverse-me
(4,'reverse'),
(4,'binary'),
(4,'x86'),

-- misc
(5,'misc'),
(5,'forensics'),
(5,'steganography');

-- ----------------------------------------------------------
-- Demo Contests (20 rows)
-- ----------------------------------------------------------

INSERT INTO contests
(id, title, description, cover_image, start_time, end_time, scoring_type)
VALUES
('90000000-0000-0000-0000-000000000001', '2026 Spring Warmup', 'Spring warmup contest for newcomers and team collaboration.', '/static/covers/contest_01.jpg', '2026-03-01 10:00:00', '2026-03-01 18:00:00', 'jeopardy'),
('90000000-0000-0000-0000-000000000002', 'Weekly Practice #01', 'Weekly practice focused on web and misc basics.', '/static/covers/contest_02.jpg', '2026-03-03 19:30:00', '2026-03-03 22:30:00', 'jeopardy'),
('90000000-0000-0000-0000-000000000003', 'Weekly Practice #02', 'Second weekly practice with crypto and reverse challenges.', '/static/covers/contest_03.jpg', '2026-03-05 19:30:00', '2026-03-05 22:30:00', 'jeopardy'),
('90000000-0000-0000-0000-000000000004', 'Weekend Mini CTF #01', 'Short weekend event with mixed categories.', '/static/covers/contest_04.jpg', '2026-03-07 14:00:00', '2026-03-07 18:00:00', 'jeopardy'),
('90000000-0000-0000-0000-000000000005', 'Reverse Monthly Challenge', 'Monthly reverse engineering challenge set.', '/static/covers/contest_05.jpg', '2026-03-10 09:00:00', '2026-03-31 23:00:00', 'jeopardy'),
('90000000-0000-0000-0000-000000000006', 'Crypto Drill Camp', 'Crypto drill camp with progressive RSA and lattice tasks.', '/static/covers/contest_06.jpg', '2026-03-12 18:00:00', '2026-03-13 23:00:00', 'jeopardy'),
('90000000-0000-0000-0000-000000000007', 'Web Security Sprint', 'Fast-paced web vulnerabilities sprint.', '/static/covers/contest_07.jpg', '2026-03-14 10:00:00', '2026-03-14 20:00:00', 'jeopardy'),
('90000000-0000-0000-0000-000000000008', 'Pwn Intro Battle', 'Introductory pwn contest for stack and heap basics.', '/static/covers/contest_08.jpg', '2026-03-15 09:30:00', '2026-03-15 17:30:00', 'jeopardy'),
('90000000-0000-0000-0000-000000000009', 'Campus Joint Training', 'Inter-university joint training event.', '/static/covers/contest_09.jpg', '2026-03-16 13:00:00', '2026-03-16 21:00:00', 'jeopardy'),
('90000000-0000-0000-0000-000000000010', 'Night Owl CTF #01', 'Night-time short challenge event.', '/static/covers/contest_10.jpg', '2026-03-17 20:00:00', '2026-03-18 00:30:00', 'jeopardy'),
('90000000-0000-0000-0000-000000000011', 'Night Owl CTF #02', 'Second night event with advanced tasks.', '/static/covers/contest_11.jpg', '2026-03-19 20:00:00', '2026-03-20 00:30:00', 'jeopardy'),
('90000000-0000-0000-0000-000000000012', 'University Selection Round A', 'Selection round A for school team candidates.', '/static/covers/contest_12.jpg', '2026-03-22 09:00:00', '2026-03-22 18:00:00', 'jeopardy'),
('90000000-0000-0000-0000-000000000013', 'University Selection Round B', 'Selection round B with stricter scoring.', '/static/covers/contest_13.jpg', '2026-03-24 09:00:00', '2026-03-24 18:00:00', 'jeopardy'),
('90000000-0000-0000-0000-000000000014', 'Attack Surface Workshop CTF', 'Workshop-themed contest focused on service hardening.', '/static/covers/contest_14.jpg', '2026-03-26 14:00:00', '2026-03-26 22:00:00', 'jeopardy'),
('90000000-0000-0000-0000-000000000015', 'Beginner Friendly CTF', 'Friendly event for first-time participants.', '/static/covers/contest_15.jpg', '2026-03-28 10:00:00', '2026-03-28 19:00:00', 'jeopardy'),
('90000000-0000-0000-0000-000000000016', 'April Monthly Open', 'Open monthly contest for all registered users.', '/static/covers/contest_16.jpg', '2026-04-02 09:00:00', '2026-04-02 23:00:00', 'jeopardy'),
('90000000-0000-0000-0000-000000000017', 'Weekend Mini CTF #02', 'Second weekend mini CTF with mixed difficulty.', '/static/covers/contest_17.jpg', '2026-04-05 14:00:00', '2026-04-05 18:00:00', 'jeopardy'),
('90000000-0000-0000-0000-000000000018', 'Blue Team Simulation Day', 'Defense-focused simulation and analysis tasks.', '/static/covers/contest_18.jpg', '2026-04-08 09:00:00', '2026-04-08 17:00:00', 'jeopardy'),
('90000000-0000-0000-0000-000000000019', 'Capture Spring Finals', 'Final spring contest with cumulative ranking.', '/static/covers/contest_19.jpg', '2026-04-12 10:00:00', '2026-04-12 20:00:00', 'jeopardy'),
('90000000-0000-0000-0000-000000000020', 'Preparation Cup 2026', 'Preparation cup for upcoming provincial CTF.', '/static/covers/contest_20.jpg', '2026-04-15 10:00:00', '2026-04-15 18:00:00', 'jeopardy');