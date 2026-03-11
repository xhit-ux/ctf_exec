# CTF 平台赛事功能开发步骤

**文档创建时间**: 2026-03-11  
**项目名称**: CTF Exec Platform  
**版本**: v0.4.0  
**关联进度文档**: [progress_02.md](./progress_02.md)

---

## 📋 任务概述

基于当前项目进度，完成赛事功能的全面开发。当前项目已具备基础架构（用户、团队、题目、提交系统），赛事功能已有数据库设计和部分基础设施，但缺少完整的业务逻辑实现。

### 当前赛事功能状态
- ✅ **数据库设计完成**: `contests`, `contest_challenges`, `contest_participants` 表已定义
- ✅ **Redis 缓存支持**: 赛事排行榜键模板和基础服务接口已定义
- ✅ **上传功能**: 比赛封面上传已实现 (`UploadContestCoverGin`)
- ⚠️ **模型层缺失**: 缺少 Contest 相关模型定义
- ❌ **数据访问层缺失**: 缺少 Repository 实现
- ❌ **业务逻辑层缺失**: 缺少 Service 实现  
- ❌ **API 接口缺失**: 缺少 Contest 相关 API 路由和处理函数
- ❌ **前端界面缺失**: 缺少 Contest 相关页面

---

## 🎯 开发目标

### 阶段一：基础 API 实现
1. **模型定义** (`internal/models/contest.go`)
   - Contest 基础模型
   - ContestChallenge 关联模型
   - ContestParticipant 参与模型
   - 请求/响应结构体

2. **数据访问层** (`internal/repository/contest.go`)
   - Contest CRUD 操作
   - 题目关联管理
   - 参赛者管理

3. **业务逻辑层** (`internal/service/contest_service.go`)
   - 赛事创建、编辑、删除
   - 题目添加到赛事
   - 用户/团队报名参赛
   - 赛事状态管理（进行中、即将开始、已结束）

4. **API 接口层** (`internal/handler/contest.go`)
   - 赛事列表/详情获取
   - 赛事管理（创建、更新、删除）
   - 参赛管理（报名、退赛）
   - 赛事题目管理

### 阶段二：赛事核心功能
1. **赛事排行榜集成**
   - 使用现有 `LeaderboardService` 实现赛事排行榜
   - 分数同步机制
   - 实时排名更新

2. **赛事时间管理**
   - 开始/结束时间控制
   - 自动状态切换
   - 时间相关限制（报名截止、提交限制）

3. **权限与访问控制**
   - 赛事管理员权限
   - 参赛者权限
   - 公开/私有赛事控制

### 阶段三：前端界面开发
1. **赛事列表页面**
2. **赛事详情页面**
3. **赛事管理后台**
4. **赛事排行榜页面**

---

## 🔧 详细开发步骤

### 步骤 1: 创建 Contest 模型定义

**文件**: `backend/internal/models/contest.go`

**内容要点**:
- `Contest` 结构体（对应数据库表）
- `ContestChallenge` 结构体（题目关联）
- `ContestParticipant` 结构体（参赛者）
- 请求/响应结构体：
  - `CreateContestRequest`
  - `UpdateContestRequest`
  - `JoinContestRequest`
  - `ContestResponse`
  - `ContestListResponse`

**注意事项**:
- 使用 `time.Time` 类型处理开始/结束时间
- 包含封面图路径字段
- 支持 `scoring_type`（计分类型：jeopardy、attack-defense 等）
- 包含状态计算字段（`Status()` 方法）

### 步骤 2: 实现 Contest Repository

**文件**: `backend/internal/repository/contest.go`

**接口定义**:
```go
type ContestRepository interface {
    // 赛事基础操作
    Create(ctx context.Context, contest *models.Contest) error
    GetByID(ctx context.Context, id string) (*models.Contest, error)
    GetAll(ctx context.Context, limit, offset int) ([]*models.Contest, error)
    Update(ctx context.Context, contest *models.Contest) error
    Delete(ctx context.Context, id string) error
    
    // 赛事题目关联
    AddChallenge(ctx context.Context, contestID string, challengeID int, scoreOverride *int) error
    RemoveChallenge(ctx context.Context, contestID string, challengeID int) error
    GetContestChallenges(ctx context.Context, contestID string) ([]*models.ContestChallenge, error)
    
    // 参赛者管理
    AddParticipant(ctx context.Context, contestID, userID, teamID string) error
    RemoveParticipant(ctx context.Context, contestID, userID, teamID string) error
    GetContestParticipants(ctx context.Context, contestID string) ([]*models.ContestParticipant, error)
    IsUserParticipant(ctx context.Context, contestID, userID string) (bool, error)
    
    // 查询相关
    GetActiveContests(ctx context.Context) ([]*models.Contest, error)
    GetUpcomingContests(ctx context.Context) ([]*models.Contest, error)
    GetPastContests(ctx context.Context) ([]*models.Contest, error)
}
```

