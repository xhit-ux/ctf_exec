# CTF 平台项目进度总结与规划

**文档创建时间**: 2026-02-12  
**项目名称**: CTF Exec Platform  
**技术栈**: Go (后端) + React + TypeScript + Vite (前端) + MySQL (数据库)

---

## 📊 当前项目状态概览

### 项目结构
```
ctf_exec/
├── backend/          # Go 后端服务
│   ├── main.go      # 主入口
│   ├── internal/    # 内部包
│   │   ├── db/      # 数据库连接
│   │   ├── handler/ # HTTP 处理器 (auth, user, problem, submission)
│   │   ├── middleware/ # 中间件 (CORS, Auth)
│   │   ├── models/  # 数据模型 (user, problem, submission)
│   │   ├── repository/ # 数据访问层
│   │   └── utils/   # 工具函数
│   └── go.mod
├── frontend/         # React 前端
│   ├── src/
│   │   ├── pages/   # 页面组件 (8个)
│   │   ├── components/ # 通用组件 (6个)
│   │   ├── context/ # AuthContext
│   │   └── App.tsx
│   └── package.json
├── db/              # 数据库脚本
│   ├── init_all.sql # 整合的初始化脚本 (MySQL)
│   ├── migrations/  # 数据库迁移脚本
│   └── *.sql        # 各表独立脚本
├── static/          # 静态资源存储
│   ├── avatars/     # 用户头像
│   ├── attachments/ # 题目附件
│   ├── covers/      # 题目封面图
│   └── uploads/     # 临时上传
└── note/            # 项目笔记
    ├── USERS_SCHEMA.md
    ├── db_design.md
    ├── 项目进度总结与规划.md
    └── env.note.txt
```

---

## ✅ 已完成功能模块

### 1. 数据库设计 (已完成 90%)

#### 核心表结构
- ✅ **users** - 用户表，包含完整的用户信息、认证、统计字段
- ✅ **teams** / **team_members** - 团队系统
- ✅ **problems** - 题目表 (支持静态/动态/容器化题目)
- ✅ **submissions** - 提交记录表
- ✅ **challenge_instances** - 动态题目实例管理
- ✅ **attachments** - 题目附件
- ✅ **docker_images** - Docker 镜像记录
- ✅ **contests** / **contest_challenges** / **contest_participants** - 比赛系统
- ✅ **user_activity_logs** - 用户活动日志

#### 数据库特性
- ✅ 已整合为单一 MySQL 初始化脚本 (`init_all.sql`)
- ✅ 包含 Demo 数据 (alice/bob 用户, 5道题目)
- ✅ 完善的索引与外键约束
- ✅ 支持 UUID 主键
- ✅ Demo 用户密码: `123456` (bcrypt 加密)
- ✅ **静态资源路径字段**：`users.avatar_url`, `contests.cover_image`, `attachments.storage_path`

### 2. 后端 API (已完成 60%)

#### 已实现的 API 端点
```
认证相关:
✅ POST /api/auth/register    - 用户注册
✅ POST /api/auth/login       - 用户登录
✅ GET  /api/auth/me          - 获取当前用户信息 (需认证)

用户相关:
✅ GET  /api/users/leaderboard - 排行榜
✅ GET  /api/users/profile     - 更新个人资料 (需认证)
✅ GET  /api/users/public      - 获取公开资料 (通过 username)

题目相关:
✅ GET  /api/problems          - 获取题目列表 (需认证)
✅ GET  /api/problems/detail   - 获取题目详情 (需认证)

提交相关:
✅ POST /api/submissions       - 提交 flag (需认证)
✅ GET  /api/submissions/me    - 获取我的提交记录 (需认证)

团队相关:
✅ GET  /api/teams/leaderboard  - 获取团队排行榜
✅ GET  /api/teams/public       - 获取公共团队详情 (支持脱敏与可选认证)
✅ GET  /api/teams/my           - 获取我的团队信息
✅ POST /api/teams              - 创建团队
✅ POST /api/teams/join         - 加入团队
✅ PUT  /api/teams/update       - 修改团队名称 (仅队长)
✅ POST /api/teams/kick         - 踢出成员 (仅队长)
✅ DELETE /api/teams/leave      - 退出团队
✅ DELETE /api/teams/dissolve   - 解散团队 (仅队长)

系统:
✅ GET  /health               - 健康检查
```

