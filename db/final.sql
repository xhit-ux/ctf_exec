-- ==========================================================
-- CTF Platform — Consolidated Initialization Script v3
-- Target:      MySQL 8.0+ / MariaDB 10.6+
-- Encoding:    utf8mb4 / utf8mb4_unicode_ci
-- Updated:     2026-05-07
-- Spec:        note/5-7_题目规范/CTF 题目规范 (v2.1).md
--              note/5-7_题目规范/CTF 题目规范 (v3.0).md
--              note/5-7_题目规范/flag_mode_explain.md
-- 关键变更：
--   * problems.base_score → problems.platform_score (难度区间硬约束)
--   * problem_flags 新增 flag_pattern；flag_rule 扩展 regex/contains；
--     CHECK 约束改为三段式 (chk_flag_fields)；flag_env_key 默认 EXEC_FLAG
--   * attachments(problem_id) 唯一约束 — 一题一附件包
--   * 新增 problem_hints / problem_prerequisites
--   * contest_challenges 拆分为完整赛事计分配置：
--     contest_score / is_dynamic_score / initial_score / min_score / decay_constant
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
DROP TABLE IF EXISTS flag_mappings;
DROP TABLE IF EXISTS problem_flags;
DROP TABLE IF EXISTS docker_images;
DROP TABLE IF EXISTS attachments;
DROP TABLE IF EXISTS challenge_instances;
DROP TABLE IF EXISTS hint_unlocks;
DROP TABLE IF EXISTS problem_hints;
DROP TABLE IF EXISTS problem_prerequisites;
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

    third_user_info       JSON          NULL,

    organization          VARCHAR(100)  NULL,
    country               CHAR(2)       NULL,

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
CREATE INDEX idx_users_leaderboard ON users(score DESC, created_at ASC);


-- ==========================================================
-- 2. Global Leaderboard Snapshot
-- ==========================================================