**实现要点**:
- 使用 `database/sql` 执行 SQL 查询
- 正确处理 NULL 值（团队ID可能为NULL）
- 包含错误处理和日志记录
- 支持分页查询

### 步骤 3: 实现 Contest Service

**文件**: `backend/internal/service/contest_service.go`

**接口定义**:
```go
type ContestService interface {
    // 赛事管理
    CreateContest(ctx context.Context, req *models.CreateContestRequest) (*models.Contest, error)
    GetContest(ctx context.Context, id string) (*models.Contest, error)
    ListContests(ctx context.Context, status string, limit, offset int) ([]*models.Contest, error)
    UpdateContest(ctx context.Context, id string, req *models.UpdateContestRequest) (*models.Contest, error)
    DeleteContest(ctx context.Context, id string) error
    
    // 赛事题目管理
    AddChallengeToContest(ctx context.Context, contestID string, challengeID int, scoreOverride *int) error
    RemoveChallengeFromContest(ctx context.Context, contestID string, challengeID int) error
    GetContestChallenges(ctx context.Context, contestID string) ([]*models.ContestChallenge, error)
    
    // 参赛管理
    JoinContest(ctx context.Context, contestID, userID, teamID string) error
    LeaveContest(ctx context.Context, contestID, userID, teamID string) error
    GetContestParticipants(ctx context.Context, contestID string) ([]*models.ContestParticipant, error)
    
    // 业务逻辑
    GetContestStatus(ctx context.Context, contestID string) (string, error)
    GetUserContests(ctx context.Context, userID string) ([]*models.Contest, error)
    ValidateContestAccess(ctx context.Context, contestID, userID string) (bool, error)
}
```

**实现要点**:
- 业务逻辑验证（时间冲突、权限检查等）
- 与 `LeaderboardService` 集成（报名时初始化排行榜）
- 状态管理逻辑（自动判断进行中/即将开始/已结束）
- 事务处理（报名、退赛等操作）

### 步骤 4: 实现 Contest Handler

**文件**: `backend/internal/handler/contest.go`

**路由设计**:
```
GET    /api/contests                     - 获取赛事列表
POST   /api/contests                     - 创建新赛事
GET    /api/contests/:id                 - 获取赛事详情
PUT    /api/contests/:id                 - 更新赛事
DELETE /api/contests/:id                 - 删除赛事

POST   /api/contests/:id/challenges      - 添加题目到赛事
DELETE /api/contests/:id/challenges/:challenge_id - 从赛事移除题目

POST   /api/contests/:id/join            - 报名参加赛事
POST   /api/contests/:id/leave           - 退出赛事
GET    /api/contests/:id/participants    - 获取参赛者列表

GET    /api/contests/:id/scoreboard      - 获取赛事排行榜
GET    /api/contests/:id/challenges      - 获取赛事题目列表
```

**实现要点**:
- Gin 框架路由处理
- JWT 认证中间件集成
- 权限验证（管理员权限等）
- 参数验证和错误处理
- 统一的响应格式

### 步骤 5: 注册路由和依赖注入

**文件**: `backend/main.go`

**需要添加**:
1. 初始化 ContestRepository 和 ContestService
2. 创建 ContestHandler
3. 注册 Contest 相关路由
4. 集成到现有路由组中

### 步骤 6: 创建赛事相关前端页面

**页面清单**:
1. `frontend/src/pages/Contests.tsx` - 赛事列表页面
2. `frontend/src/pages/ContestDetail.tsx` - 赛事详情页面
3. `frontend/src/pages/ContestScoreboard.tsx` - 赛事排行榜页面
4. `frontend/src/pages/AdminContest.tsx` - 赛事管理页面