#### 后端架构特点
- ✅ 清晰的分层架构 (Handler → Repository → DB)
- ✅ JWT 认证中间件
- ✅ CORS 中间件
- ✅ bcrypt 密码加密
- ✅ 使用 MySQL 数据库
- ✅ 依赖注入设计模式

### 3. 前端界面 (已完成 70%)

#### 已实现的页面
- ✅ **Login** - 登录页面
- ✅ **Register** - 注册页面
- ✅ **Problems** - 题目列表页 (首页)
- ✅ **ProblemDetail** - 题目详情页
- ✅ **MySubmissions** - 我的提交记录
- ✅ **Scoreboard** - 排行榜
- ✅ **Profile** - 个人资料页
- ✅ **TeamProfile** - 团队详情页 (带管理中心)
- ✅ **MyTeam** - 我的团队页 (带管理中心)
- ✅ **Admin** - 管理员页面

#### 前端功能特性
- ✅ React Router 路由管理
- ✅ AuthContext 全局认证状态管理
- ✅ ProtectedRoute 路由保护
- ✅ Layout 布局组件 (Header, Sidebar, Footer)
- ✅ Tailwind CSS 样式系统
- ✅ TypeScript 类型支持
- ✅ **已本地化为中文界面**

---

## 🔄 当前运行状态

根据终端信息，项目当前正在运行:
- ✅ 前端开发服务器: `yarn dev` (运行中，19分钟+)
- ✅ 后端服务器: `go run main.go` (运行中，19分钟+)
- ✅ 数据库: MySQL (通过 sudo su 管理)

**访问地址**:
- 前端: http://localhost:5173 (Vite 默认端口)
- 后端: http://localhost:8080

---

## ⚠️ 已知问题与待解决事项

### 1. 前端渲染问题 (已在历史会话中提及)
- ⚠️ 根路径 `/` 可能存在空白页问题
- 需要检查 `Problems.tsx` 和 `Layout.tsx` 的渲染逻辑

### 2. 密码处理一致性 (历史会话提及)
- ⚠️ 前后端密码处理逻辑可能不一致
- 需要确保前端发送明文密码，后端进行 bcrypt 加密

### 3. 数据库迁移管理
- ⚠️ 当前缺少数据库迁移工具
- 建议引入 Flyway / golang-migrate / Goose

---

## 🎯 下一步开发规划

### 阶段一: 核心功能完善 (优先级: 高)

#### 1.1 修复已知 Bug
- [ ] **修复前端根路径空白页问题**
  - 检查 `Problems.tsx` 数据获取逻辑
  - 确认 API 调用是否正常
  - 验证 Layout 组件渲染

- [ ] **验证密码处理流程**
  - 前端登录/注册时发送明文密码
  - 后端使用 bcrypt 加密存储
  - 测试 demo 用户登录 (alice/bob, 密码: 123456)

#### 1.2 完善 API 功能
- [ ] **题目管理 API** (管理员功能)
  - `POST /api/admin/problems` - 创建题目
  - `PUT /api/admin/problems/:id` - 更新题目
  - `DELETE /api/admin/problems/:id` - 删除题目

- [ ] **用户管理 API**
  - `GET /api/users/:id` - 获取用户详情
  - `PUT /api/users/:id` - 更新用户信息
  - `GET /api/users/:id/submissions` - 获取用户提交记录

