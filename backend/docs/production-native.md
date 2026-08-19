# 生产环境原生部署（不使用 Docker）

本方案直接运行 Go 编译出的 `api` 和 `worker` 二进制，MySQL、Redis/Memurai 和 Nginx 使用服务器已有服务。密钥只放在服务器环境文件中。

## 1. 构建 Linux 二进制

在 Windows 开发机的 `D:\微信小游戏\backend` 执行：

```powershell
$env:CGO_ENABLED = "0"
$env:GOOS = "linux"
$env:GOARCH = "amd64"
D:\bin\go.exe build -trimpath -ldflags "-s -w" -o api ./cmd/api
D:\bin\go.exe build -trimpath -ldflags "-s -w" -o worker ./cmd/worker
```

同时复制以下目录到服务器：

- `api`
- `worker`
- `database/migrations/`
- `scripts/`
- `configs/`（只作为结构参考，不放密钥）

建议服务器目录：`/opt/24-calculate/`。

## 2. 服务器环境文件

创建 `/etc/24-calculate/api.env`，权限设为 `600`，至少包含：

```text
GO_SERVICE_APP_NAME=twenty-four-calculate
GO_SERVICE_APP_ENV=production
GO_SERVICE_SERVER_HOST=127.0.0.1
GO_SERVICE_SERVER_PORT=8080
GO_SERVICE_DATABASE_HOST=127.0.0.1
GO_SERVICE_DATABASE_PORT=3306
GO_SERVICE_DATABASE_USER=<production-db-user>
GO_SERVICE_DATABASE_PASSWORD=<production-db-password>
GO_SERVICE_DATABASE_NAME=<production-db-name>
GO_SERVICE_REDIS_ADDR=127.0.0.1:6379
GO_SERVICE_REDIS_DB=2
GO_SERVICE_WECHAT_APP_ID=wx1e7ac815548c561c
GO_SERVICE_WECHAT_APP_SECRET=<wechat-app-secret>
GO_SERVICE_WECHAT_API_BASE_URL=https://api.weixin.qq.com
GO_SERVICE_JWT_SECRET=<random-secret-at-least-32-bytes>
GO_SERVICE_GAME_DAILY_SEED_SECRET=<separate-random-daily-secret>
GO_SERVICE_GAME_CAMPAIGN_CONTENT_VERSION=v1
GO_SERVICE_GAME_CAMPAIGN_CONTENT_SECRET=<stable-campaign-content-secret>
GO_SERVICE_AVATAR_STORAGE_DIR=/var/lib/24-calculate/avatars
GO_SERVICE_AVATAR_PUBLIC_BASE_URL=https://<actual-image-domain>
GO_SERVICE_QUEUE_NAME=twenty_four_calculate
GO_SERVICE_LOG_LEVEL=info
GO_SERVICE_LOG_FORMAT=json
```

生产数据库名、Redis 地址和账号必须以服务器实际配置为准，不要直接照抄示例值。`actual-image-domain` 必须替换成真实的 HTTPS 图片下载域名；如果由同一台 Nginx 提供静态文件，可以使用 `https://calc-api.pdurl.cn`，并为 `/avatars/` 配置只读静态目录，否则使用实际对象存储/CDN 域名。

## 3. 数据库迁移

先备份数据库，再在服务器上使用 Goose 执行：

```bash
cd /opt/24-calculate
goose -dir database/migrations mysql \
  "<production-db-user>:<production-db-password>@tcp(<production-db-host>:3306)/<production-db-name>?parseTime=true&charset=utf8mb4&loc=UTC" up
```

执行后确认 Goose 显示当前迁移版本，并记录输出。不要执行 `down` 或删除已有表。

## 4. systemd 服务

创建 `/etc/systemd/system/twenty-four-calculate-api.service`：

```ini
[Unit]
Description=Twenty Four Calculate API
After=network-online.target mysql.service redis.service
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=/opt/24-calculate
EnvironmentFile=/etc/24-calculate/api.env
ExecStart=/opt/24-calculate/api
Restart=always
RestartSec=3
User=twenty-four
Group=twenty-four
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

创建 `twenty-four-calculate-worker.service` 时，把 `ExecStart` 改为 `/opt/24-calculate/worker`，其余配置保持一致。启动：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now twenty-four-calculate-api twenty-four-calculate-worker
sudo systemctl status twenty-four-calculate-api twenty-four-calculate-worker
```

## 5. Nginx 与验收

Nginx 的 HTTPS server 使用 `calc-api.pdurl.cn` 证书，并代理到：

```nginx
proxy_pass http://127.0.0.1:8080;
```

如果头像使用本机文件存储，还要在同一个 HTTPS server 中增加静态下载位置，目录要与
`GO_SERVICE_AVATAR_STORAGE_DIR` 对应（示例）：

```nginx
location /avatars/ {
    alias /var/lib/24-calculate/avatars/avatars/;
    add_header Cache-Control "public, max-age=86400";
}
```

执行：

```bash
sudo nginx -t
sudo systemctl reload nginx
curl --fail https://calc-api.pdurl.cn/health
curl --fail https://calc-api.pdurl.cn/ready
```

`/health` 成功只代表 API 进程存活；`/ready` 成功才代表 MySQL 和 Redis 可用。最后确认生产环境的 `/api/v1/auth/dev-login` 和 Swagger 均不可访问。

## 6. 回滚与备份

发布前保留上一版 `api`、`worker` 和配置备份；数据库迁移前执行 `mysqldump --single-transaction` 并至少恢复演练一次。Redis 中的房间、匹配和 Run 状态是临时数据，禁止用清空 Redis 的方式发布或回滚。

### 6.1 使用仓库脚本完成备份与恢复演练

仓库中的 `scripts/backup-mysql.sh` 和 `scripts/restore-mysql-verify.sh` 只适用于
原生 MySQL，不依赖 Docker。先在服务器创建权限为 `600` 的 MySQL option 文件，
再执行：

```bash
export MYSQL_DEFAULTS_FILE=/etc/24-calculate/mysql-backup.cnf
export MYSQL_DATABASE=<production-db-name>
export BACKUP_DIR=/var/backups/24-calculate
bash /opt/24-calculate/scripts/backup-mysql.sh
```

恢复演练必须使用新的隔离库名，脚本会拒绝覆盖已有数据库和生产数据库：

```bash
export BACKUP_FILE=/var/backups/24-calculate/<backup-file>.sql.gz
export RESTORE_DATABASE=verify_$(date -u +%Y%m%d)
bash /opt/24-calculate/scripts/restore-mysql-verify.sh
```

备份文件和 `.sha256` 校验文件应保存到独立磁盘或对象存储；脚本不会自动删除旧备份。

## 7. 健康检查与告警

`scripts/production-health.sh` 会同时检查 `/health` 和 `/ready`，适合被现有监控系统、
定时任务或 Uptime 服务调用。`/health` 只表示进程存活，`/ready` 还会检查 MySQL 和
Redis/Memurai。生产环境至少应对以下情况告警：连续健康检查失败、5xx 比例升高、接口
延迟异常、API/worker systemd 服务停止，以及备份任务失败。

```bash
bash /opt/24-calculate/scripts/production-health.sh https://calc-api.pdurl.cn
```