**组件清单**:
1. `frontend/src/components/ContestCard.tsx` - 赛事卡片组件
2. `frontend/src/components/ContestChallengeList.tsx` - 赛事题目列表组件
3. `frontend/src/components/ContestTimer.tsx` - 赛事倒计时组件

### 步骤 7: API 客户端集成

**文件**: 创建统一的 API 客户端或扩展现有功能
- 添加 Contest 相关的 API 调用函数
- 集成到现有 API 模块中

### 步骤 8: 测试和验证

1. **单元测试**:
   - Service 层单元测试
   - Repository 层单元测试

2. **集成测试**:
   - API 接口测试
   - 数据库操作测试

3. **端到端测试**:
   - 完整的赛事创建、报名、解题流程
   - 排行榜更新测试

---

## 📁 文件结构变化

### 新增文件
```
backend/
├── internal/
│   ├── models/
│   │   └── contest.go
│   ├── repository/
│   │   └── contest.go
│   ├── service/
│   │   └── contest_service.go
│   └── handler/
│       └── contest.go
frontend/
├── src/
│   ├── pages/
│   │   ├── Contests.tsx
│   │   ├── ContestDetail.tsx
│   │   ├── ContestScoreboard.tsx
│   │   └── AdminContest.tsx
│   └── components/
│       ├── ContestCard.tsx
│       ├── ContestChallengeList.tsx
│       └── ContestTimer.tsx
```

### 修改文件
```
backend/
├── main.go                    # 添加 Contest 路由和依赖注入
└── internal/
    └── service/
        └── submission_service.go  # 修改提交逻辑，支持赛事提交
```

---

## 🔌 与现有系统集成

### 1. 与用户系统集成
- 用户信息包含参赛记录
- 用户个人资料显示参与的赛事

### 2. 与团队系统集成
- 支持团队形式参赛
- 团队排行榜同步

### 3. 与题目系统集成
- 题目可以关联到多个赛事
- 赛事题目可能有独立的分数覆盖

### 4. 与提交系统集成
- 提交记录关联赛事ID
- 赛事提交单独计分
- 赛事独立的提交限制

### 5. 与排行榜系统集成
- 使用现有 `LeaderboardService` 实现赛事排行榜
- Redis 缓存赛事排名数据
- 定时同步数据库和缓存

---

## 🚀 高级功能扩展（可选）

### 1. 实时排行榜更新
- WebSocket 连接推送排名变化
- Redis Pub/Sub 实现实时更新

### 2. 赛事动态通知
- 赛事开始/结束通知
- 新题目发布通知
- 排行榜变动通知

### 3. 赛事模板系统
- 预定义赛事模板
- 快速创建相似赛事

### 4. 赛事数据分析
- 参赛者活跃度统计
- 题目解决率分析
- 赛事热度指标

### 5. 多赛事模式支持
- Jeopardy 模式（传统CTF）
- Attack-Defense 模式（攻防对抗）
- King of the Hill 模式（占旗模式）

---

## 📊 开发里程碑

### 里程碑 1: 基础 API 完成
- [ ] Contest 模型定义完成
- [ ] Repository 层实现完成
- [ ] Service 层实现完成
- [ ] Handler 层实现完成
- [ ] 路由注册完成

**预计时间**: 2-3天

### 里程碑 2: 核心功能测试
- [ ] 赛事 CRUD 功能测试通过
- [ ] 参赛管理功能测试通过
- [ ] 赛事题目关联测试通过
- [ ] 基础权限验证完成

**预计时间**: 1-2天

### 里程碑 3: 前端界面完成
- [ ] 赛事列表页面完成
- [ ] 赛事详情页面完成
- [ ] 赛事管理后台完成
- [ ] 赛事排行榜页面完成

**预计时间**: 3-4天

### 里程碑 4: 集成测试完成
- [ ] 端到端测试完成
- [ ] 性能测试完成
- [ ] 安全测试完成
- [ ] 文档更新完成

**预计时间**: 1-2天

**总预计时间**: 7-11天

---

## ⚠️ 技术风险和应对策略

### 风险 1: 数据库设计兼容性
**问题**: 现有数据库表设计可能不满足所有业务需求
**应对**: 
- 详细分析现有表结构
- 必要时添加迁移脚本
- 保持向后兼容