- [x] **团队功能 API** (已完成)
  - [x] `POST /api/teams` - 创建团队 (生成邀请码)
  - [x] `POST /api/teams/join` - 加入团队 (通过邀请码)
  - [x] `DELETE /api/teams/leave` - 离开团队
  - [x] `DELETE /api/teams/dissolve` - 解散团队 (队长权限)
  - [x] `PUT /api/teams/update` - 修改团队名称 (队长)
  - [x] `POST /api/teams/kick` - 踢出成员 (队长)
  - [x] `GET /api/teams/my` - 获取我的团队信息
  - [x] `GET /api/teams/public` - 获取公共团队详情
  - [x] `GET /api/teams/leaderboard` - 团队排行榜 (按人均积分, Fixed Score Bug)

#### 1.3 前端功能增强
- [ ] **完善 Admin 页面**
  - 题目管理界面 (CRUD)
  - 用户管理界面
  - 比赛管理界面

- [x] **完善 Profile 页面**
  - [x] 显示用户统计信息
  - [x] 编辑个人资料
  - [x] 上传头像功能 (支持裁剪)
  - [x] 支持查看他人公开主页
- [x] **完善 排行榜页**
  - [x] 点击用户主页链接实现跳转
  - [x] 点击团队名称跳转至团队主页 (NEW)
- [x] **新增 TeamProfile 页面**
  - [x] 实现管理功能（改名、踢人）
  - [x] 集成权限校检

- [ ] **优化题目列表页**
  - [ ] 添加分类筛选
  - [ ] 添加难度筛选
  - [ ] 添加搜索功能
  - [x] 显示解题状态

### 阶段二: 图片裁剪与优化 (优先级: 高)

#### 2.1 图片裁剪功能
- [ ] **前端图片裁剪组件**
  - 集成图片裁剪库（react-easy-crop 或 react-image-crop）
  - 头像裁剪：1:1 比例（正方形）
  - 比赛封面裁剪：4:3 比例
  - 支持缩放、旋转、拖拽
  - 实时预览裁剪结果

- [ ] **更新上传组件**
  - `AvatarUpload` 组件添加裁剪步骤
  - 创建 `ContestCoverUpload` 组件（4:3 比例）
  - 裁剪后生成 Blob 上传
  - 优化用户体验流程

- [ ] **后端图片处理**（可选）
  - 服务端图片压缩
  - 生成多种尺寸缩略图
  - WebP 格式转换

### 阶段三: 动态题目支持 (优先级: 中)

#### 3.1 容器化题目功能
- [ ] **集成 Docker/K3s**
  - 实现题目容器启动 API
  - 实现题目容器停止 API
  - 实现容器状态查询

- [ ] **动态 Flag 生成**
  - 为每个用户/团队生成唯一 flag
  - 存储在 `challenge_instances` 表

- [ ] **前端动态题目支持**
  - 显示容器启动按钮
  - 显示容器连接信息 (IP/端口)
  - 显示容器状态

### 阶段四: 比赛系统 (优先级: 中)

#### 4.1 比赛管理
- [ ] **比赛 CRUD API**
  - 创建/编辑/删除比赛
  - 设置比赛时间窗口
  - 关联题目到比赛

- [ ] **比赛参与**
  - 用户/团队报名比赛
  - 比赛期间提交限制
  - 比赛排行榜

- [ ] **前端比赛页面**
  - 比赛列表页
  - 比赛详情页
  - 比赛排行榜

### 阶段五: 高级功能 (优先级: 低)

#### 5.1 安全与审计
- [ ] **活动日志系统**
  - 记录所有关键操作
  - 管理员审计界面

- [ ] **防作弊机制**
  - 提交频率限制
  - IP 地址记录
  - 异常行为检测

#### 5.2 用户体验优化
- [ ] **实时通知系统**
  - WebSocket 支持
  - 题目更新通知
  - 排名变化通知

- [ ] **题目附件管理**
  - 文件上传下载
  - 附件存储 (本地/OSS)

