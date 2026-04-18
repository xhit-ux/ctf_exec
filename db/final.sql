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
DROP TABLE IF EXISTS image_build_tasks;
DROP TABLE IF EXISTS problem_containers;
DROP TABLE IF EXISTS problem_flags;
DROP TABLE IF EXISTS docker_images;
DROP TABLE IF EXISTS attachments;
DROP TABLE IF EXISTS challenge_instances;
DROP TABLE IF EXISTS problem_tags;
DROP TABLE IF EXISTS challenge_tags;
DROP TABLE IF EXISTS submissions;
DROP TABLE IF EXISTS problems;
DROP TABLE IF EXISTS user_follows;
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
    
    third_user_info       JSON          NULL,   -- For storing extra info from external auth providers

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
    id                 CHAR(36)      NOT NULL,
    title              VARCHAR(128)  NOT NULL,
    category           VARCHAR(32)   NOT NULL,
    type               VARCHAR(16)   NOT NULL DEFAULT 'static',
    description        TEXT          NULL,
    base_score         INT           NOT NULL DEFAULT 0,
    is_dynamic_score   TINYINT(1)    NOT NULL DEFAULT 0,
    difficulty         VARCHAR(16)   NOT NULL,

    status             VARCHAR(16)   NOT NULL DEFAULT 'draft',
    created_by         CHAR(36)      NULL,

    visible_from       DATETIME      NULL,
    visible_to         DATETIME      NULL,

    created_at         DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP
                                            ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (id),

    CONSTRAINT chk_problem_type   CHECK (type IN ('static', 'container')),
    CONSTRAINT chk_problem_status CHECK (status IN ('draft', 'ready', 'hidden')),

    FOREIGN KEY fk_problems_created_by (created_by)
        REFERENCES users(id) ON DELETE SET NULL
) ENGINE=InnoDB;

CREATE INDEX idx_problems_status   ON problems(status);
CREATE INDEX idx_problems_category ON problems(category);
CREATE INDEX idx_problems_created  ON problems(created_by);

-- ----------------------------------------------------------
-- 5.1 Problem Flags
-- ----------------------------------------------------------

