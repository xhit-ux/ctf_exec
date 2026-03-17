# 比赛/赛事功能模块设计实现建议（不含管理端增删改）

**文档更新时间**: 2026-03-17  
**适用范围**: CTF 练习平台后端（Gin + Repository/Service 分层）  
**明确排除**: 管理端赛事创建/编辑/删除、题目编排管理接口

---

## 1. 目标与边界

本设计聚焦“参赛侧”赛事能力，确保用户可以完整经历：

1. 查看赛事列表与详情
2. 报名/退赛（个人或团队）
3. 在赛事时间窗口内查看赛事题目并提交 flag
4. 查看赛事个人榜/团队榜

本阶段不做：

1. 赛事增删改（管理端）
2. 赛事题目编排管理（管理端）
3. 复杂计分模式（仅先支持 `jeopardy`）

---

## 2. 项目现状盘点（基于当前代码与 SQL）

### 2.1 已有能力

1. 数据库已具备赛事核心表：`contests`、`contest_challenges`、`contest_participants`、`contest_scores`
2. 已有公开查询接口：
`GET /api/contests`
`GET /api/contests/:id`
3. 已有排行榜服务中“赛事维度 Redis key 与读写方法”定义：
`UpdateContestUserScore` / `GetContestUserTopN` / `GetContestTeamTopN` 等
4. 题目、提交、团队、全局榜链路可用

### 2.2 当前缺口

1. 无参赛接口（join/leave）
2. 无赛事题目列表接口（仅返回当前赛事可见题）
3. 提交流程未绑定赛事上下文（`submissions` 尚未写 `contest_id`）
4. 无赛事榜查询接口（目前只有 service 层能力，无 handler 暴露）
5. 无赛事时间窗口与参赛资格校验中间层

---

## 3. 数据结构设计解读与使用策略

### 3.1 `contests`

作用：赛事元数据与时间窗口控制。

关键字段：

1. `id`, `title`, `description`, `cover_image`
2. `start_time`, `end_time`
3. `scoring_type`（本阶段固定按 `jeopardy` 解释）

### 3.2 `contest_challenges`

作用：定义赛事包含哪些题，以及是否覆盖原分值。

关键字段：

1. `(contest_id, challenge_id)` 复合主键
2. `score_override`（允许赛事分值覆盖题库原始分值）

### 3.3 `contest_participants`

作用：报名记录。

关键字段：

1. 主键 `id`
2. `contest_id`
3. `user_id` 或 `team_id`（支持个人赛/团队赛）
4. 唯一索引：
`uniq_contest_user(contest_id, user_id)`
`uniq_contest_team(contest_id, team_id)`

约束建议（服务层保证）：

1. 单次报名必须且仅能指定 `user_id` 或 `team_id` 之一
2. 若走团队赛，用户必须是该团队成员

### 3.4 `contest_scores`

作用：赛事榜持久化快照（权威分数来源），Redis 为加速缓存。

关键字段：

1. `contest_id`, `user_id` / `team_id`
2. `score`, `solved_count`, `last_solve_time`
3. 唯一索引支持 upsert

建议：

1. 每次正确提交先更新 `contest_scores`，再异步/同步写 Redis ZSet
2. Redis 丢失时可按 `contest_scores` 重建

### 3.5 `submissions`

现状：表中有 `contest_id`，但当前提交仓储写入时未使用。

建议：

1. 增加提交请求中的可选 `contest_id`
2. 当有 `contest_id` 时，强制走赛事校验链路（时间、报名、题目归属）

---

## 4. 参赛侧业务流程设计

### 4.1 赛事状态机（服务层计算）

按当前时间与 `start_time/end_time` 计算：

1. `upcoming`: now < start_time
2. `running`: start_time <= now <= end_time
3. `ended`: now > end_time

用于：

1. 报名是否允许（通常 upcoming/running 可报名，按策略）
2. 提交是否允许（仅 running）
3. 赛事题目可见策略

### 4.2 报名流程（join）

1. 认证用户调用 `POST /api/contests/:id/join`
2. 服务层检查赛事存在、状态允许
3. 个人赛：写 `contest_participants(user_id)`
4. 团队赛：校验调用者属于目标团队，再写 `contest_participants(team_id)`
5. 初始化 `contest_scores` 记录（score=0）

### 4.3 退赛流程（leave）

1. 认证用户调用 `POST /api/contests/:id/leave`
2. 删除 `contest_participants` 对应行
3. 退赛后分数策略二选一：
- 保留历史分数（推荐，便于审计）
- 或同步删除 `contest_scores`（不推荐）

### 4.4 赛事题目查看

1. `GET /api/contests/:id/challenges`
2. 仅返回该赛事在 `contest_challenges` 绑定的题
3. 可附带当前用户在该赛事内的解题状态（is_solved）

### 4.5 赛事提交与计分

1. 在现有 `SubmitFlag` 链路新增 `contest_id` 分支
2. 校验：
- 赛事存在且 running
- 用户/团队已报名
- `problem_id` 属于该赛事
- 未在该赛事重复正确提交同题
3. 计分：
- 基础分取 `score_override`，无覆盖则取 `problems.points`
- 更新 `contest_scores`（score、solved_count、last_solve_time）
4. 写入 `submissions.contest_id`
5. 更新 Redis 赛事榜（个人或团队）

---

## 5. API 设计（本阶段建议）

### 5.1 公开接口

