-- ==========================================================
-- CTF Platform - Consolidated Initialization Script
-- Target: MySQL / MariaDB
-- ==========================================================

CREATE DATABASE IF NOT EXISTS ctf_exec CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE ctf_exec;

-- Disable foreign key checks to allow dropping tables in any order
SET FOREIGN_KEY_CHECKS = 0;
DROP TABLE IF EXISTS user_activity_logs;
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
    role VARCHAR(16) NOT NULL DEFAULT 'user', -- user, admin, staff, vip
    
    -- Display & Social
    display_name VARCHAR(64),
    avatar_url VARCHAR(1024),
    bio TEXT,
    website_url VARCHAR(512),
    github_username VARCHAR(100),
    twitter_username VARCHAR(100),
    
    -- Personal/Internal Info
    real_name VARCHAR(100),
    student_id VARCHAR(50),
    organization VARCHAR(100),
    country CHAR(2), -- ISO 3166-1 alpha-2
    
    -- Team Context
    team_id CHAR(36),
    
    -- Game Stats
    score INT NOT NULL DEFAULT 0,
    solved_count INT NOT NULL DEFAULT 0,
    rank_num INT,
    
    -- Security & Auth
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
    
    -- System
    preferences JSON,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    CONSTRAINT username_format CHECK (username REGEXP '^[a-zA-Z0-9_]{3,32}$'),
    CONSTRAINT score_non_negative CHECK (score >= 0)
) ENGINE=InnoDB;

CREATE INDEX idx_users_team_id ON users(team_id);
CREATE INDEX idx_users_score ON users(score DESC, last_login_at ASC);

-- ----------------------------------------------------------
-- 2. Teams Table
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
-- 3. Problems (Challenges) Table
-- ----------------------------------------------------------
CREATE TABLE problems (
    id INT PRIMARY KEY AUTO_INCREMENT,
    short_id INT UNIQUE, -- Legacy compatibility
    title VARCHAR(255) NOT NULL,
    category VARCHAR(32) NOT NULL,
    type VARCHAR(32) NOT NULL DEFAULT 'static', -- static / dynamic / containerized
    points INT NOT NULL DEFAULT 0,
    difficulty VARCHAR(16) NOT NULL,
    tags JSON,
    description TEXT,
    flag VARCHAR(255) NOT NULL, -- Used as the answer
    flag_rule VARCHAR(255), -- metadata: 'fixed', 'regex', etc.
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
-- 4. Submissions Table
-- ----------------------------------------------------------
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

-- ----------------------------------------------------------
-- 5. Additional Service Tables
-- ----------------------------------------------------------

-- Challenge Instances (Dynamic Pods)
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

-- Attachments
CREATE TABLE attachments (
    id CHAR(36) PRIMARY KEY,
    challenge_id INT NOT NULL,
    file_name VARCHAR(255),
    storage_path VARCHAR(1024),
    file_type VARCHAR(50),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (challenge_id) REFERENCES problems(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- Docker Images
CREATE TABLE docker_images (
    id CHAR(36) PRIMARY KEY,
    challenge_id INT NOT NULL,
    image_name VARCHAR(255) NOT NULL,
    image_tag VARCHAR(128),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (challenge_id) REFERENCES problems(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- Contests
CREATE TABLE contests (
    id CHAR(36) PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    cover_image VARCHAR(1024), -- Contest cover image path
    start_time DATETIME,
    end_time DATETIME,
    scoring_type VARCHAR(32) DEFAULT 'jeopardy',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB;

CREATE TABLE contest_challenges (
    contest_id CHAR(36) NOT NULL,
    challenge_id INT NOT NULL,
    score_override INT,
    PRIMARY KEY (contest_id, challenge_id),
    FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE CASCADE,
    FOREIGN KEY (challenge_id) REFERENCES problems(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE TABLE contest_participants (
    contest_id CHAR(36) NOT NULL,
    user_id CHAR(36),
    team_id CHAR(36),
    registered_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (contest_id, user_id, team_id),
    FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- Activity Logs
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

-- ----------------------------------------------------------
-- 6. Demo Data
-- ----------------------------------------------------------

-- Demo Users (Password: 123456)
INSERT INTO users (id, username, email, password_hash, role, display_name, bio, real_name, organization, country, is_active, is_verified)
VALUES
  ('11111111-1111-1111-1111-111111111111', 'alice', 'alice@example.com', '$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe', 'user', 'Alice', 'Demo user Alice', 'Alice L', 'University A', 'CN', 1, 1),
  ('22222222-2222-2222-2222-222222222222', 'bob', 'bob@example.com', '$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe', 'admin', 'Bob', 'Demo admin Bob', 'Bob K', 'University B', 'US', 1, 1);

-- Demo Teams
INSERT INTO teams (id, name, invite_code, score, solved_count) 
VALUES (UUID(), 'Team Phoenix', 'PHOENIX8', 0, 0), (UUID(), 'Red Team', 'REDTEAM1', 0, 0);

-- Demo Problems
INSERT INTO problems (id, short_id, title, category, points, difficulty, tags, description, flag) 
VALUES
(1, 1, 'Hello Web', 'web', 100, '简单', '["http", "xss"]', '一个简单的 Web 题，目标是找到隐藏在页面中的 flag。', 'FLAG{demo}'),
(2, 2, 'babyheap', 'pwn', 300, '困难', '["heap", "exploit"]', 'pwn 题，练习 heap 漏洞利用。', 'FLAG{pwn_demo}'),
(3, 3, 'easycrypto', 'crypto', 150, '中等', '["rsa", "crypto"]', '基础的密码学题目，包含简短的 RSA 破解思路。', 'FLAG{crypto_demo}'),
(4, 4, 'reverse-me', 'reverse', 200, '中等', '["binary", "reverse"]', '反汇编与逆向题，需要分析二进制行为。', 'FLAG{rev_demo}'),
(5, 5, 'misc-challenge', 'misc', 120, '简单', '["logic"]', '杂项题目，考察逻辑与观察能力。', 'FLAG{misc_demo}');

-- Demo Submissions (Alice solved Hello Web)
INSERT INTO submissions (id, user_id, problem_id, flag, is_correct, score_awarded)
SELECT UUID(), u.id, p.id, p.flag, 1, p.points 
FROM users u, problems p 
WHERE u.username = 'alice' AND p.id = 1;

-- Update Alice's score
UPDATE users SET score = 100, solved_count = 1 WHERE username = 'alice';
