# CTF 平台静态资源管理说明

## 目录结构

static/
├── avatars/           # 用户头像
├── attachments/       # 题目附件（二进制文件、压缩包等）
├── covers/           # 比赛封面图
└── uploads/          # 临时上传目录

## 文件存储路径规范

### 1. 用户头像 (avatars/)

**存储路径格式**：`static/avatars/{user_id}/{timestamp}_{original_filename}`

**示例**：
```
static/avatars/11111111-1111-1111-1111-111111111111/1707667200_avatar.jpg
```

**数据库字段**：`users.avatar_url`

**建议**：
- 支持格式：JPG, PNG, GIF, WebP
- 最大文件大小：2MB
- 推荐尺寸：256x256 或 512x512
- 自动生成缩略图（可选）

---

### 2. 题目附件 (attachments/)

**存储路径格式**：`static/attachments/{problem_id}/{timestamp}_{original_filename}`

**示例**：
```
static/attachments/1/1707667300_challenge.zip
static/attachments/2/1707667400_binary_file
```

**数据库字段**：`attachments.storage_path`, `attachments.file_name`, `attachments.file_type`

**建议**：
- 支持格式：ZIP, TAR.GZ, EXE, ELF, PDF, TXT 等
- 最大文件大小：50MB
- 压缩包建议使用 ZIP 格式
- 记录文件 SHA256 哈希值（可选，用于完整性校验）

---

### 3. 比赛封面图 (covers/)

**存储路径格式**：`static/covers/{contest_id}/{timestamp}_{original_filename}`

**示例**：
```
static/covers/11111111-1111-1111-1111-111111111111/1707667500_ctf_contest_2024.png
static/covers/22222222-2222-2222-2222-222222222222/1707667600_spring_ctf.jpg
```

**数据库字段**：`contests.cover_image`

**建议**：
- 支持格式：JPG, PNG, WebP
- 最大文件大小：5MB
- 推荐尺寸：1200x630 (适合卡片展示)
- 如未设置，使用默认封面图

---

### 4. 临时上传目录 (uploads/)

**用途**：临时存储用户上传的文件，处理完成后移动到对应目录或删除

**清理策略**：
- 定期清理超过 24 小时的临时文件
- 上传成功后立即移动到目标目录
- 上传失败后立即删除

---

## 文件命名规范

### 时间戳格式
使用 Unix 时间戳（秒级）：`1707667200`

### 文件名处理
1. **保留原始文件名**：便于用户识别
2. **添加时间戳前缀**：避免文件名冲突
3. **移除特殊字符**：仅保留字母、数字、下划线、点号、连字符
4. **转换为小写**（可选）：统一命名风格

### 示例转换
```
原始文件名: "My Challenge File (v2).zip"
处理后: "1707667200_my_challenge_file_v2.zip"
```

---

## 文件访问控制

### 公开访问
- ✅ 用户头像：所有人可访问
- ✅ 比赛封面图：所有人可访问

### 需要认证
- 🔒 题目附件：仅登录用户可下载
- 🔒 临时上传文件：仅上传者可访问

### 管理员专属
- 🔐 上传题目附件
- 🔐 上传比赛封面图
- 🔐 删除任意文件

---

## 后端 API 端点（待实现）

### 文件上传
```
POST /api/upload/avatar          # 上传用户头像（需认证）
POST /api/upload/attachment      # 上传题目附件（需管理员权限）
POST /api/upload/contest-cover   # 上传比赛封面（需管理员权限）
```

### 文件访问
```
GET /static/avatars/{path}       # 访问头像（公开）
GET /static/covers/{path}        # 访问封面图（公开）
GET /static/attachments/{path}   # 下载附件（需认证）
```

### 文件管理
```
DELETE /api/files/{id}           # 删除文件（需管理员权限）
GET /api/files/{id}/info         # 获取文件信息
```

---

## 安全建议

### 1. 文件类型验证
- 检查文件扩展名
- 检查 MIME 类型
- 检查文件魔数（Magic Number）

### 2. 文件大小限制
```go
const (
    MaxAvatarSize     = 2 * 1024 * 1024    // 2MB
    MaxCoverSize      = 5 * 1024 * 1024    // 5MB
    MaxAttachmentSize = 50 * 1024 * 1024   // 50MB
)
```

### 3. 防止路径遍历攻击
- 验证文件路径不包含 `..`
- 使用白名单验证文件名字符
- 使用 `filepath.Clean()` 清理路径

### 4. 病毒扫描（可选）
- 集成 ClamAV 或其他杀毒引擎
- 扫描所有上传文件

### 5. 存储配额
- 限制每个用户的总存储空间
- 限制每个题目的附件总大小

---

## 数据库字段映射

| 资源类型 | 数据库表 | 字段名 | 类型 | 说明 |
|---------|---------|--------|------|------|
| 用户头像 | `users` | `avatar_url` | VARCHAR(1024) | 存储相对路径 |
| 比赛封面 | `contests` | `cover_image` | VARCHAR(1024) | 存储相对路径 |
| 题目附件 | `attachments` | `storage_path` | VARCHAR(1024) | 存储相对路径 |
| 题目附件 | `attachments` | `file_name` | VARCHAR(255) | 原始文件名 |
| 题目附件 | `attachments` | `file_type` | VARCHAR(50) | MIME 类型 |

**路径存储示例**：
- ✅ 相对路径：`static/avatars/xxx/xxx.jpg`
- ❌ 绝对路径：`/home/thit/ctf_exec/static/avatars/xxx/xxx.jpg`
- ❌ URL：`http://localhost:8080/static/avatars/xxx/xxx.jpg`

---

## 实施清单

- [x] 创建目录结构
- [x] 添加 .gitkeep 文件
- [x] 更新数据库 schema（添加 `contests.cover_image`）
- [ ] 实现文件上传 API
- [ ] 实现文件访问控制
- [ ] 添加文件类型验证
- [ ] 实现文件大小限制
- [ ] 添加临时文件清理任务

---

## 未来扩展

### 对象存储集成
支持将文件存储到云端对象存储服务：
- 阿里云 OSS
- 腾讯云 COS
- AWS S3
- MinIO（自托管）

### CDN 加速
- 静态资源通过 CDN 分发
- 提升全球访问速度
- 减轻服务器带宽压力

### 图片处理
- 自动生成缩略图
- 图片压缩优化
- WebP 格式转换
- 水印添加
