-- ==========================================================
-- CTF Platform - Consolidated Initialization Script v2
-- Target:      MySQL 8.0+ / MariaDB 10.6+
-- Encoding:    utf8mb4 / utf8mb4_unicode_ci
-- Updated:     2026-03-22
-- Changes:     Integrate RBAC v2 (site_admin / contest_admins)
--              Add problems.scope / contest_id / created_by / status
--              Add contests.created_by / visibility
--              Add contest_participants.status
--              Add role_change_logs audit table
-- ==========================================================

CREATE DATABASE IF NOT EXISTS ctf_exec
    CHARACTER SET utf8mb4
    COLLATE utf8mb4_unicode_ci;

USE ctf_exec;

SET FOREIGN_KEY_CHECKS = 0;

-- ----------------------------------------------------------
-- Drop tables (dependency order: children first)
-- ----------------------------------------------------------
DROP TABLE IF EXISTS role_change_logs;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS contest_admins;
DROP TABLE IF EXISTS user_activity_logs;
DROP TABLE IF EXISTS contest_leaderboards;
DROP TABLE IF EXISTS contest_scores;
DROP TABLE IF EXISTS contest_participants;
DROP TABLE IF EXISTS contest_challenges;
DROP TABLE IF EXISTS leaderboard_global;
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

-- ==========================================================
-- 1. Users
-- ==========================================================

CREATE TABLE users (
    id                    CHAR(36)      NOT NULL,
    username              VARCHAR(32)   NOT NULL,
    email                 VARCHAR(254)  NULL,
    password_hash         VARCHAR(255)  NOT NULL,

    -- Global role: root / site_admin / user
    -- contest_admin is expressed via contest_admins table, not here
    role                  VARCHAR(16)   NOT NULL DEFAULT 'user',

    display_name          VARCHAR(64)   NULL,
    avatar_url            VARCHAR(1024) NULL,
    bio                   TEXT          NULL,
    website_url           VARCHAR(512)  NULL,
    github_username       VARCHAR(100)  NULL,
    twitter_username      VARCHAR(100)  NULL,

    real_name             VARCHAR(100)  NULL,
    student_id            VARCHAR(50)   NULL,
    organization          VARCHAR(100)  NULL,
    country               CHAR(2)       NULL,

    -- Global score & rank (platform-wide, separate from contest scores)
    score                 INT           NOT NULL DEFAULT 0,
    solved_count          INT           NOT NULL DEFAULT 0,
    rank_num              INT           NULL,

    is_active             TINYINT(1)    NOT NULL DEFAULT 1,
    is_verified           TINYINT(1)    NOT NULL DEFAULT 0,

    twofa_enabled         TINYINT(1)    NOT NULL DEFAULT 0,
    twofa_secret          VARCHAR(100)  NULL,
    twofa_recovery_codes  TEXT          NULL,

    auth_provider         VARCHAR(32)   NOT NULL DEFAULT 'local',
    external_id           VARCHAR(255)  NULL,

    password_reset_token  VARCHAR(128)  NULL,
    password_reset_expires DATETIME     NULL,

    last_login_at         DATETIME      NULL,
    last_login_ip         VARCHAR(45)   NULL,
    registration_ip       VARCHAR(45)   NULL,

    preferences           JSON          NOT NULL DEFAULT (JSON_OBJECT()),

    created_at            DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at            DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP
                                            ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (id),
    UNIQUE KEY uniq_users_username (username),
    UNIQUE KEY uniq_users_email    (email),

    CONSTRAINT chk_username_format CHECK (username REGEXP '^[a-zA-Z0-9_]{3,32}$'),
    CONSTRAINT chk_role_valid      CHECK (role IN ('root', 'site_admin', 'user')),
    CONSTRAINT chk_score_positive  CHECK (score >= 0)
) ENGINE=InnoDB;

CREATE INDEX idx_users_score ON users(score DESC, last_login_at ASC);
CREATE INDEX idx_users_role  ON users(role);
-- 全站榜定时快照查询用：按 score 降序 + 创建时间升序
CREATE INDEX idx_users_leaderboard ON users(score DESC, created_at ASC);


-- ==========================================================
-- 2. Global Leaderboard Snapshot
-- ==========================================================

CREATE TABLE leaderboard_global (
    id            BIGINT        NOT NULL AUTO_INCREMENT,
    board_type    VARCHAR(16)   NOT NULL,   -- 'user' | 'team'

    rank_num      INT           NOT NULL,
    ref_id        CHAR(36)      NOT NULL,   -- user.id or team.id

    username      VARCHAR(32)   NULL,
    display_name  VARCHAR(64)   NULL,
    avatar_url    VARCHAR(1024) NULL,
    team_name     VARCHAR(100)  NULL,

    -- User board fields
    score         INT           NOT NULL DEFAULT 0,
    solved_count  INT           NOT NULL DEFAULT 0,

    -- Team board aggregation fields
    total_score   INT           NOT NULL DEFAULT 0,
    avg_score     DECIMAL(10,2) NOT NULL DEFAULT 0,
    member_count  INT           NOT NULL DEFAULT 0,

    snapshot_at   DATETIME      NOT NULL,
    created_at    DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,

    is_current    TINYINT(1)   NOT NULL DEFAULT 1,  -- 1=当前有效快照, 0=历史快照

    PRIMARY KEY (id),

    CONSTRAINT chk_board_type CHECK (board_type IN ('user', 'team')),
    UNIQUE KEY uniq_board_rank (board_type, rank_num),
    UNIQUE KEY uniq_board_ref  (board_type, ref_id)
) ENGINE=InnoDB;

CREATE INDEX idx_leaderboard_board_rank ON leaderboard_global(board_type, rank_num);
CREATE INDEX idx_leaderboard_board_user ON leaderboard_global(board_type, username);

-- ==========================================================
-- 3. Teams
-- ==========================================================