1. `GET /api/contests?limit=20`
2. `GET /api/contests/:id`
3. `GET /api/contests/:id/scoreboard?type=user|team&limit=50`

### 5.2 认证接口（参赛侧）

1. `POST /api/contests/:id/join`
请求示例：
```json
{
  "team_id": "optional-team-id"
}
```
2. `POST /api/contests/:id/leave`
请求示例：
```json
{
  "team_id": "optional-team-id"
}
```
3. `GET /api/contests/:id/challenges`
4. `POST /api/submissions`
请求体扩展建议：
```json
{
  "problem_id": 1,
  "flag": "flag{...}",
  "contest_id": "optional-contest-id"
}
```

---

## 6. 后端实现方案（按现有分层）

### 6.1 Models

新增/调整：

1. `models/contest.go`
- `JoinContestRequest`
- `LeaveContestRequest`
- `ContestScoreboardEntry`
- `ContestChallengeView`

2. `models/submission.go`
- `CreateSubmissionRequest` 增加 `ContestID *string`

### 6.2 Repository

扩展 `repository/contest.go`：

1. `JoinContest(contestID, userID, teamID)`
2. `LeaveContest(contestID, userID, teamID)`
3. `IsParticipant(contestID, userID, teamID)`
4. `ListContestChallenges(contestID)`
5. `GetContestChallengeScore(contestID, problemID)`
6. `UpsertContestUserScore(...)` / `UpsertContestTeamScore(...)`
7. `GetContestScoreboardFromDB(contestID, type, limit)`（Redis miss 回源）

扩展 `repository/submission.go`：

1. 创建提交时支持写 `contest_id`
2. 提供“某赛事中某主体是否已正确解过该题”查询

### 6.3 Service

新增 `service/contest_service.go`（建议）：

1. `GetContestStatus(contest)`
2. `JoinContest(...)`
3. `LeaveContest(...)`
4. `GetContestChallengesForUser(...)`
5. `GetContestScoreboard(...)`
6. `ValidateContestSubmission(...)`

并在 `submission_service.go` 增加：

1. 分支处理 `contest_id` 提交
2. 赛事计分更新 + Redis 更新

### 6.4 Handler

扩展 `handler/contest.go`：

1. `JoinContestGin`
2. `LeaveContestGin`
3. `GetContestChallengesGin`
4. `GetContestScoreboardGin`

### 6.5 路由（不含管理端）

在 `main.go` 中赛事组补充：

1. 公开：`GET /api/contests/:id/scoreboard`
2. 认证：
`POST /api/contests/:id/join`
`POST /api/contests/:id/leave`
`GET /api/contests/:id/challenges`

---

## 7. 关键校验与异常策略

1. 时间校验：非 `running` 阶段拒绝赛事提交
2. 资格校验：未报名拒绝查看赛事题目/提交
3. 归属校验：题目不在赛事中时拒绝提交
4. 幂等处理：重复报名返回已报名，重复正确提交同题返回冲突
5. 团队一致性：团队赛提交时，校验当前用户仍属于该团队

建议错误码（可后续统一）：

1. `CONTEST_NOT_FOUND`
2. `CONTEST_NOT_RUNNING`
3. `CONTEST_NOT_PARTICIPANT`
4. `CONTEST_PROBLEM_NOT_ALLOWED`
5. `CONTEST_ALREADY_SOLVED`

---

## 8. 性能与一致性

1. 榜单读路径：Redis 优先，DB 回源
2. 榜单写路径：先 DB 后 Redis
3. 定时任务：按 `contest_scores` 重建 Redis 赛事榜（防缓存丢失）
4. 索引建议（可在迁移中补充）：
- `submissions(contest_id, user_id, problem_id, is_correct)`
- `contest_scores(contest_id, score DESC, last_solve_time ASC)`

---

## 9. 安全与审计建议

1. 报名/退赛记录可追加 `user_activity_logs`（`contest_join`/`contest_leave`）
2. 赛事提交保留 `ip_address` 与时间戳
3. 保持角色边界：参赛接口只要求登录，不依赖 admin 权限

---

## 10. 分阶段落地计划（建议）

### Phase 1（最小闭环）

1. join/leave
2. 赛事题目列表
3. 提交支持 `contest_id` + 基础校验
4. 写 `contest_scores`

### Phase 2（体验增强）

1. 赛事榜接口（user/team）
2. Redis 与 DB 双写 + 回源
3. 赛事状态字段在列表/详情中显式返回

### Phase 3（稳定性增强）

1. 审计日志补齐
2. 幂等与错误码统一
3. 集成测试（报名->提交->榜单）

---

## 11. 验收标准（参赛侧）

1. 用户可成功报名/退赛，且具备幂等行为
2. 非参赛用户无法访问赛事题目与赛事提交
3. 赛事外题目无法在该赛事计分
4. 正确提交后 `contest_scores` 与赛事榜一致更新
5. 比赛结束后拒绝赛事提交
6. Redis 清空后仍可从 DB 恢复赛事榜数据

---

## 12. 结论

结合当前项目状态，赛事模块下一步应优先完成“参赛闭环”而非继续扩展管理端。以现有数据库结构为基础，不需要大规模改表即可落地：核心在于把 `contest_participants`、`contest_challenges`、`contest_scores` 与 `submissions(contest_id)` 在服务层真正串起来，并以 Redis 作为赛事榜加速层。