CREATE TABLE leaderboard_global (
    id            BIGINT        NOT NULL AUTO_INCREMENT,
    board_type    VARCHAR(16)   NOT NULL,

    rank_num      INT           NOT NULL,
    ref_id        CHAR(36)      NOT NULL,

    username      VARCHAR(32)   NULL,
    display_name  VARCHAR(64)   NULL,
    avatar_url    VARCHAR(1024) NULL,
    team_name     VARCHAR(100)  NULL,

    score         INT           NOT NULL DEFAULT 0,
    solved_count  INT           NOT NULL DEFAULT 0,

    total_score   INT           NOT NULL DEFAULT 0,
    avg_score     DECIMAL(10,2) NOT NULL DEFAULT 0,
    member_count  INT           NOT NULL DEFAULT 0,

    snapshot_at   DATETIME      NOT NULL,
    created_at    DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,

    is_current    TINYINT(1)    NOT NULL DEFAULT 1,

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

CREATE TABLE team_members (
    team_id      CHAR(36)    NOT NULL,
    user_id      CHAR(36)    NOT NULL,
    role_in_team VARCHAR(16) NOT NULL DEFAULT 'member',
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

    visibility    VARCHAR(16)   NOT NULL DEFAULT 'public',

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
-- 5. Problems (Challenges) — 平台题库模板
-- ==========================================================
-- v2.1 关键字段：
--   * platform_score  — 平台题库固定分值，受 difficulty 区间硬约束
--                        easy:1-200 / medium:200-500 / hard:500-1000 / expert:1000-2000
--   * 动态计分配置全部下沉到 contest_challenges 表（赛事 scope）
-- ==========================================================

CREATE TABLE problems (
    id                 CHAR(36)      NOT NULL,
    title              VARCHAR(128)  NOT NULL,
    category           VARCHAR(32)   NOT NULL,
    type               VARCHAR(16)   NOT NULL DEFAULT 'static',
    description        TEXT          NULL,
    author             VARCHAR(64)   NULL,

    -- 平台题库固定分值（v2.1 Part 3.1）
    platform_score     INT           NOT NULL DEFAULT 0,

    difficulty         VARCHAR(16)   NOT NULL,
    deploy_mode        VARCHAR(16)   NOT NULL DEFAULT 'shared',
    use_kata           TINYINT(1)    NOT NULL DEFAULT 0,

    status             VARCHAR(16)   NOT NULL DEFAULT 'draft',
    created_by         CHAR(36)      NULL,
    source_problem_id  CHAR(36)      NULL,

    created_at         DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP
                                            ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (id),

    CONSTRAINT chk_problem_type        CHECK (type IN ('static', 'container_web', 'container_tcp')),
    CONSTRAINT chk_problem_deploy_mode CHECK (deploy_mode IN ('shared', 'per_user')),
    CONSTRAINT chk_problem_status      CHECK (status IN ('draft', 'ready', 'hidden')),
    CONSTRAINT chk_problem_category    CHECK (category IN ('web', 'pwn', 'crypto', 'reverse', 'misc')),
    CONSTRAINT chk_problem_difficulty  CHECK (difficulty IN ('easy', 'medium', 'hard', 'expert')),
    CONSTRAINT chk_platform_score_nn   CHECK (platform_score >= 0),

    FOREIGN KEY fk_problems_created_by (created_by)
        REFERENCES users(id) ON DELETE SET NULL,
    FOREIGN KEY fk_problems_source_problem (source_problem_id)
        REFERENCES problems(id) ON DELETE SET NULL
) ENGINE=InnoDB;

CREATE INDEX idx_problems_status   ON problems(status);
CREATE INDEX idx_problems_category ON problems(category);
CREATE INDEX idx_problems_created  ON problems(created_by);

-- ----------------------------------------------------------
-- 5.1 Problem Flags — 三正交维度
-- ----------------------------------------------------------
-- flag_type     : static / dynamic         (正确答案从哪来)
-- flag_rule     : exact / case_insensitive / regex / contains   (怎么比对)
-- flag_inject   : env / sidecar            (动态 flag 怎么进容器)
--
-- 字段一致性约束 (chk_flag_fields, 三段式):
--   * exact / case_insensitive  → flag_hash NOT NULL, flag_pattern NULL
--   * regex / contains          → flag_pattern NOT NULL, flag_hash NULL
--   * dynamic                   → flag_hash NULL, flag_pattern NULL（真值在 flag_mappings）
-- ----------------------------------------------------------

CREATE TABLE problem_flags (
    problem_id            CHAR(36)      NOT NULL,
    flag_type             VARCHAR(16)   NOT NULL DEFAULT 'static',
    flag_rule             VARCHAR(32)   NOT NULL DEFAULT 'exact',
    flag_hash             VARCHAR(255)  NULL,
    flag_pattern          VARCHAR(512)  NULL,
    flag_example          VARCHAR(255)  NULL,
    flag_inject           VARCHAR(16)   NOT NULL DEFAULT 'env',
    flag_env_key          VARCHAR(64)   NOT NULL DEFAULT 'EXEC_FLAG',
    flag_sidecar_pattern  VARCHAR(255)  NULL,

    PRIMARY KEY (problem_id),

    CONSTRAINT chk_flag_type    CHECK (flag_type IN ('static', 'dynamic')),
    CONSTRAINT chk_flag_rule    CHECK (flag_rule IN ('exact', 'case_insensitive', 'regex', 'contains')),
    CONSTRAINT chk_flag_inject  CHECK (flag_inject IN ('env', 'sidecar')),
    CONSTRAINT chk_flag_fields  CHECK (
        (flag_type = 'static'  AND flag_rule IN ('exact', 'case_insensitive') AND flag_hash IS NOT NULL AND flag_pattern IS NULL)
        OR (flag_type = 'static'  AND flag_rule IN ('regex', 'contains')          AND flag_pattern IS NOT NULL AND flag_hash IS NULL)
        OR (flag_type = 'dynamic' AND flag_hash IS NULL AND flag_pattern IS NULL)
    ),

    CONSTRAINT fk_pf_problem FOREIGN KEY (problem_id)
        REFERENCES problems(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- ----------------------------------------------------------
-- 5.2 Dynamic Flag Mappings — 每用户/每实例独立 flag
-- ----------------------------------------------------------

CREATE TABLE flag_mappings (
    id          CHAR(36)      NOT NULL,
    user_id     CHAR(36)      NOT NULL,
    problem_id  CHAR(36)      NOT NULL,
    flag_value  VARCHAR(255)  NOT NULL,
    created_at  DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),
    UNIQUE KEY uniq_fm_user_problem (user_id, problem_id),

    FOREIGN KEY fk_fm_user    (user_id)    REFERENCES users(id)    ON DELETE CASCADE,
    FOREIGN KEY fk_fm_problem (problem_id) REFERENCES problems(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- ----------------------------------------------------------
-- 5.3 Problem Containers
-- ----------------------------------------------------------

CREATE TABLE problem_containers (
    problem_id        CHAR(36)      NOT NULL,
    image_name        VARCHAR(255)  NOT NULL,
    image_tag         VARCHAR(128)  NOT NULL DEFAULT 'latest',
    challenge_port    INT           NOT NULL,
    sidecar_image     VARCHAR(255)  NULL,
    sidecar_port      INT           NULL,
    cpu_limit         VARCHAR(32)   NOT NULL DEFAULT '500m',
    memory_limit      VARCHAR(32)   NOT NULL DEFAULT '512Mi',
    env               JSON          NULL,
    ttl_seconds       INT           NOT NULL DEFAULT 3600,
    ingress_host_tpl  VARCHAR(255)  NULL,

    PRIMARY KEY (problem_id),

    CONSTRAINT fk_pc_problem FOREIGN KEY (problem_id)
        REFERENCES problems(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- ----------------------------------------------------------
-- 5.4 Problem Tags
-- ----------------------------------------------------------

CREATE TABLE problem_tags (
    problem_id  CHAR(36)    NOT NULL,
    tag         VARCHAR(64) NOT NULL,

    PRIMARY KEY (problem_id, tag),

    CONSTRAINT fk_pt_problem FOREIGN KEY (problem_id)
        REFERENCES problems(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- ----------------------------------------------------------
-- 5.5 Problem Hints — v2.1 Part 7
-- ----------------------------------------------------------

CREATE TABLE problem_hints (
    id           CHAR(36)      NOT NULL,
    problem_id   CHAR(36)      NOT NULL,
    content      TEXT          NOT NULL,
    cost         INT           NOT NULL DEFAULT 0,
    sort_order   INT           NOT NULL DEFAULT 0,
    is_visible   TINYINT(1)    NOT NULL DEFAULT 1,
    created_at   DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),

    CONSTRAINT chk_hint_cost_nn CHECK (cost >= 0),

    FOREIGN KEY fk_hints_problem (problem_id)
        REFERENCES problems(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE INDEX idx_hints_problem_order ON problem_hints(problem_id, sort_order);

-- ----------------------------------------------------------
-- 5.6 Hint Unlocks — 用户提示解锁记录（按 scope 扣分）
-- ----------------------------------------------------------

CREATE TABLE hint_unlocks (
    id              CHAR(36)    NOT NULL,
    user_id         CHAR(36)    NOT NULL,
    hint_id         CHAR(36)    NOT NULL,
    contest_id      CHAR(36)    NULL,         -- NULL = platform scope
    cost_at_unlock  INT         NOT NULL DEFAULT 0,
    unlocked_at     DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),

    UNIQUE KEY uniq_hint_unlock_scope (user_id, hint_id, contest_id),

    FOREIGN KEY fk_hu_user    (user_id) REFERENCES users(id)         ON DELETE CASCADE,
    FOREIGN KEY fk_hu_hint    (hint_id) REFERENCES problem_hints(id) ON DELETE CASCADE,
    FOREIGN KEY fk_hu_contest (contest_id) REFERENCES contests(id)   ON DELETE CASCADE
) ENGINE=InnoDB;

-- ----------------------------------------------------------
-- 5.7 Problem Prerequisites — v2.1 Part 8
-- ----------------------------------------------------------

CREATE TABLE problem_prerequisites (
    problem_id       CHAR(36) NOT NULL,
    prerequisite_id  CHAR(36) NOT NULL,

    PRIMARY KEY (problem_id, prerequisite_id),

    CONSTRAINT chk_no_self_prereq CHECK (problem_id <> prerequisite_id),

    FOREIGN KEY fk_pp_problem      (problem_id)      REFERENCES problems(id) ON DELETE CASCADE,
    FOREIGN KEY fk_pp_prerequisite (prerequisite_id) REFERENCES problems(id) ON DELETE CASCADE
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
    contest_id      CHAR(36)     NULL,

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

CREATE INDEX idx_sub_user          ON submissions(user_id);
CREATE INDEX idx_sub_problem       ON submissions(problem_id);
CREATE INDEX idx_sub_instance      ON submissions(instance_id);
CREATE INDEX idx_sub_user_problem  ON submissions(user_id, problem_id);
CREATE INDEX idx_sub_problem_correct ON submissions(problem_id, is_correct);
CREATE INDEX idx_sub_user_time     ON submissions(user_id, created_at);
CREATE INDEX idx_sub_contest_user_problem_correct
    ON submissions(contest_id, user_id, problem_id, is_correct);
CREATE INDEX idx_sub_user_problem_noc ON submissions(user_id, problem_id, is_correct, contest_id);


-- ==========================================================
-- 6.1 Refresh Tokens (Persistent, DB-backed)
-- ==========================================================

CREATE TABLE refresh_tokens (
    id          CHAR(36)    NOT NULL,
    user_id     CHAR(36)    NOT NULL,
    token_hash  CHAR(64)    NOT NULL,
    jti         CHAR(36)    NOT NULL,
    expires_at  DATETIME    NOT NULL,
    revoked     TINYINT(1)  NOT NULL DEFAULT 0,
    ip_address  VARCHAR(45) NULL,
    user_agent  VARCHAR(512) NULL,
    created_at  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),

    UNIQUE KEY uniq_rt_hash (token_hash),

    INDEX idx_rt_user_active (user_id, revoked, expires_at),

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
    pod_name            VARCHAR(255)  NULL,
    namespace           VARCHAR(128)  NULL,
    access_url          VARCHAR(512)  NULL,
    node_port           INT           NULL,
    status              VARCHAR(16)   NOT NULL DEFAULT 'pending',
    flag_value          VARCHAR(255)  NULL,
    started_at          DATETIME      NULL,
    expired_at          DATETIME      NULL,
    destroyed_at        DATETIME      NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uniq_active_instance (user_id, problem_id),

    CONSTRAINT chk_instance_status CHECK (status IN ('pending', 'running', 'stopping', 'destroyed', 'failed')),

    FOREIGN KEY fk_ci_user    (user_id)    REFERENCES users(id)    ON DELETE CASCADE,
    FOREIGN KEY fk_ci_problem (problem_id) REFERENCES problems(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE INDEX idx_ci_user_status         ON challenge_instances(user_id, status);
CREATE INDEX idx_ci_problem_status      ON challenge_instances(problem_id, status);
CREATE INDEX idx_ci_expired             ON challenge_instances(expired_at, status);
CREATE INDEX idx_ci_user_problem_active ON challenge_instances(user_id, problem_id, status);

-- ==========================================================
-- 8. Attachments — 单附件包约束（v2.1 Part 6）
-- ==========================================================

CREATE TABLE attachments (
    id              CHAR(36)      NOT NULL,
    problem_id      CHAR(36)      NOT NULL,
    file_name       VARCHAR(255)  NOT NULL,
    bucket_name     VARCHAR(128)  NOT NULL,
    object_key      VARCHAR(512)  NOT NULL,
    file_size       BIGINT        NOT NULL DEFAULT 0,
    sha256          CHAR(64)      NULL,
    created_at      DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),

    -- DB 层强制：一题一附件包
    UNIQUE KEY uniq_attachment_problem (problem_id),

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

CREATE TABLE contest_admins (
    id          CHAR(36)    NOT NULL,
    contest_id  CHAR(36)    NOT NULL,
    user_id     CHAR(36)    NOT NULL,

    role        VARCHAR(16) NOT NULL DEFAULT 'admin',

    granted_by  CHAR(36)    NULL,
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
-- 11. Contest Challenges — 赛事题目实例（v2.1 Part 1.1 / Part 3.2）
-- ==========================================================
-- 双 Scope 计分核心：每条 contest_challenges 行携带完整的赛事计分配置。
--   * contest_score      — 赛事基础分值（自由设置）
--   * is_dynamic_score   — 是否启用 CTFd 多项式衰减
--   * initial_score / min_score / decay_constant — 动态参数（is_dynamic_score=1 时必填）
-- current_score 由 service 层根据 solve_count 实时计算，不入库。
-- ==========================================================

CREATE TABLE contest_challenges (
    contest_id        CHAR(36)   NOT NULL,
    challenge_id      CHAR(36)   NOT NULL,

    contest_score     INT        NOT NULL DEFAULT 500,
    is_dynamic_score  TINYINT(1) NOT NULL DEFAULT 0,
    initial_score     INT        NULL,
    min_score         INT        NULL,
    decay_constant    INT        NULL,

    sort_order        INT        NOT NULL DEFAULT 0,
    created_at        DATETIME   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME   NOT NULL DEFAULT CURRENT_TIMESTAMP
                                     ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (contest_id, challenge_id),

    CONSTRAINT chk_contest_score_pos CHECK (contest_score > 0),
    CONSTRAINT chk_dynamic_params CHECK (
        (is_dynamic_score = 0)
        OR (
            is_dynamic_score = 1
            AND initial_score  IS NOT NULL AND initial_score  > 0
            AND min_score      IS NOT NULL AND min_score      > 0
            AND decay_constant IS NOT NULL AND decay_constant > 0
            AND min_score < initial_score
        )
    ),

    FOREIGN KEY fk_cc_contest   (contest_id)   REFERENCES contests(id)  ON DELETE CASCADE,
    FOREIGN KEY fk_cc_challenge (challenge_id) REFERENCES problems(id)  ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE INDEX idx_cc_contest_sort ON contest_challenges(contest_id, sort_order);

-- ==========================================================
-- 12. Contest Participants
-- ==========================================================

CREATE TABLE contest_participants (
    id            CHAR(36)  NOT NULL,
    contest_id    CHAR(36)  NOT NULL,
    user_id       CHAR(36)  NULL,
    team_id       CHAR(36)  NULL,

    status        VARCHAR(16) NOT NULL DEFAULT 'approved',

    registered_at DATETIME  NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),

    CONSTRAINT chk_cp_status CHECK (status IN ('pending', 'approved', 'rejected')),

    FOREIGN KEY fk_cp_contest (contest_id) REFERENCES contests(id) ON DELETE CASCADE,
    FOREIGN KEY fk_cp_user    (user_id)    REFERENCES users(id)    ON DELETE CASCADE,
    FOREIGN KEY fk_cp_team    (team_id)    REFERENCES teams(id)    ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE UNIQUE INDEX uniq_cp_contest_user ON contest_participants(contest_id, user_id);
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
CREATE INDEX idx_ual_user_time   ON user_activity_logs(user_id, created_at DESC);

-- ==========================================================
-- 16. Role Change Logs (Structured Audit)
-- ==========================================================

CREATE TABLE role_change_logs (
    id          CHAR(36)     NOT NULL,
    operator_id CHAR(36)     NULL,
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

CREATE TABLE user_follows (
    id          CHAR(36)    NOT NULL,
    follower_id CHAR(36)    NOT NULL,
    followee_id CHAR(36)    NOT NULL,
    created_at  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),

    UNIQUE KEY uniq_follow (follower_id, followee_id),
    CONSTRAINT chk_no_self_follow CHECK (follower_id <> followee_id),

    INDEX idx_followee (followee_id, created_at),
    INDEX idx_follower (follower_id, created_at),

    CONSTRAINT fk_follow_follower FOREIGN KEY (follower_id)
        REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_follow_followee FOREIGN KEY (followee_id)
        REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- ==========================================================
-- End of script
-- ==========================================================