### 风险 2: Redis 缓存一致性
**问题**: 赛事排行榜与数据库数据可能不一致
**应对**:
- 实现数据同步机制
- 添加缓存失效策略
- 提供手动刷新接口

### 风险 3: 权限控制复杂性
**问题**: 赛事权限模型复杂（创建者、管理员、参赛者、观众等）
**应对**:
- 设计清晰的权限层级
- 实现灵活的权限检查中间件
- 提供详细的权限文档

### 风险 4: 时间处理复杂性
**问题**: 时区处理、时间比较等容易出错
**应对**:
- 统一使用 UTC 时间存储
- 提供时区转换工具函数
- 添加时间验证逻辑

---

## 📝 数据库变更说明

### 现有表结构（已存在）
```sql
-- Contests 表
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
);

-- Contest Challenges 关联表
CREATE TABLE contest_challenges (
    contest_id CHAR(36) NOT NULL,
    challenge_id INT NOT NULL,
    score_override INT,
    PRIMARY KEY (contest_id, challenge_id)
);

-- Contest Participants 参赛者表
CREATE TABLE contest_participants (
    contest_id CHAR(36) NOT NULL,
    user_id CHAR(36),
    team_id CHAR(36),
    registered_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (contest_id, user_id, team_id)
);
```

### 可能需要的新字段
1. **contests 表**:
   - `is_public` TINYINT(1) - 是否公开赛事
   - `max_participants` INT - 最大参赛人数限制
   - `registration_deadline` DATETIME - 报名截止时间

2. **contest_participants 表**:
   - `status` VARCHAR(16) - 参赛状态（active, banned, withdrawn）
   - `last_submission_at` DATETIME - 最后提交时间

**注意**: 这些字段可在后续迭代中添加，当前版本优先实现核心功能。

---

## 🧪 测试计划

### 单元测试覆盖
1. **Service 层测试**:
   - 赛事创建和验证
   - 参赛逻辑测试
   - 状态计算测试

2. **Repository 层测试**:
   - 数据库操作测试
   - 关联查询测试
   - 错误处理测试

### 集成测试
1. **API 接口测试**:
   - 使用 Postman/curl 测试所有端点
   - 认证和权限测试
   - 错误响应测试

2. **数据库集成测试**:
   - 事务一致性测试
   - 并发操作测试
   - 数据完整性测试

### 端到端测试
1. **完整赛事流程测试**:
   - 创建赛事 → 添加题目 → 用户报名 → 提交flag → 查看排行榜
   - 团队参赛流程测试
   - 时间限制测试

---

## 📚 文档更新计划

1. **API 文档**:
   - 使用 Swagger/OpenAPI 生成接口文档
   - 更新 API 使用示例

2. **开发文档**:
   - 更新 progress 系列文档
   - 添加赛事功能架构说明

3. **用户文档**:
   - 赛事功能使用指南
   - 常见问题解答

---

## ✅ 验收标准

### 功能验收
- [ ] 可以创建、编辑、删除赛事
- [ ] 可以向赛事添加/移除题目
- [ ] 用户可以报名/退出赛事
- [ ] 赛事排行榜正常显示
- [ ] 赛事状态正确计算（进行中、即将开始、已结束）

### 性能验收
- [ ] 赛事列表加载时间 < 500ms
- [ ] 赛事详情加载时间 < 300ms
- [ ] 排行榜查询时间 < 100ms
- [ ] 支持至少 1000 人同时参赛

### 安全验收
- [ ] 只有管理员可以创建/修改赛事
- [ ] 用户只能报名公开赛事或受邀请赛事
- [ ] 赛事数据访问权限正确控制
- [ ] 无 SQL 注入、XSS 等安全漏洞

---

## 🎉 总结

赛事功能是 CTF 平台的核心功能之一，完善的赛事系统能够极大地提升用户体验和平台价值。本开发计划按照"基础API → 核心功能 → 前端界面 → 测试优化"的渐进式开发思路，确保每个阶段都有明确的交付物和验收标准。

通过本计划的实施，CTF 平台将具备完整的赛事管理能力，支持从赛事创建到结束的全流程管理，为后续的高级功能（如实时排行榜、多赛事模式等）打下坚实基础。

---
*文档版本: v1.0 | 创建时间: 2026-03-11 | 创建人: CTF平台开发团队*