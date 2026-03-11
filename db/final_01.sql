-- ==========================================================
-- CTF Platform - Consolidated Initialization Script (Revised)
-- Target: MySQL / MariaDB
-- ==========================================================

CREATE DATABASE IF NOT EXISTS ctf_exec CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE ctf_exec;

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
    CONSTRAINT score_non_negative CHECK (score >= 0)
) ENGINE=InnoDB;

CREATE INDEX idx_users_score ON users(score DESC, last_login_at ASC);

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

-- ----------------------------------------------------------
-- Demo Users
-- ----------------------------------------------------------

INSERT INTO users
(id, username, email, password_hash, role, display_name, bio, real_name, organization, country, is_active, is_verified)
VALUES
('11111111-1111-1111-1111-111111111111','alice','alice@example.com','$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe','user','Alice','Demo user Alice','Alice L','University A','CN',1,1),
('22222222-2222-2222-2222-222222222222','bob','bob@example.com','$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe','admin','Bob','Demo admin Bob','Bob K','University B','US',1,1);

-- ----------------------------------------------------------
-- Demo Teams
-- ----------------------------------------------------------

INSERT INTO teams (id,name,invite_code,leader_id)
VALUES
('5be521ef-1d35-11f1-a099-da2caeb72bcd','Team Phoenix','PHOENIX8','11111111-1111-1111-1111-111111111111'),
('6be521ef-1d35-11f1-a099-da2caeb72bcd','Red Team','REDTEAM1','22222222-2222-2222-2222-222222222222');


INSERT INTO team_members (team_id, user_id, role_in_team, joined_at)
VALUES
('5be521ef-1d35-11f1-a099-da2caeb72bcd','11111111-1111-1111-1111-111111111111','leader',CURRENT_TIMESTAMP),
('6be521ef-1d35-11f1-a099-da2caeb72bcd','22222222-2222-2222-2222-222222222222','leader',CURRENT_TIMESTAMP);

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