CREATE TABLE problem_flags (
    problem_id      CHAR(36)      NOT NULL,

    flag_hash       VARCHAR(255)  NULL,
    flag_rule       VARCHAR(32)   NOT NULL DEFAULT 'exact',
    flag_example    VARCHAR(255)  NULL,

    flag_type       VARCHAR(16)   NOT NULL DEFAULT 'static',
    flag_handler    VARCHAR(16)   NULL,
    flag_path_key   VARCHAR(128)  NULL,

    PRIMARY KEY (problem_id),

    CONSTRAINT chk_flag_type    CHECK (flag_type IN ('static', 'dynamic')),
    CONSTRAINT chk_flag_handler CHECK (flag_handler IN ('env', 'file')),
    CONSTRAINT chk_flag_rule    CHECK (flag_rule IN ('exact', 'case_insensitive')),
    CONSTRAINT chk_flag_hash_nullable CHECK (
        (flag_type = 'static' AND flag_hash IS NOT NULL)
        OR (flag_type = 'dynamic' AND flag_hash IS NULL)
    ),

    CONSTRAINT fk_pf_problem FOREIGN KEY (problem_id)
        REFERENCES problems(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- ----------------------------------------------------------
-- 5.2 Problem Containers
-- ----------------------------------------------------------

CREATE TABLE problem_containers (
    problem_id        CHAR(36)      NOT NULL,

    image_name        VARCHAR(255)  NOT NULL,
    image_repo        VARCHAR(255)  NULL,
    image_tag         VARCHAR(128)  NULL,

    internal_port     INT           NOT NULL DEFAULT 80,
    access_type       VARCHAR(16)   NOT NULL DEFAULT 'ingress',

    cpu_limit         VARCHAR(32)   NOT NULL DEFAULT '500m',
    memory_limit      VARCHAR(32)   NOT NULL DEFAULT '512Mi',

    env               JSON          NULL,
    command           JSON          NULL,
    args              JSON          NULL,

    deployment_mode   VARCHAR(16)   NOT NULL DEFAULT 'per_user',
    ttl_seconds       INT           NOT NULL DEFAULT 3600,
    namespace_prefix  VARCHAR(64)   NOT NULL DEFAULT 'ctf',

    enable_sidecar    TINYINT(1)    NOT NULL DEFAULT 0,
    sidecar_config    JSON          NULL,

    PRIMARY KEY (problem_id),

    CONSTRAINT chk_access_type     CHECK (access_type IN ('ingress', 'nodeport')),
    CONSTRAINT chk_deployment_mode CHECK (deployment_mode IN ('per_user', 'shared')),

    CONSTRAINT fk_pc_problem FOREIGN KEY (problem_id)
        REFERENCES problems(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- ----------------------------------------------------------
-- 5.3 Problem Tags
-- ----------------------------------------------------------

CREATE TABLE problem_tags (
    problem_id  CHAR(36)    NOT NULL,
    tag         VARCHAR(64) NOT NULL,

    PRIMARY KEY (problem_id, tag),

    CONSTRAINT fk_pt_problem FOREIGN KEY (problem_id)
        REFERENCES problems(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- ==========================================================
-- 6. Submissions
-- ==========================================================

CREATE TABLE submissions (
    id              CHAR(36)     NOT NULL,
    user_id         CHAR(36)     NULL,
    team_id         CHAR(36)     NULL,
    problem_id      CHAR(36)     NOT NULL,
    instance_id     CHAR(36)     NULL,
    contest_id      CHAR(36)     NULL,   -- NULL = practice (non-contest) submission

    flag            VARCHAR(1024) NULL,
    is_correct      TINYINT(1)   NOT NULL DEFAULT 0,
    score_awarded   INT          NOT NULL DEFAULT 0,

    ip_address      VARCHAR(45)  NULL,
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
CREATE INDEX idx_sub_instance      ON submissions(instance_id);
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
    id                  CHAR(36)      NOT NULL,
    user_id             CHAR(36)      NOT NULL,
    problem_id          CHAR(36)      NOT NULL,

    instance_flag_hash  VARCHAR(255)  NULL,

    pod_name            VARCHAR(255)  NULL,
    namespace           VARCHAR(128)  NULL,

    access_url          VARCHAR(512)  NULL,
    node_port           INT           NULL,

    status              VARCHAR(16)   NOT NULL DEFAULT 'pending',
    k8s_status          JSON          NULL,

    expired_at          DATETIME      NULL,

    created_at          DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    destroyed_at        DATETIME      NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uniq_node_port (node_port),

    CONSTRAINT chk_instance_status CHECK (status IN ('pending', 'running', 'stopping', 'destroyed', 'failed', 'crashed')),

    FOREIGN KEY fk_ci_user    (user_id)    REFERENCES users(id)    ON DELETE CASCADE,
    FOREIGN KEY fk_ci_problem (problem_id) REFERENCES problems(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE INDEX idx_ci_user_status         ON challenge_instances(user_id, status);
CREATE INDEX idx_ci_problem_status      ON challenge_instances(problem_id, status);
CREATE INDEX idx_ci_expired             ON challenge_instances(expired_at, status);
CREATE INDEX idx_ci_user_problem_active ON challenge_instances(user_id, problem_id, status);

-- ==========================================================
-- 8. Attachments
-- ==========================================================

CREATE TABLE attachments (
    id              CHAR(36)      NOT NULL,
    problem_id      CHAR(36)      NOT NULL,
    file_name       VARCHAR(255)  NOT NULL,
    storage_path    VARCHAR(1024) NOT NULL,
    cdn_url         VARCHAR(512)  NULL,
    file_type       VARCHAR(50)   NULL,
    file_size       BIGINT        NOT NULL DEFAULT 0,
    sha256          CHAR(64)      NULL,
    is_public       TINYINT(1)    NOT NULL DEFAULT 1,
    created_at      DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),
    UNIQUE KEY uniq_problem_attachment (problem_id),

    FOREIGN KEY fk_attachments_problem (problem_id)
        REFERENCES problems(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- ==========================================================
-- 9. Image Build Tasks
-- ==========================================================

CREATE TABLE image_build_tasks (
    id              CHAR(36)      NOT NULL,
    problem_id      CHAR(36)      NOT NULL,

    file_path       VARCHAR(512)  NOT NULL,
    file_size       BIGINT        NOT NULL DEFAULT 0,

    status          VARCHAR(16)   NOT NULL DEFAULT 'pending',
    current_step    VARCHAR(32)   NOT NULL DEFAULT 'uploading',

    image_repo      VARCHAR(255)  NULL,
    image_tag       VARCHAR(128)  NULL,
    error_log       TEXT          NULL,

    retry_count     INT           NOT NULL DEFAULT 0,
    max_retry       INT           NOT NULL DEFAULT 3,

    worker_id       VARCHAR(64)   NULL,
    claimed_at      DATETIME      NULL,

    started_at      DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at     DATETIME      NULL,

    PRIMARY KEY (id),

    CONSTRAINT chk_build_status CHECK (status IN ('pending', 'running', 'success', 'failed')),

    CONSTRAINT fk_ibt_problem FOREIGN KEY (problem_id)
        REFERENCES problems(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE INDEX idx_ibt_problem ON image_build_tasks(problem_id);
CREATE INDEX idx_ibt_status  ON image_build_tasks(status);

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
    challenge_id   CHAR(36) NOT NULL,
    score_override INT      NULL,   -- NULL = use problems.base_score

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
-- 添加覆盖索引，加速 "某用户最近活跃时间" 查询
CREATE INDEX idx_ual_user_time   ON user_activity_logs(user_id, created_at DESC);

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
-- 17. User Follows (Social)
-- ==========================================================
-- 单向关注模型（follow），非双向好友。
-- follower_id 关注了 followee_id。

CREATE TABLE user_follows (
    id          CHAR(36)    NOT NULL,
    follower_id CHAR(36)    NOT NULL,  -- 关注者
    followee_id CHAR(36)    NOT NULL,  -- 被关注者
    created_at  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),

    -- 不能重复关注
    UNIQUE KEY uniq_follow (follower_id, followee_id),
    -- 不能关注自己
    CONSTRAINT chk_no_self_follow CHECK (follower_id <> followee_id),

    -- 查"某人的粉丝列表"：WHERE followee_id = ? ORDER BY created_at
    INDEX idx_followee (followee_id, created_at),
    -- 查"某人关注了谁"：WHERE follower_id = ? ORDER BY created_at
    INDEX idx_follower (follower_id, created_at),

    CONSTRAINT fk_follow_follower FOREIGN KEY (follower_id)
        REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_follow_followee FOREIGN KEY (followee_id)
        REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- ==========================================================
-- End of script
-- ==========================================================