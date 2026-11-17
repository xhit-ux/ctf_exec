# 本地 MySQL 初始化数据

此目录中的 `.sql` 文件按文件名顺序写入 `ctf_exec` 数据库。首次创建本地 MySQL 数据卷或执行 `local-dev/dev.ps1 mysql reset` 时导入。文件名使用数字开头的顺序编号。

此目录用于可选的本地测试数据。后端启动时使用本机生成的密码创建平台账号 `root`。
