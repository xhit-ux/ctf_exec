-- Users Table Design for Production CTF Platform
-- MySQL Version

CREATE DATABASE IF NOT EXISTS ctf_exec;
USE ctf_exec;

DROP TABLE IF EXISTS submissions;
DROP TABLE IF EXISTS problems;
DROP TABLE IF EXISTS users;

CREATE TABLE IF NOT EXISTS users (
  -- Core Identity
  id char(36) PRIMARY KEY,
  username varchar(32) NOT NULL UNIQUE,
  email varchar(254) UNIQUE, -- Unique but nullable if we allow anonymous/username-only accounts initially
  password_hash varchar(255) NOT NULL, -- Increased length for future-proof hashing algorithms
  role varchar(16) NOT NULL DEFAULT 'user', -- user, admin, staff, vip
  
  -- Display & Social
  display_name varchar(64), -- Can be different from username
  avatar_url varchar(1024), -- URL to avatar image
  bio text, -- Markdown supported biography
  website_url varchar(512),
  github_username varchar(100),
  twitter_username varchar(100),
  
  -- Academic / Organization Info (Common in CTF)
  real_name varchar(100), -- For certificate generation or internal verification
  student_id varchar(50), -- If applicable
  organization varchar(100), -- University or Company name
  country varchar(2), -- ISO 3166-1 alpha-2 country code (e.g., CN, US)

  -- Team Context
  team_id char(36), -- Foreign key to teams table (to be created later)
  
  -- Game Stats (Denormalized for performance, but requires careful updates)
  score integer NOT NULL DEFAULT 0,
  solved_count integer NOT NULL DEFAULT 0,
  rank_num integer, -- 'rank' is a keyword in MySQL 8.0+
  
  -- Security & Auth
  is_active boolean NOT NULL DEFAULT true, -- Soft delete / Ban status
  is_verified boolean NOT NULL DEFAULT false, -- Email verification status
  twofa_enabled boolean NOT NULL DEFAULT false,
  twofa_secret varchar(100), -- TOTP secret key
  twofa_recovery_codes text, -- JSON array
  
  auth_provider varchar(32) DEFAULT 'local', -- local, google, github, etc.
  external_id varchar(255), -- ID in the external provider
  
  password_reset_token varchar(128),
  password_reset_expires timestamp NULL DEFAULT NULL,
  
  last_login_at timestamp NULL DEFAULT NULL,
  last_login_ip varchar(45),
  registration_ip varchar(45),
  
  -- System
  preferences json DEFAULT NULL, -- UI Theme, Notification settings, etc.
  created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  
  -- Constraints
  CONSTRAINT username_format CHECK (username REGEXP '^[a-zA-Z0-9_]{3,32}$'),
  CONSTRAINT score_non_negative CHECK (score >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS problems (
    id integer PRIMARY KEY AUTO_INCREMENT,
    title varchar(255) NOT NULL,
    category varchar(32) NOT NULL,
    points integer NOT NULL DEFAULT 0,
    difficulty varchar(16) NOT NULL, -- 简单, 中等, 困难
    tags json,
    description text,
    flag varchar(255) NOT NULL,
    created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS submissions (
    id char(36) PRIMARY KEY,
    user_id char(36) NOT NULL,
    problem_id integer NOT NULL,
    flag varchar(255) NOT NULL,
    is_correct boolean NOT NULL DEFAULT false,
    created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE INDEX idx_submissions_user_id ON submissions(user_id);
CREATE INDEX idx_submissions_problem_id ON submissions(problem_id);


-- Indices for Performance
CREATE INDEX idx_users_team_id ON users(team_id);
CREATE INDEX idx_users_score_desc ON users(score DESC, last_login_at ASC);
CREATE INDEX idx_users_email_lower ON users(email); -- Note: MySQL case insensitivity depends on collation, strict lower() index might need functional index in MySQL 8
CREATE INDEX idx_users_created_at ON users(created_at);

-- ---------------------------------------------------------------------------
-- Demo data: insert a couple of users for local development
-- Password is '123456' ($2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe)
-- ---------------------------------------------------------------------------
INSERT INTO users (id, username, email, password_hash, role, display_name, avatar_url, bio, real_name, organization, country, is_active, is_verified)
VALUES
  ('11111111-1111-1111-1111-111111111111', 'alice', 'alice@example.com', '$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe', 'user', 'Alice', NULL, 'Demo user Alice', 'Alice L', 'University A', 'CN', 1, 1),
  ('22222222-2222-2222-2222-222222222222', 'bob', 'bob@example.com', '$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe', 'admin', 'Bob', NULL, 'Demo admin Bob', 'Bob K', 'University B', 'US', 1, 1)
ON DUPLICATE KEY UPDATE password_hash=VALUES(password_hash);

INSERT INTO problems (id, title, category, points, difficulty, tags, description, flag) VALUES
(1, 'Hello Web', 'web', 100, '简单', '["http", "xss"]', '一个简单的 Web 题，目标是找到隐藏在页面中的 flag。', 'FLAG{demo}'),
(2, 'babyheap', 'pwn', 300, '困难', '["heap", "exploit"]', 'pwn 题，练习 heap 漏洞利用。', 'FLAG{pwn_demo}'),
(3, 'easycrypto', 'crypto', 150, '中等', '["rsa", "crypto"]', '基础的密码学题目，包含简短的 RSA 破解思路。', 'FLAG{crypto_demo}'),
(4, 'reverse-me', 'reverse', 200, '中等', '["binary", "reverse"]', '反汇编与逆向题，需要分析二进制行为。', 'FLAG{rev_demo}'),
(5, 'misc-challenge', 'misc', 120, '简单', '["logic"]', '杂项题目，考察逻辑与观察能力。', 'FLAG{misc_demo}');


