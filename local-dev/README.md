# Windows 本地中间件

需要 Windows x64、PowerShell 7.4+、运行 Linux 容器的 Docker Desktop 和 `kubectl` 1.34。k3d 工具和 Kata 发布包首次启动时下载并校验。Kata 使用 Cloud Hypervisor，经 Unix socket 与客体代理通信。Docker Desktop 的 Linux 环境需要能够访问 KVM 和 vhost-net 设备。

从仓库根目录执行：

```powershell
.\local-dev\dev.ps1 up
.\local-dev\dev.ps1 status
.\local-dev\dev.ps1 down
.\local-dev\dev.ps1 mysql reset
```

`up` 准备 MySQL、Redis、RabbitMQ、Registry 和 k3d。MySQL、Redis 和 RabbitMQ 的端口与凭证来自仓库根目录的 `.env`。`up` 复用配置一致的容器，只为缺少的服务执行 Compose `up`。如果已有容器的配置与 `.env` 不一致，先确认旧数据卷中的账号和数据，再处理容器配置。`down` 停止中间件和集群，保留容器和数据卷。

`.env` 中的 `MINIO_ENABLED=true` 会启用 MinIO。`MINIO_ENDPOINT` 使用本机地址和端口，例如 `localhost:9000`，`MINIO_USE_SSL` 设置为 `false`。凭证读取 `MINIO_ACCESS_KEY` 和 `MINIO_SECRET_KEY`，对象存储使用独立数据卷。

中间件就绪后，在 `backend/` 执行 `go run .`，在另一个终端进入 `frontend/` 执行 `npm.cmd run dev -- --host 127.0.0.1 --strictPort`。两者从仓库根目录读取 `.env`。以 `backend/` 为工作目录时，`KUBECONFIG_PATH=../local-dev/.local/kubeconfig.yaml`。

`mysql reset` 会清空本地 `ctf_exec` 数据库，再导入 `db/final.sql` 和 `db/seed/` 中已有的 SQL 文件。执行前结束题目实例并手动停止后端。下次启动后端时会使用 `.env` 中的 `ROOT_DEFAULT_PASSWORD` 创建平台账号 `root`。如果 MySQL 原本已停止，重置完成后会恢复停止状态。

Web 题使用 `http://<实例>.chal.localhost`，TCP 题使用 `127.0.0.1:30100` 至 `127.0.0.1:30115` 中分配的端口。容器题可以保留默认的 `use_kata=true`。本地 Registry 使用 `127.0.0.1:15000`，只用于本机开发。

### 日志与本机配置

kubeconfig 和下载的工具保存在 Git 忽略的 `local-dev/.local/`。如果 Kata 启动检查失败，可用以下命令查看 Pod 事件：

```powershell
kubectl --kubeconfig .\local-dev\.local\kubeconfig.yaml describe pod ctf-exec-kata-smoke
docker logs k3d-ctf-exec-local-server-0
```

可在 `machine.psd1` 中设置 `K3dPath`，指定已安装的 k3d 5.9.0。需要填写本机绝对路径时，先将空白版本纳入 Git 索引，再执行 `git update-index --skip-worktree -- local-dev/machine.psd1`，然后填写路径。用 `git ls-files -v -- local-dev/machine.psd1` 检查文件前的 `S` 标记。恢复跟踪检查时使用 `git update-index --no-skip-worktree -- local-dev/machine.psd1`。
