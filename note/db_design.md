# CTF 平台数据库设计说明

本文档概述为本 CTF 平台设计的数据库表、字段、关系与设计理由。目标是兼顾教学/演示场景的清晰性与工程可扩展性。

## 高层概览

- `users`：用户主表，包含认证信息、展示资料与缓存的比赛统计（`score`、`solved_count`）。
- `teams`、`team_members`：支持团队赛模式与成员管理。
- `challenges`、`challenge_tags`：题目元数据与标签，支持静态与动态（容器）题目。
- `submissions`：提交记录表，作为排行榜与审计的单一数据来源。
- `contests`、`contest_challenges`、`contest_participants`：管理比赛（时间窗口、题目映射、参赛者）。
- `challenge_instances`：跟踪动态题目的运行实例（与 k3s 集成时使用的 pod/job 名称、地址、状态）。
- `user_activity_logs`：记录用户重要行为（登录、提交、创建队伍等），便于审计。
- `attachments`、`docker_images`：题目附件与容器镜像记录。

## 设计要点与实践建议

- 去中心化主键：使用 UUID（`uuid_generate_v4()`）以支持分布式和迁移场景。
- 密码处理：仅存储 `password_hash`，实际后端应使用 `bcrypt` 或 `argon2`。
- 解题/登分一致性：`submissions` 为事实表，`users.score` 与 `teams.score` 为缓存字段，推荐通过应用层事务或异步任务来同步（或使用数据库触发器）。
- 动态题目管理：通过 `challenge_instances` 跟踪 k3s 中的 Pod/Job，包含 `status`、`address` 与 `meta` 字段用于存储端口映射等信息。
- 审计：`user_activity_logs` 记录操作并保存 IP，便于复现与安全分析。
- 可扩展的认证：建议增加 `identities` 表用于外部登录映射（provider + external_id），避免在 `users` 表中放过多外部 auth 字段。

## 初始化与 Demo 数据

项目包含 `db/init.sql`（创建 `users` 表并演示基本索引）以及多个按功能拆分的 SQL 文件（`db/*.sql`），每个文件内包含表定义和本地 demo 插入语句，方便在开发环境快速起库。

运行顺序建议：

1. `psql -f db/init.sql` （创建 uuid 扩展与 users 基础表）
2. 按需执行 `db/teams.sql`、`db/challenges.sql`、`db/submissions.sql`、`db/contests.sql`、`db/challenge_instances.sql`、`db/activity_logs.sql`、`db/attachments.sql`、`db/docker_images.sql` 等。

示例（本地测试）：

```bash
psql -f db/init.sql
psql -f db/teams.sql
psql -f db/challenges.sql
psql -f db/submissions.sql
```

## 后续改进建议（可选）

- 将 demo 插入与迁移脚本分离，使用迁移工具（Flyway / Sqitch / Goose / golang-migrate）管理 schema 变更与回滚。
- 为 `submissions` 建立物化视图或定期 job 以计算排行榜，避免高并发提交时的锁竞争。
- 增加 `ratelimit` 或 `blocklist` 表用于防止刷题或恶意行为。
- 为 GDPR/隐私合规添加 `user_data_exports` 与 `deletions` 审计表。

---

如果你同意此架构，我可以：

- 生成对应的 SQL migration（按时间戳命名）并加入 `migrations/`；
- 用 `docker-compose` 快速搭建 PostgreSQL 并运行这些 SQL 做本地演示；
- 补充 `teams`、`submissions`、`flags` 更详细的示例数据。