#### 5.3 数据分析
- [ ] **统计仪表板**
  - 题目解题率统计
  - 用户活跃度分析
  - 比赛数据可视化

#### 5.4 社交与通信 (新规划)
- [ ] **站内通信系统**
  - 用户间私信功能
  - 团队内部聊天室
  - 全局系统通知
  - 消息推送与提醒

---

## 🛠️ 技术债务与改进建议

### 代码质量
- [ ] 添加单元测试 (后端 Go 测试)
- [ ] 添加集成测试 (API 测试)
- [ ] 添加前端组件测试 (React Testing Library)
- [ ] 添加 ESLint/Prettier 配置
- [ ] 添加 Go 代码格式化检查

### 部署与运维
- [ ] 编写 Dockerfile (前端/后端)
- [ ] 编写 docker-compose.yml
- [ ] 添加 CI/CD 配置 (GitHub Actions)
- [ ] 编写部署文档
- [ ] 配置环境变量管理 (.env 文件)

### 文档完善
- [ ] 编写 API 文档 (Swagger/OpenAPI)
- [ ] 编写开发者指南
- [ ] 编写用户使用手册
- [ ] 添加代码注释

### 性能优化
- [ ] 数据库查询优化 (添加必要索引)
- [ ] 实现排行榜缓存 (Redis)
- [ ] 前端代码分割与懒加载
- [ ] 图片/附件 CDN 加速

---

## 📝 立即可执行的任务 (本周建议)

### 优先级 1: 修复核心问题
1. **✅ 修复头像上传 JSON 解析错误**
   - 后端统一返回 JSON 格式错误
   - 前端改进错误处理逻辑
   - 已完成修复

2. **测试并修复前端空白页问题**
   - 打开浏览器访问 http://localhost:5173
   - 检查控制台错误
   - 修复 API 调用或组件渲染问题

2. **验证用户登录流程**
   - 使用 alice/123456 登录
   - 确认 JWT token 正常生成
   - 确认受保护路由可访问

### 优先级 2: 完善基础功能
3. **实现题目筛选功能**
   - 前端添加分类/难度筛选器
   - 后端添加查询参数支持

4. **完善 Admin 页面**
   - 实现题目创建表单
   - 实现题目列表管理

### 优先级 3: 用户体验优化
5. **优化前端 UI**
   - 添加加载状态提示
   - 添加错误提示
   - 优化响应式布局

6. **添加表单验证**
   - 前端表单验证
   - 后端输入验证

---

## 📚 参考资料

### 已有文档
- [用户表设计](file:///home/thit/ctf_exec/note/USERS_SCHEMA.md) - 详细的用户表字段说明
- [数据库设计说明](file:///home/thit/ctf_exec/note/db_design.md) - 整体数据库架构
- [数据库初始化脚本](file:///home/thit/ctf_exec/db/init_all.sql) - MySQL 建表与 Demo 数据

### 技术栈文档
- Go: https://go.dev/doc/
- React: https://react.dev/
- Vite: https://vitejs.dev/
- Tailwind CSS: https://tailwindcss.com/
- MySQL: https://dev.mysql.com/doc/

---

## 💡 总结

**项目整体进度**: 约 **75%** 完成

**优势**:
- ✅ 数据库设计完善，考虑了扩展性
- ✅ 后端架构清晰，分层合理
- ✅ 前端页面基本齐全
- ✅ 已实现核心的认证与题目提交流程

**待改进**:
- ⚠️ 需要修复前端渲染问题
- ⚠️ 管理员功能尚未完全实现
- ⚠️ 缺少动态题目支持
- ⚠️ 缺少测试与文档

**建议下一步**:
1. 先修复已知 Bug，确保基础功能稳定
2. 完善管理员功能，实现题目 CRUD
3. 添加题目筛选与搜索功能
4. 逐步实现动态题目与比赛系统

---

**文档维护**: 建议每完成一个阶段更新此文档，保持项目进度透明化。