CREATE TABLE teams (
    id           CHAR(36)     NOT NULL,
    name         VARCHAR(100) NOT NULL,
    invite_code  VARCHAR(32)  NOT NULL,
    leader_id    CHAR(36)     NULL,

    score        INT          NOT NULL DEFAULT 0,
    solved_count INT          NOT NULL DEFAULT 0,
    rank_cache   INT          NULL,

    created_at   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP
                                  ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (id),
    UNIQUE KEY uniq_teams_name        (name),
    UNIQUE KEY uniq_teams_invite_code (invite_code),

    FOREIGN KEY fk_teams_leader (leader_id)
        REFERENCES users(id) ON DELETE SET NULL
) ENGINE=InnoDB;

-- ----------------------------------------------------------
-- 3.1 Team Members
-- ----------------------------------------------------------

CREATE TABLE team_members (
    team_id      CHAR(36)    NOT NULL,
    user_id      CHAR(36)    NOT NULL,
    role_in_team VARCHAR(16) NOT NULL DEFAULT 'member',  -- 'leader' | 'member'
    joined_at    DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (team_id, user_id),

    FOREIGN KEY fk_team_members_team (team_id)
        REFERENCES teams(id) ON DELETE CASCADE,
    FOREIGN KEY fk_team_members_user (user_id)
        REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- ==========================================================
-- 4. Contests
-- ==========================================================

CREATE TABLE contests (
    id            CHAR(36)      NOT NULL,
    title         VARCHAR(255)  NOT NULL,
    description   TEXT          NULL,
    cover_image   VARCHAR(1024) NULL,

    start_time    DATETIME      NULL,
    end_time      DATETIME      NULL,

    scoring_type  VARCHAR(32)   NOT NULL DEFAULT 'jeopardy',

    -- Visibility controls who can access/see the contest
    -- 'public'       : listed publicly, open registration
    -- 'invite_only'  : listed but requires invite_code to register
    -- 'private'      : unlisted, admin access only (internal/test)
    visibility    VARCHAR(16)   NOT NULL DEFAULT 'public',

    -- Who created this contest (site_admin or root)
    created_by    CHAR(36)      NULL,

    created_at    DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP
                                    ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (id),

    CONSTRAINT chk_contest_visibility CHECK (visibility IN ('public', 'invite_only', 'private')),

    FOREIGN KEY fk_contests_created_by (created_by)
        REFERENCES users(id) ON DELETE SET NULL
) ENGINE=InnoDB;

-- ==========================================================
-- 5. Problems (Challenges)
-- ==========================================================

CREATE TABLE problems (
    id            INT          NOT NULL AUTO_INCREMENT,
    short_id      INT          NULL,

    title         VARCHAR(255) NOT NULL,
    category      VARCHAR(32)  NOT NULL,
    type          VARCHAR(32)  NOT NULL DEFAULT 'static',  -- 'static' | 'dynamic'

    points        INT          NOT NULL DEFAULT 0,
    difficulty    VARCHAR(16)  NOT NULL,                   -- '简单' | '中等' | '困难'

    description   TEXT         NULL,

    -- Store hash only; never store raw flag in DB
    flag_hash     VARCHAR(255) NOT NULL,
    flag_rule     VARCHAR(255) NULL,
    flag_example  VARCHAR(255) NULL,

    docker_image  VARCHAR(255) NULL,
    run_command   TEXT         NULL,

    -- Creator (site_admin or contest_admin)
    created_by    CHAR(36)     NULL,

    -- Scope controls visibility domain
    -- 'platform' : public practice library, managed by site_admin
    -- 'contest'  : scoped to one contest, auto-archived after end_time
    scope         VARCHAR(16)  NOT NULL DEFAULT 'platform',

    -- Populated only when scope = 'contest'
    contest_id    CHAR(36)     NULL,

    -- Publication lifecycle
    -- 'draft'     : visible to creator only
    -- 'review'    : pending site_admin approval
    -- 'published' : visible to eligible users
    -- 'archived'  : hidden, read-only
    status        VARCHAR(16)  NOT NULL DEFAULT 'published',

    -- Effective window for platform-scope problems (ignored for contest-scope)
    visible_from  DATETIME     NULL,
    visible_to    DATETIME     NULL,

    created_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP
                                   ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (id),
    UNIQUE KEY uniq_problems_short_id (short_id),

    CONSTRAINT chk_problem_scope  CHECK (scope  IN ('platform', 'contest')),
    CONSTRAINT chk_problem_status CHECK (status IN ('draft', 'review', 'published', 'archived')),

    FOREIGN KEY fk_problems_created_by (created_by)
        REFERENCES users(id)    ON DELETE SET NULL,
    FOREIGN KEY fk_problems_contest_id (contest_id)
        REFERENCES contests(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE INDEX idx_problems_scope      ON problems(scope);
CREATE INDEX idx_problems_status     ON problems(status);
CREATE INDEX idx_problems_created_by ON problems(created_by);

-- ----------------------------------------------------------
-- 5.1 Challenge Tags
-- ----------------------------------------------------------

CREATE TABLE challenge_tags (
    challenge_id INT         NOT NULL,
    tag          VARCHAR(64) NOT NULL,

    PRIMARY KEY (challenge_id, tag),

    FOREIGN KEY fk_challenge_tags_problem (challenge_id)
        REFERENCES problems(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- ==========================================================
-- 6. Submissions
-- ==========================================================

CREATE TABLE submissions (
    id              CHAR(36)     NOT NULL,
    user_id         CHAR(36)     NULL,
    team_id         CHAR(36)     NULL,
    problem_id      INT          NOT NULL,
    contest_id      CHAR(36)     NULL,   -- NULL = practice (non-contest) submission

    flag            VARCHAR(1024) NULL,
    is_correct      TINYINT(1)   NOT NULL DEFAULT 0,
    score_awarded   INT          NOT NULL DEFAULT 0,

    ip_address      VARCHAR(45)  NULL,
    run_instance_id CHAR(36)     NULL,
    meta            JSON         NULL,

    created_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),

    FOREIGN KEY fk_submissions_user    (user_id)    REFERENCES users(id)    ON DELETE SET NULL,
    FOREIGN KEY fk_submissions_team    (team_id)    REFERENCES teams(id)    ON DELETE SET NULL,
    FOREIGN KEY fk_submissions_problem (problem_id) REFERENCES problems(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- User / problem lookup
CREATE INDEX idx_sub_user          ON submissions(user_id);
CREATE INDEX idx_sub_problem       ON submissions(problem_id);
-- Fast "has user solved this problem?" check
CREATE INDEX idx_sub_user_problem  ON submissions(user_id, problem_id);
-- First-blood queries
CREATE INDEX idx_sub_problem_correct ON submissions(problem_id, is_correct);
-- Scoreboard timeline
CREATE INDEX idx_sub_user_time     ON submissions(user_id, created_at);
-- Contest submission dedup & scoreboard
CREATE INDEX idx_sub_contest_user_problem_correct
    ON submissions(contest_id, user_id, problem_id, is_correct);
-- 非赛事正确提交去重：WHERE user_id = ? AND problem_id = ? AND is_correct = 1 AND contest_id IS NULL
CREATE INDEX idx_sub_user_problem_noc ON submissions(user_id, problem_id, is_correct, contest_id);


-- ==========================================================
-- 6.1 Refresh Tokens (Persistent, DB-backed)
-- ==========================================================
-- Refresh token 不走 Redis，持久化到 MySQL，避免 Redis 重启丢失。
-- 存储 SHA256 哈希，不存明文。
-- 每次 refresh rotation 旧 token 标记 revoked=1，插入新行。

CREATE TABLE refresh_tokens (
    id          CHAR(36)    NOT NULL,
    user_id     CHAR(36)    NOT NULL,
    token_hash  CHAR(64)    NOT NULL,       -- SHA256(refresh_token)
    jti         CHAR(36)    NOT NULL,       -- 关联 access token 的 jti，用于登出时吊销 access
    expires_at  DATETIME    NOT NULL,
    revoked     TINYINT(1)  NOT NULL DEFAULT 0,
    ip_address  VARCHAR(45) NULL,           -- 签发时的客户端 IP
    user_agent  VARCHAR(512) NULL,          -- 签发时的 UA（可选，用于会话管理）
    created_at  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),

    -- 登录时快速校验 token 有效性
    UNIQUE KEY uniq_rt_hash (token_hash),

    -- 按用户查询活跃会话（"管理登录设备"功能）
    INDEX idx_rt_user_active (user_id, revoked, expires_at),

    -- 定时清理过期 token
    INDEX idx_rt_expires (expires_at),

    CONSTRAINT fk_rt_user FOREIGN KEY (user_id)
        REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- ==========================================================
-- 7. Challenge Instances (Dynamic Challenges)
-- ==========================================================

CREATE TABLE challenge_instances (
    id             CHAR(36)    NOT NULL,
    challenge_id   INT         NOT NULL,
    owner_user_id  CHAR(36)    NULL,
    owner_team_id  CHAR(36)    NULL,

    pod_name       VARCHAR(255) NULL,
    job_id         VARCHAR(255) NULL,
    status         VARCHAR(32)  NOT NULL DEFAULT 'running',
    address        VARCHAR(255) NULL,

    created_at     DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    destroyed_at   DATETIME    NULL,
    meta           JSON        NULL,

    PRIMARY KEY (id),

    FOREIGN KEY fk_ci_challenge  (challenge_id)  REFERENCES problems(id) ON DELETE CASCADE,
    FOREIGN KEY fk_ci_owner_user (owner_user_id) REFERENCES users(id)    ON DELETE SET NULL,
    FOREIGN KEY fk_ci_owner_team (owner_team_id) REFERENCES teams(id)    ON DELETE SET NULL
) ENGINE=InnoDB;

-- ==========================================================
-- 8. Attachments
-- ==========================================================

CREATE TABLE attachments (
    id            CHAR(36)      NOT NULL,
    challenge_id  INT           NOT NULL,
    file_name     VARCHAR(255)  NULL,
    storage_path  VARCHAR(1024) NULL,
    file_type     VARCHAR(50)   NULL,
    created_at    DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),

    FOREIGN KEY fk_attachments_problem (challenge_id)
        REFERENCES problems(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- ==========================================================
-- 9. Docker Images
-- ==========================================================

CREATE TABLE docker_images (
    id            CHAR(36)     NOT NULL,
    challenge_id  INT          NOT NULL,
    image_name    VARCHAR(255) NOT NULL,
    image_tag     VARCHAR(128) NULL,
    created_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),

    FOREIGN KEY fk_docker_images_problem (challenge_id)
        REFERENCES problems(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- ==========================================================
-- 10. Contest Admins (Resource-level RBAC)
-- ==========================================================
-- A user listed here has contest-scoped admin rights.
-- root and site_admin implicitly have all contest rights;
-- they do NOT need entries here.
-- Maximum 10 admins per contest is enforced at service layer.

CREATE TABLE contest_admins (
    id          CHAR(36)    NOT NULL,
    contest_id  CHAR(36)    NOT NULL,
    user_id     CHAR(36)    NOT NULL,

    -- 'admin'    : full contest management (problems, participants, meta)
    -- 'observer' : read-only; useful for judges / invigilators
    role        VARCHAR(16) NOT NULL DEFAULT 'admin',

    granted_by  CHAR(36)    NULL,   -- the site_admin / root who granted this
    granted_at  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),

    CONSTRAINT chk_contest_admin_role CHECK (role IN ('admin', 'observer')),
    UNIQUE KEY uniq_contest_admin_user (contest_id, user_id),

    FOREIGN KEY fk_ca_contest    (contest_id) REFERENCES contests(id) ON DELETE CASCADE,
    FOREIGN KEY fk_ca_user       (user_id)    REFERENCES users(id)    ON DELETE CASCADE,
    FOREIGN KEY fk_ca_granted_by (granted_by) REFERENCES users(id)    ON DELETE SET NULL
) ENGINE=InnoDB;

CREATE INDEX idx_contest_admins_contest ON contest_admins(contest_id);
CREATE INDEX idx_contest_admins_user    ON contest_admins(user_id);

-- ==========================================================
-- 11. Contest Challenges
-- ==========================================================

CREATE TABLE contest_challenges (
    contest_id     CHAR(36) NOT NULL,
    challenge_id   INT      NOT NULL,
    score_override INT      NULL,   -- NULL = use problems.points

    PRIMARY KEY (contest_id, challenge_id),

    FOREIGN KEY fk_cc_contest   (contest_id)   REFERENCES contests(id)  ON DELETE CASCADE,
    FOREIGN KEY fk_cc_challenge (challenge_id) REFERENCES problems(id)  ON DELETE CASCADE
) ENGINE=InnoDB;

-- ==========================================================
-- 12. Contest Participants
-- ==========================================================

CREATE TABLE contest_participants (
    id            CHAR(36)  NOT NULL,
    contest_id    CHAR(36)  NOT NULL,
    user_id       CHAR(36)  NULL,   -- NULL when team participates
    team_id       CHAR(36)  NULL,   -- NULL when individual participates

    -- Registration workflow
    -- 'pending'  : awaiting contest_admin approval
    -- 'approved' : confirmed participant
    -- 'rejected' : denied by contest_admin
    status        VARCHAR(16) NOT NULL DEFAULT 'approved',

    registered_at DATETIME  NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),

    CONSTRAINT chk_cp_status CHECK (status IN ('pending', 'approved', 'rejected')),

    FOREIGN KEY fk_cp_contest (contest_id) REFERENCES contests(id) ON DELETE CASCADE,
    FOREIGN KEY fk_cp_user    (user_id)    REFERENCES users(id)    ON DELETE CASCADE,
    FOREIGN KEY fk_cp_team    (team_id)    REFERENCES teams(id)    ON DELETE CASCADE
) ENGINE=InnoDB;

-- Prevent duplicate individual registration
CREATE UNIQUE INDEX uniq_cp_contest_user ON contest_participants(contest_id, user_id);
-- Prevent duplicate team registration
CREATE UNIQUE INDEX uniq_cp_contest_team ON contest_participants(contest_id, team_id);

-- ==========================================================
-- 13. Contest Scores
-- ==========================================================

CREATE TABLE contest_scores (
    id              CHAR(36)  NOT NULL,
    contest_id      CHAR(36)  NOT NULL,
    user_id         CHAR(36)  NULL,
    team_id         CHAR(36)  NULL,

    score           INT       NOT NULL DEFAULT 0,
    solved_count    INT       NOT NULL DEFAULT 0,
    last_solve_time DATETIME  NULL,

    updated_at      DATETIME  NOT NULL DEFAULT CURRENT_TIMESTAMP
                                  ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (id),

    UNIQUE KEY uniq_cs_contest_user (contest_id, user_id),
    UNIQUE KEY uniq_cs_contest_team (contest_id, team_id),

    FOREIGN KEY fk_cs_contest (contest_id) REFERENCES contests(id) ON DELETE CASCADE,
    FOREIGN KEY fk_cs_user    (user_id)    REFERENCES users(id)    ON DELETE CASCADE,
    FOREIGN KEY fk_cs_team    (team_id)    REFERENCES teams(id)    ON DELETE CASCADE
) ENGINE=InnoDB;

-- ==========================================================
-- 14. Contest Realtime Leaderboard Metadata
-- ==========================================================

CREATE TABLE contest_leaderboards (
    contest_id     CHAR(36)     NOT NULL,
    mode           VARCHAR(32)  NOT NULL DEFAULT 'redis_realtime',
    redis_user_key VARCHAR(255) NOT NULL,
    redis_team_key VARCHAR(255) NOT NULL,
    is_active      TINYINT(1)   NOT NULL DEFAULT 1,
    last_synced_at DATETIME     NULL,
    created_at     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP
                                    ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (contest_id),

    CONSTRAINT chk_lb_mode CHECK (mode IN ('redis_realtime', 'snapshot')),

    FOREIGN KEY fk_cl_contest (contest_id) REFERENCES contests(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- ==========================================================
-- 15. User Activity Logs
-- ==========================================================
-- General-purpose audit log.
-- action_type examples:
--   login, logout, submit_flag, join_team, leave_team
--   contest_admin_add, contest_admin_remove
--   contest_create, contest_update
--   problem_publish, problem_archive
--   user_ban, user_unban
-- (role_change events go to role_change_logs for structured queries)

CREATE TABLE user_activity_logs (
    id          CHAR(36)    NOT NULL,
    user_id     CHAR(36)    NULL,
    action_type VARCHAR(64) NOT NULL,
    target_id   CHAR(36)    NULL,
    details     JSON        NULL,
    ip_address  VARCHAR(45) NULL,
    created_at  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),

    FOREIGN KEY fk_ual_user (user_id) REFERENCES users(id) ON DELETE SET NULL
) ENGINE=InnoDB;

CREATE INDEX idx_ual_action_time ON user_activity_logs(action_type, created_at);
CREATE INDEX idx_ual_user        ON user_activity_logs(user_id);

-- ==========================================================
-- 16. Role Change Logs (Structured Audit)
-- ==========================================================
-- Only for global role mutations (root upgrades/downgrades site_admin).
-- contest_admin grants are tracked in user_activity_logs.

CREATE TABLE role_change_logs (
    id          CHAR(36)     NOT NULL,
    operator_id CHAR(36)     NULL,    -- who performed the change (root)
    target_id   CHAR(36)     NOT NULL,
    old_role    VARCHAR(16)  NOT NULL,
    new_role    VARCHAR(16)  NOT NULL,
    reason      VARCHAR(255) NULL,
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),

    FOREIGN KEY fk_rcl_operator (operator_id) REFERENCES users(id) ON DELETE SET NULL,
    FOREIGN KEY fk_rcl_target   (target_id)   REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE INDEX idx_rcl_target   ON role_change_logs(target_id);
CREATE INDEX idx_rcl_operator ON role_change_logs(operator_id);



-- ==========================================================
-- Demo Data
-- ==========================================================

-- ----------------------------------------------------------
-- Users
-- All demo passwords = "password" (bcrypt, cost 10)
-- ----------------------------------------------------------

INSERT INTO users
    (id, username, email, password_hash, role,
     display_name, bio, real_name, organization, country,
     is_active, is_verified)
VALUES
    ('00000000-0000-0000-0000-000000000001',
     'root', 'root@ctf.local',
     '$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe',
     'root', 'Root', 'Default root account',
     'System Root', 'CTF Platform', 'CN', 1, 1),

    ('11111111-1111-1111-1111-111111111111',
     'alice', 'alice@example.com',
     '$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe',
     'site_admin', 'Alice', 'Demo site_admin Alice',
     'Alice L', 'University A', 'CN', 1, 1),

    ('22222222-2222-2222-2222-222222222222',
     'bob', 'bob@example.com',
     '$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe',
     'user', 'Bob', 'Demo user Bob',
     'Bob K', 'University B', 'US', 1, 1);

INSERT INTO users
    (id, username, email, password_hash, role,
     display_name, bio, real_name, organization, country,
     is_active, is_verified)
VALUES
    ('30000000-0000-0000-0000-000000000003', 'user03', 'user03@example.com', '$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe', 'user', 'User03', 'Demo user 03', 'User 03', 'University C', 'CN', 1, 1),
    ('30000000-0000-0000-0000-000000000004', 'user04', 'user04@example.com', '$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe', 'user', 'User04', 'Demo user 04', 'User 04', 'University C', 'CN', 1, 1),
    ('30000000-0000-0000-0000-000000000005', 'user05', 'user05@example.com', '$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe', 'user', 'User05', 'Demo user 05', 'User 05', 'University D', 'CN', 1, 1),
    ('30000000-0000-0000-0000-000000000006', 'user06', 'user06@example.com', '$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe', 'user', 'User06', 'Demo user 06', 'User 06', 'University D', 'CN', 1, 1),
    ('30000000-0000-0000-0000-000000000007', 'user07', 'user07@example.com', '$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe', 'user', 'User07', 'Demo user 07', 'User 07', 'University E', 'CN', 1, 1),
    ('30000000-0000-0000-0000-000000000008', 'user08', 'user08@example.com', '$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe', 'user', 'User08', 'Demo user 08', 'User 08', 'University E', 'CN', 1, 1),
    ('30000000-0000-0000-0000-000000000009', 'user09', 'user09@example.com', '$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe', 'user', 'User09', 'Demo user 09', 'User 09', 'University F', 'CN', 1, 1),
    ('30000000-0000-0000-0000-000000000010', 'user10', 'user10@example.com', '$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe', 'user', 'User10', 'Demo user 10', 'User 10', 'University F', 'CN', 1, 1),
    ('30000000-0000-0000-0000-000000000011', 'user11', 'user11@example.com', '$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe', 'user', 'User11', 'Demo user 11', 'User 11', 'University G', 'CN', 1, 1),
    ('30000000-0000-0000-0000-000000000012', 'user12', 'user12@example.com', '$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe', 'user', 'User12', 'Demo user 12', 'User 12', 'University G', 'CN', 1, 1),
    ('30000000-0000-0000-0000-000000000013', 'user13', 'user13@example.com', '$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe', 'user', 'User13', 'Demo user 13', 'User 13', 'University H', 'CN', 1, 1),
    ('30000000-0000-0000-0000-000000000014', 'user14', 'user14@example.com', '$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe', 'user', 'User14', 'Demo user 14', 'User 14', 'University H', 'CN', 1, 1),
    ('30000000-0000-0000-0000-000000000015', 'user15', 'user15@example.com', '$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe', 'user', 'User15', 'Demo user 15', 'User 15', 'University I', 'CN', 1, 1),
    ('30000000-0000-0000-0000-000000000016', 'user16', 'user16@example.com', '$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe', 'user', 'User16', 'Demo user 16', 'User 16', 'University I', 'CN', 1, 1),
    ('30000000-0000-0000-0000-000000000017', 'user17', 'user17@example.com', '$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe', 'user', 'User17', 'Demo user 17', 'User 17', 'University J', 'CN', 1, 1),
    ('30000000-0000-0000-0000-000000000018', 'user18', 'user18@example.com', '$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe', 'user', 'User18', 'Demo user 18', 'User 18', 'University J', 'CN', 1, 1),
    ('30000000-0000-0000-0000-000000000019', 'user19', 'user19@example.com', '$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe', 'user', 'User19', 'Demo user 19', 'User 19', 'University K', 'CN', 1, 1),
    ('30000000-0000-0000-0000-000000000020', 'user20', 'user20@example.com', '$2a$10$fxXO7SQMxuwppXDth8.3reqW5H6Tj5E/aExSB19Fa2p4/yH0FG/pe', 'user', 'User20', 'Demo user 20', 'User 20', 'University K', 'CN', 1, 1);

-- ----------------------------------------------------------
-- Teams
-- ----------------------------------------------------------

INSERT INTO teams (id, name, invite_code, leader_id)
VALUES
    ('5be521ef-1d35-11f1-a099-da2caeb72bcd', 'Team Phoenix', 'PHOENIX8', '11111111-1111-1111-1111-111111111111'),
    ('6be521ef-1d35-11f1-a099-da2caeb72bcd', 'Red Team',     'REDTEAM1', '22222222-2222-2222-2222-222222222222');

INSERT INTO teams (id, name, invite_code, leader_id)
VALUES
    ('70000000-0000-0000-0000-000000000003', 'Team 03', 'TEAM0003', '30000000-0000-0000-0000-000000000003'),
    ('70000000-0000-0000-0000-000000000004', 'Team 04', 'TEAM0004', '30000000-0000-0000-0000-000000000004'),
    ('70000000-0000-0000-0000-000000000005', 'Team 05', 'TEAM0005', '30000000-0000-0000-0000-000000000005'),
    ('70000000-0000-0000-0000-000000000006', 'Team 06', 'TEAM0006', '30000000-0000-0000-0000-000000000006'),
    ('70000000-0000-0000-0000-000000000007', 'Team 07', 'TEAM0007', '30000000-0000-0000-0000-000000000007'),
    ('70000000-0000-0000-0000-000000000008', 'Team 08', 'TEAM0008', '30000000-0000-0000-0000-000000000008'),
    ('70000000-0000-0000-0000-000000000009', 'Team 09', 'TEAM0009', '30000000-0000-0000-0000-000000000009'),
    ('70000000-0000-0000-0000-000000000010', 'Team 10', 'TEAM0010', '30000000-0000-0000-0000-000000000010'),
    ('70000000-0000-0000-0000-000000000011', 'Team 11', 'TEAM0011', '30000000-0000-0000-0000-000000000011'),
    ('70000000-0000-0000-0000-000000000012', 'Team 12', 'TEAM0012', '30000000-0000-0000-0000-000000000012'),
    ('70000000-0000-0000-0000-000000000013', 'Team 13', 'TEAM0013', '30000000-0000-0000-0000-000000000013'),
    ('70000000-0000-0000-0000-000000000014', 'Team 14', 'TEAM0014', '30000000-0000-0000-0000-000000000014'),
    ('70000000-0000-0000-0000-000000000015', 'Team 15', 'TEAM0015', '30000000-0000-0000-0000-000000000015'),
    ('70000000-0000-0000-0000-000000000016', 'Team 16', 'TEAM0016', '30000000-0000-0000-0000-000000000016'),
    ('70000000-0000-0000-0000-000000000017', 'Team 17', 'TEAM0017', '30000000-0000-0000-0000-000000000017'),
    ('70000000-0000-0000-0000-000000000018', 'Team 18', 'TEAM0018', '30000000-0000-0000-0000-000000000018'),
    ('70000000-0000-0000-0000-000000000019', 'Team 19', 'TEAM0019', '30000000-0000-0000-0000-000000000019'),
    ('70000000-0000-0000-0000-000000000020', 'Team 20', 'TEAM0020', '30000000-0000-0000-0000-000000000020');

-- ----------------------------------------------------------
-- Team Members
-- ----------------------------------------------------------

INSERT INTO team_members (team_id, user_id, role_in_team, joined_at)
VALUES
    ('5be521ef-1d35-11f1-a099-da2caeb72bcd', '11111111-1111-1111-1111-111111111111', 'leader', CURRENT_TIMESTAMP),
    ('6be521ef-1d35-11f1-a099-da2caeb72bcd', '22222222-2222-2222-2222-222222222222', 'leader', CURRENT_TIMESTAMP);

INSERT INTO team_members (team_id, user_id, role_in_team, joined_at)
VALUES
    ('70000000-0000-0000-0000-000000000003', '30000000-0000-0000-0000-000000000003', 'leader', CURRENT_TIMESTAMP),
    ('70000000-0000-0000-0000-000000000004', '30000000-0000-0000-0000-000000000004', 'leader', CURRENT_TIMESTAMP),
    ('70000000-0000-0000-0000-000000000005', '30000000-0000-0000-0000-000000000005', 'leader', CURRENT_TIMESTAMP),
    ('70000000-0000-0000-0000-000000000006', '30000000-0000-0000-0000-000000000006', 'leader', CURRENT_TIMESTAMP),
    ('70000000-0000-0000-0000-000000000007', '30000000-0000-0000-0000-000000000007', 'leader', CURRENT_TIMESTAMP),
    ('70000000-0000-0000-0000-000000000008', '30000000-0000-0000-0000-000000000008', 'leader', CURRENT_TIMESTAMP),
    ('70000000-0000-0000-0000-000000000009', '30000000-0000-0000-0000-000000000009', 'leader', CURRENT_TIMESTAMP),
    ('70000000-0000-0000-0000-000000000010', '30000000-0000-0000-0000-000000000010', 'leader', CURRENT_TIMESTAMP),
    ('70000000-0000-0000-0000-000000000011', '30000000-0000-0000-0000-000000000011', 'leader', CURRENT_TIMESTAMP),
    ('70000000-0000-0000-0000-000000000012', '30000000-0000-0000-0000-000000000012', 'leader', CURRENT_TIMESTAMP),
    ('70000000-0000-0000-0000-000000000013', '30000000-0000-0000-0000-000000000013', 'leader', CURRENT_TIMESTAMP),
    ('70000000-0000-0000-0000-000000000014', '30000000-0000-0000-0000-000000000014', 'leader', CURRENT_TIMESTAMP),
    ('70000000-0000-0000-0000-000000000015', '30000000-0000-0000-0000-000000000015', 'leader', CURRENT_TIMESTAMP),
    ('70000000-0000-0000-0000-000000000016', '30000000-0000-0000-0000-000000000016', 'leader', CURRENT_TIMESTAMP),
    ('70000000-0000-0000-0000-000000000017', '30000000-0000-0000-0000-000000000017', 'leader', CURRENT_TIMESTAMP),
    ('70000000-0000-0000-0000-000000000018', '30000000-0000-0000-0000-000000000018', 'leader', CURRENT_TIMESTAMP),
    ('70000000-0000-0000-0000-000000000019', '30000000-0000-0000-0000-000000000019', 'leader', CURRENT_TIMESTAMP),
    ('70000000-0000-0000-0000-000000000020', '30000000-0000-0000-0000-000000000020', 'leader', CURRENT_TIMESTAMP);

-- ----------------------------------------------------------
-- Problems
-- All demo flags = flag{demo}
-- flag_hash = SHA256('flag{demo}')
-- ----------------------------------------------------------

INSERT INTO problems
    (id, short_id, title, category, points, difficulty,
     description, flag_hash, scope, status, created_by)
VALUES
    (1,  1,  'Hello Web',              'web',     100, '简单', '一个简单的 Web 入门题',   '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4', 'platform', 'published', '11111111-1111-1111-1111-111111111111'),
    (2,  2,  'babyheap',               'pwn',     300, '困难', '经典 heap 漏洞利用',      '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4', 'platform', 'published', '11111111-1111-1111-1111-111111111111'),
    (3,  3,  'easycrypto',             'crypto',  150, '中等', 'RSA 入门题',              '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4', 'platform', 'published', '11111111-1111-1111-1111-111111111111'),
    (4,  4,  'reverse-me',             'reverse', 200, '中等', '简单逆向分析',             '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4', 'platform', 'published', '11111111-1111-1111-1111-111111111111'),
    (5,  5,  'misc-challenge',         'misc',    120, '简单', '基础 Misc 题',             '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4', 'platform', 'published', '11111111-1111-1111-1111-111111111111'),
    (6,  6,  'web-login-bypass',       'web',     130, '简单', '登录绕过基础题',           '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4', 'platform', 'published', '11111111-1111-1111-1111-111111111111'),
    (7,  7,  'web-template-injection', 'web',     220, '中等', '模板注入利用',             '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4', 'platform', 'published', '11111111-1111-1111-1111-111111111111'),
    (8,  8,  'pwn-ret2libc',           'pwn',     260, '中等', 'ret2libc 基础利用',       '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4', 'platform', 'published', '11111111-1111-1111-1111-111111111111'),
    (9,  9,  'pwn-uaf-training',       'pwn',     340, '困难', 'Use-After-Free 训练题',  '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4', 'platform', 'published', '11111111-1111-1111-1111-111111111111'),
    (10, 10, 'crypto-shift',           'crypto',  110, '简单', '古典密码位移分析',         '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4', 'platform', 'published', '11111111-1111-1111-1111-111111111111'),
    (11, 11, 'crypto-rsa-common-mod',  'crypto',  240, '中等', 'RSA 共模攻击',            '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4', 'platform', 'published', '11111111-1111-1111-1111-111111111111'),
    (12, 12, 'crypto-lcg-break',       'crypto',  320, '困难', '线性同余生成器预测',       '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4', 'platform', 'published', '11111111-1111-1111-1111-111111111111'),
    (13, 13, 'reverse-crackme-1',      'reverse', 140, '简单', 'CrackMe 入门',            '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4', 'platform', 'published', '11111111-1111-1111-1111-111111111111'),
    (14, 14, 'reverse-vm-disasm',      'reverse', 280, '中等', '虚拟机指令逆向',           '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4', 'platform', 'published', '11111111-1111-1111-1111-111111111111'),
    (15, 15, 'reverse-anti-debug',     'reverse', 360, '困难', '反调试绕过分析',           '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4', 'platform', 'published', '11111111-1111-1111-1111-111111111111'),
    (16, 16, 'misc-forensics-disk',    'misc',    160, '中等', '磁盘取证基础题',           '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4', 'platform', 'published', '11111111-1111-1111-1111-111111111111'),
    (17, 17, 'misc-stego-audio',       'misc',    180, '中等', '音频隐写分析',             '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4', 'platform', 'published', '11111111-1111-1111-1111-111111111111'),
    (18, 18, 'misc-pcap-hunt',         'misc',    210, '中等', '流量包线索提取',           '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4', 'platform', 'published', '11111111-1111-1111-1111-111111111111'),
    (19, 19, 'web-race-condition',     'web',     310, '困难', '竞态条件漏洞利用',         '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4', 'platform', 'published', '11111111-1111-1111-1111-111111111111'),
    (20, 20, 'pwn-srop-lab',           'pwn',     400, '困难', 'SROP 技术实验题',         '9e3b2d1b1f9c3c9a8f8c0a8d6a9b9b9f4f2c0b1f6cbbd6c8f7b9e7f1c1d9e0e4', 'platform', 'published', '11111111-1111-1111-1111-111111111111');

-- ----------------------------------------------------------
-- Challenge Tags
-- ----------------------------------------------------------

INSERT INTO challenge_tags (challenge_id, tag)
VALUES
    (1, 'web'),     (1, 'http'),          (1, 'beginner'),
    (2, 'pwn'),     (2, 'heap'),          (2, 'glibc'),
    (3, 'crypto'),  (3, 'rsa'),           (3, 'number-theory'),
    (4, 'reverse'), (4, 'binary'),        (4, 'x86'),
    (5, 'misc'),    (5, 'forensics'),     (5, 'steganography');

-- ----------------------------------------------------------
-- Contests
-- created_by = alice (site_admin demo)
-- ----------------------------------------------------------

INSERT INTO contests
    (id, title, description, cover_image,
     start_time, end_time, scoring_type, visibility, created_by)
VALUES
    ('90000000-0000-0000-0000-000000000001', '2026 Spring Warmup',         'Spring warmup contest for newcomers and team collaboration.',     '/static/covers/contest_01.jpg', '2026-03-01 10:00:00', '2026-03-01 18:00:00', 'jeopardy', 'public', '11111111-1111-1111-1111-111111111111'),
    ('90000000-0000-0000-0000-000000000002', 'Weekly Practice #01',        'Weekly practice focused on web and misc basics.',                 '/static/covers/contest_02.jpg', '2026-03-03 19:30:00', '2026-03-03 22:30:00', 'jeopardy', 'public', '11111111-1111-1111-1111-111111111111'),
    ('90000000-0000-0000-0000-000000000003', 'Weekly Practice #02',        'Second weekly practice with crypto and reverse challenges.',      '/static/covers/contest_03.jpg', '2026-03-05 19:30:00', '2026-03-05 22:30:00', 'jeopardy', 'public', '11111111-1111-1111-1111-111111111111'),
    ('90000000-0000-0000-0000-000000000004', 'Weekend Mini CTF #01',       'Short weekend event with mixed categories.',                      '/static/covers/contest_04.jpg', '2026-03-07 14:00:00', '2026-03-07 18:00:00', 'jeopardy', 'public', '11111111-1111-1111-1111-111111111111'),
    ('90000000-0000-0000-0000-000000000005', 'Reverse Monthly Challenge',  'Monthly reverse engineering challenge set.',                      '/static/covers/contest_05.jpg', '2026-03-10 09:00:00', '2026-03-31 23:00:00', 'jeopardy', 'public', '11111111-1111-1111-1111-111111111111'),
    ('90000000-0000-0000-0000-000000000006', 'Crypto Drill Camp',          'Crypto drill camp with progressive RSA and lattice tasks.',       '/static/covers/contest_06.jpg', '2026-03-12 18:00:00', '2026-03-13 23:00:00', 'jeopardy', 'public', '11111111-1111-1111-1111-111111111111'),
    ('90000000-0000-0000-0000-000000000007', 'Web Security Sprint',        'Fast-paced web vulnerabilities sprint.',                          '/static/covers/contest_07.jpg', '2026-03-14 10:00:00', '2026-03-14 20:00:00', 'jeopardy', 'public', '11111111-1111-1111-1111-111111111111'),
    ('90000000-0000-0000-0000-000000000008', 'Pwn Intro Battle',           'Introductory pwn contest for stack and heap basics.',             '/static/covers/contest_08.jpg', '2026-03-15 09:30:00', '2026-03-15 17:30:00', 'jeopardy', 'public', '11111111-1111-1111-1111-111111111111'),
    ('90000000-0000-0000-0000-000000000009', 'Campus Joint Training',      'Inter-university joint training event.',                          '/static/covers/contest_09.jpg', '2026-03-16 13:00:00', '2026-03-16 21:00:00', 'jeopardy', 'public', '11111111-1111-1111-1111-111111111111'),
    ('90000000-0000-0000-0000-000000000010', 'Night Owl CTF #01',          'Night-time short challenge event.',                               '/static/covers/contest_10.jpg', '2026-03-17 20:00:00', '2026-03-18 00:30:00', 'jeopardy', 'public', '11111111-1111-1111-1111-111111111111'),
    ('90000000-0000-0000-0000-000000000011', 'Night Owl CTF #02',          'Second night event with advanced tasks.',                         '/static/covers/contest_11.jpg', '2026-03-19 20:00:00', '2026-03-20 00:30:00', 'jeopardy', 'public', '11111111-1111-1111-1111-111111111111'),
    ('90000000-0000-0000-0000-000000000012', 'University Selection Round A','Selection round A for school team candidates.',                  '/static/covers/contest_12.jpg', '2026-03-22 09:00:00', '2026-03-22 18:00:00', 'jeopardy', 'invite_only', '11111111-1111-1111-1111-111111111111'),
    ('90000000-0000-0000-0000-000000000013', 'University Selection Round B','Selection round B with stricter scoring.',                       '/static/covers/contest_13.jpg', '2026-03-24 09:00:00', '2026-03-24 18:00:00', 'jeopardy', 'invite_only', '11111111-1111-1111-1111-111111111111'),
    ('90000000-0000-0000-0000-000000000014', 'Attack Surface Workshop CTF','Workshop-themed contest focused on service hardening.',           '/static/covers/contest_14.jpg', '2026-03-26 14:00:00', '2026-03-26 22:00:00', 'jeopardy', 'public', '11111111-1111-1111-1111-111111111111'),
    ('90000000-0000-0000-0000-000000000015', 'Beginner Friendly CTF',      'Friendly event for first-time participants.',                     '/static/covers/contest_15.jpg', '2026-03-28 10:00:00', '2026-03-28 19:00:00', 'jeopardy', 'public', '11111111-1111-1111-1111-111111111111'),
    ('90000000-0000-0000-0000-000000000016', 'April Monthly Open',         'Open monthly contest for all registered users.',                  '/static/covers/contest_16.jpg', '2026-04-02 09:00:00', '2026-04-02 23:00:00', 'jeopardy', 'public', '11111111-1111-1111-1111-111111111111'),
    ('90000000-0000-0000-0000-000000000017', 'Weekend Mini CTF #02',       'Second weekend mini CTF with mixed difficulty.',                  '/static/covers/contest_17.jpg', '2026-04-05 14:00:00', '2026-04-05 18:00:00', 'jeopardy', 'public', '11111111-1111-1111-1111-111111111111'),
    ('90000000-0000-0000-0000-000000000018', 'Blue Team Simulation Day',   'Defense-focused simulation and analysis tasks.',                  '/static/covers/contest_18.jpg', '2026-04-08 09:00:00', '2026-04-08 17:00:00', 'jeopardy', 'private', '11111111-1111-1111-1111-111111111111'),
    ('90000000-0000-0000-0000-000000000019', 'Capture Spring Finals',      'Final spring contest with cumulative ranking.',                   '/static/covers/contest_19.jpg', '2026-04-12 10:00:00', '2026-04-12 20:00:00', 'jeopardy', 'public', '11111111-1111-1111-1111-111111111111'),
    ('90000000-0000-0000-0000-000000000020', 'Preparation Cup 2026',       'Preparation cup for upcoming provincial CTF.',                    '/static/covers/contest_20.jpg', '2026-04-15 10:00:00', '2026-04-15 18:00:00', 'jeopardy', 'public', '11111111-1111-1111-1111-111111111111');

-- ----------------------------------------------------------
-- Demo Contest Admins
-- bob (user) is assigned as contest_admin for contest #1
-- granted by alice (site_admin)
-- ----------------------------------------------------------

INSERT INTO contest_admins (id, contest_id, user_id, role, granted_by, granted_at)
VALUES
    ('ca000000-0000-0000-0000-000000000001',
     '90000000-0000-0000-0000-000000000001',
     '22222222-2222-2222-2222-222222222222',
     'admin',
     '11111111-1111-1111-1111-111111111111',
     CURRENT_TIMESTAMP);

-- ==========================================================
-- End of script
-- ==========================================================