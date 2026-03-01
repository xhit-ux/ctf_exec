# 用户表（users）字段设计

该文档提供 CTF 平台中用户（`users`）表的建议字段、类型与说明，基于 PostgreSQL。设计兼顾安全、查询效率与可扩展性。

## 建议字段

- `id` : `uuid` — 主键，使用 UUID v4。示例：`uuid_generate_v4()`。
- `username` : `varchar(32)` — 用户名，唯一，索引。仅允许字母数字与下划线，便于展示与登录（可同时支持邮箱登录）。
- `email` : `varchar(254)` — 邮箱，唯一，可为空（若允许匿名或仅用户名）。建立唯一索引并使用大小写不敏感比较（citext 或 lower() 索引）。
- `password_hash` : `varchar(128)` — 密码哈希（例如 bcrypt/argon2 输出）。不要存明文密码。
- `role` : `varchar(16)` — 角色，例如 `user`、`admin`、`staff`。可用枚举类型约束。
- `display_name` : `varchar(64)` — 显示名称，可与 `username` 不同。
- `team_id` : `uuid` — 外键，指向 `teams` 表（若支持队伍），可为空。
- `score` : `integer` — 用户总分，默认 0。注意通过事务/触发器或应用层更新以保证一致性。
- `solved_count` : `integer` — 已通过题目数，默认 0。
- `is_active` : `boolean` — 帐号是否激活/可用，默认 `true`。
- `created_at` : `timestamptz` — 创建时间，默认 `now()`。
- `updated_at` : `timestamptz` — 最后更新（可用触发器或应用层维护）。
- `last_login_at` : `timestamptz` — 最后登录时间，可为空。
- `avatar_url` : `varchar(512)` — 头像链接，可为空。
- `bio` : `text` — 用户简介，可为空。
- `preferences` : `jsonb` — 可选的用户设置（如主题、通知偏好），便于扩展。
- `twofa_enabled` : `boolean` — 是否启用二次验证（TOTP 等），默认 `false`。
- `password_reset_token` : `varchar(128)` — 重置令牌（有时用单独表更安全），可为空。
- `password_reset_expires` : `timestamptz` — 重置令牌过期时间，可为空。

## 索引与约束建议

- 主键：`PRIMARY KEY (id)`。
- 唯一索引：`UNIQUE (username)`，`UNIQUE (email)`（若允许为空，则根据业务决定是否唯一约束）。
- 普通索引：`INDEX (team_id)`、`INDEX (score)`（按排行榜排序时有用）。
- 使用 `CHECK` 或应用层校验 `username` 格式与长度。

## 示例 PostgreSQL 建表语句

```sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE users (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  username varchar(32) NOT NULL UNIQUE,
  email varchar(254) UNIQUE,
  password_hash varchar(128) NOT NULL,
  role varchar(16) NOT NULL DEFAULT 'user',
  display_name varchar(64),
  team_id uuid,
  score integer NOT NULL DEFAULT 0,
  solved_count integer NOT NULL DEFAULT 0,
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  last_login_at timestamptz,
  avatar_url varchar(512),
  bio text,
  preferences jsonb DEFAULT '{}'::jsonb,
  twofa_enabled boolean NOT NULL DEFAULT false,
  password_reset_token varchar(128),
  password_reset_expires timestamptz,
  CONSTRAINT username_check CHECK (username ~ '^[A-Za-z0-9_]{3,32}$')
);

CREATE INDEX idx_users_team ON users(team_id);
CREATE INDEX idx_users_score ON users(score DESC);
```

## 实践建议

- 密码存储：在后端使用成熟库（bcrypt/argon2）处理，不在数据库中存明文或自实现哈希。不要在 `preferences` 中放敏感信息。
- 登分一致性：推荐使用单独的 `submissions` 表记录解题记录，再通过应用层或数据库事务更新 `users.score` 与 `users.solved_count`，或通过定期批处理/物化视图计算排行榜以避免并发问题。
- 隐私与合规：若平台公开排名或邮箱，确认隐私政策并提供用户资料导出/删除流程。
- 扩展性：如需多种认证（OAuth、LDAP），建议增加 `auth_providers` 或 `identities` 表存放外部登录信息。

---

如果你希望我把 `teams`、`submissions`、`flags` 表也一并设计，我可以继续补充并把这些表的 SQL 也写进同一文件。
