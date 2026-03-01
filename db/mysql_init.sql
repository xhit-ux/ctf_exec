-- Clean MySQL-compatible initialization for CTF demo
-- Creates database `ctf_exec` and demo tables/data

CREATE DATABASE IF NOT EXISTS ctf_exec CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE ctf_exec;

-- Users
CREATE TABLE IF NOT EXISTS users (
  id CHAR(36) PRIMARY KEY,
  username VARCHAR(32) NOT NULL UNIQUE,
  email VARCHAR(254),
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
  team_id CHAR(36),
  score INT NOT NULL DEFAULT 0,
  solved_count INT NOT NULL DEFAULT 0,
  rank_cache INT,
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
  preferences JSON NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_team_id ON users(team_id);
CREATE INDEX idx_users_score ON users(score);

-- Teams
CREATE TABLE IF NOT EXISTS teams (
  id CHAR(36) PRIMARY KEY,
  name VARCHAR(100) NOT NULL UNIQUE,
  invite_code VARCHAR(32) NOT NULL UNIQUE,
  leader_id CHAR(36),
  score INT NOT NULL DEFAULT 0,
  solved_count INT NOT NULL DEFAULT 0,
  rank_cache INT,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS team_members (
  team_id CHAR(36) NOT NULL,
  user_id CHAR(36) NOT NULL,
  role_in_team VARCHAR(16) NOT NULL DEFAULT 'member',
  joined_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY(team_id, user_id)
);

CREATE INDEX idx_teams_leader ON teams(leader_id);

-- Challenges
CREATE TABLE IF NOT EXISTS challenges (
  id CHAR(36) PRIMARY KEY,
  short_id INT UNIQUE,
  title VARCHAR(255) NOT NULL,
  category VARCHAR(32) NOT NULL,
  type VARCHAR(32) NOT NULL DEFAULT 'static',
  points INT NOT NULL DEFAULT 0,
  difficulty VARCHAR(16),
  description TEXT,
  flag_rule VARCHAR(255),
  flag_example VARCHAR(255),
  docker_image VARCHAR(255),
  run_command TEXT,
  visible_from DATETIME,
  visible_to DATETIME,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS challenge_tags (
  challenge_id CHAR(36) NOT NULL,
  tag VARCHAR(64) NOT NULL,
  PRIMARY KEY (challenge_id, tag),
  FOREIGN KEY (challenge_id) REFERENCES challenges(id) ON DELETE CASCADE
);

-- Submissions
CREATE TABLE IF NOT EXISTS submissions (
  id CHAR(36) PRIMARY KEY,
  user_id CHAR(36),
  team_id CHAR(36),
  challenge_id CHAR(36) NOT NULL,
  submitted_flag VARCHAR(1024),
  is_correct TINYINT(1) NOT NULL DEFAULT 0,
  score_awarded INT NOT NULL DEFAULT 0,
  submitted_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  ip_address VARCHAR(45),
  run_instance_id CHAR(36),
  meta JSON NULL,
  FOREIGN KEY (challenge_id) REFERENCES challenges(id) ON DELETE CASCADE
);

CREATE INDEX idx_submissions_user ON submissions(user_id);
CREATE INDEX idx_submissions_challenge ON submissions(challenge_id);

-- Contests
CREATE TABLE IF NOT EXISTS contests (
  id CHAR(36) PRIMARY KEY,
  title VARCHAR(255) NOT NULL,
  description TEXT,
  start_time DATETIME,
  end_time DATETIME,
  scoring_type VARCHAR(32) DEFAULT 'jeopardy',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS contest_challenges (
  contest_id CHAR(36) NOT NULL,
  challenge_id CHAR(36) NOT NULL,
  score_override INT,
  PRIMARY KEY (contest_id, challenge_id),
  FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE CASCADE,
  FOREIGN KEY (challenge_id) REFERENCES challenges(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS contest_participants (
  contest_id CHAR(36) NOT NULL,
  user_id CHAR(36),
  team_id CHAR(36),
  registered_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (contest_id, user_id, team_id)
);

-- Challenge instances
CREATE TABLE IF NOT EXISTS challenge_instances (
  id CHAR(36) PRIMARY KEY,
  challenge_id CHAR(36) NOT NULL,
  owner_user_id CHAR(36),
  owner_team_id CHAR(36),
  pod_name VARCHAR(255),
  job_id VARCHAR(255),
  status VARCHAR(32) NOT NULL DEFAULT 'running',
  address VARCHAR(255),
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  destroyed_at DATETIME,
  meta JSON NULL,
  FOREIGN KEY (challenge_id) REFERENCES challenges(id) ON DELETE CASCADE
);

CREATE INDEX idx_instances_challenge ON challenge_instances(challenge_id);

-- Activity logs
CREATE TABLE IF NOT EXISTS user_activity_logs (
  id CHAR(36) PRIMARY KEY,
  user_id CHAR(36),
  action_type VARCHAR(64) NOT NULL,
  target_id CHAR(36),
  details JSON NULL,
  ip_address VARCHAR(45),
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_activity_user ON user_activity_logs(user_id);

-- Attachments
CREATE TABLE IF NOT EXISTS attachments (
  id CHAR(36) PRIMARY KEY,
  challenge_id CHAR(36) NOT NULL,
  file_name VARCHAR(255),
  storage_path VARCHAR(1024),
  file_type VARCHAR(50),
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (challenge_id) REFERENCES challenges(id) ON DELETE CASCADE
);

-- Docker images
CREATE TABLE IF NOT EXISTS docker_images (
  id CHAR(36) PRIMARY KEY,
  challenge_id CHAR(36) NOT NULL,
  image_name VARCHAR(255) NOT NULL,
  image_tag VARCHAR(128),
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (challenge_id) REFERENCES challenges(id) ON DELETE CASCADE
);

-- Demo data: users
INSERT INTO users (id, username, email, password_hash, role, display_name, bio, real_name, organization, country)
VALUES (UUID(), 'alice', 'alice@example.com', '$2y$12$EXAMPLEHASHFORDEMO', 'user', 'Alice', 'Demo user Alice', 'Alice L', 'University A', 'CN'),
       (UUID(), 'bob', 'bob@example.com', '$2y$12$EXAMPLEHASHFORDEMO', 'admin', 'Bob', 'Demo admin Bob', 'Bob K', 'University B', 'US');

-- Demo data: teams
INSERT INTO teams (id, name, invite_code, score, solved_count) VALUES (UUID(), 'Team Phoenix', 'PHOENIX8', 0, 0), (UUID(), 'Red Team', 'REDTEAM1', 0, 0);

-- Demo challenges (matching frontend demo problems)
INSERT INTO challenges (id, short_id, title, category, points, difficulty, description, flag_rule, flag_example)
VALUES
  (UUID(), 1, 'Hello Web', 'web', 100, '简单', '一个简单的 Web 题，目标是找到隐藏在页面中的 flag。', 'fixed', 'FLAG{demo}'),
  (UUID(), 2, 'babyheap', 'pwn', 300, '困难', 'pwn 题，练习 heap 漏洞利用。', 'fixed', 'FLAG{pwn_demo}'),
  (UUID(), 3, 'easycrypto', 'crypto', 150, '中等', '基础的密码学题目，包含简短的 RSA 破解思路。', 'fixed', 'FLAG{crypto_demo}'),
  (UUID(), 4, 'reverse-me', 'reverse', 200, '中等', '反汇编与逆向题，需要分析二进制行为。', 'fixed', 'FLAG{rev_demo}'),
  (UUID(), 5, 'misc-challenge', 'misc', 120, '简单', '杂项题目，考察逻辑与观察能力。', 'fixed', 'FLAG{misc_demo}');

-- Demo tags
INSERT INTO challenge_tags (challenge_id, tag) SELECT id, 'http' FROM challenges WHERE short_id = 1;
INSERT INTO challenge_tags (challenge_id, tag) SELECT id, 'xss' FROM challenges WHERE short_id = 1;
INSERT INTO challenge_tags (challenge_id, tag) SELECT id, 'heap' FROM challenges WHERE short_id = 2;
INSERT INTO challenge_tags (challenge_id, tag) SELECT id, 'exploit' FROM challenges WHERE short_id = 2;
INSERT INTO challenge_tags (challenge_id, tag) SELECT id, 'rsa' FROM challenges WHERE short_id = 3;
INSERT INTO challenge_tags (challenge_id, tag) SELECT id, 'crypto' FROM challenges WHERE short_id = 3;
INSERT INTO challenge_tags (challenge_id, tag) SELECT id, 'binary' FROM challenges WHERE short_id = 4;
INSERT INTO challenge_tags (challenge_id, tag) SELECT id, 'reverse' FROM challenges WHERE short_id = 4;
INSERT INTO challenge_tags (challenge_id, tag) SELECT id, 'logic' FROM challenges WHERE short_id = 5;

-- Demo submission: alice solves Hello Web
INSERT INTO submissions (id, user_id, challenge_id, submitted_flag, is_correct, score_awarded)
SELECT UUID(), u.id, c.id, c.flag_example, TRUE, c.points FROM users u, challenges c WHERE u.username = 'alice' AND c.short_id = 1 LIMIT 1;

-- Demo contest
INSERT INTO contests (id, title, description, start_time, end_time) VALUES (UUID(), 'Demo Contest', 'A short demo contest for local development', NOW(), DATE_ADD(NOW(), INTERVAL 2 HOUR));

-- End of script
