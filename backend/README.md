# 三火算术练习后端

这是微信小游戏的 Go API 服务。后端负责微信身份、题目合同、Run 状态、服务端计分、金币奖励、商城、好友房间、快速匹配、人机对手和排行榜。前端只负责显示、交互、本地缓存和发起请求。

## 技术与目录

- Go 1.26、Gin、MySQL 8+、Redis/Memurai、JWT、Asynq、Goose、sqlc
- `cmd/api`：HTTP API
- `cmd/worker`：异步任务进程
- `internal/modules`：auth、user、player 业务模块
- `database/migrations`：Goose 数据库迁移
- `database/queries`：sqlc 查询
- `docs/openapi.yaml`：OpenAPI 文档
- `docs/backend-completion.md`：实现范围、测试结果和已知限制

## Windows 本机启动（不使用 Docker）

先启动本机 MySQL 8+ 和 Memurai/Redis，然后在 PowerShell 执行：

```powershell
cd D:\微信小游戏\backend

$env:GO_SERVICE_DATABASE_PASSWORD = "你的 MySQL 密码"
$env:GO_SERVICE_REDIS_PASSWORD = ""
$env:GO_SERVICE_JWT_SECRET = "至少 32 个字符的本地 JWT 密钥"
$env:GO_SERVICE_WECHAT_APP_ID = "wx1e7ac815548c561c"
$env:GO_SERVICE_WECHAT_APP_SECRET = "对应的微信 AppSecret"
$env:GO_SERVICE_GAME_DAILY_SEED_SECRET = "生产环境单独的每日题目密钥"
# 本机可留空；生产必须配置实际 HTTPS 图片域名和可写目录。
$env:GO_SERVICE_AVATAR_STORAGE_DIR = "var/avatars"
$env:GO_SERVICE_AVATAR_PUBLIC_BASE_URL = ""

D:\bin\go.exe run ./cmd/api
```

看到 `api server started` 后，API 默认地址为 `http://127.0.0.1:8080`。AppSecret、数据库密码和 JWT 密钥只通过环境变量传入，不写入代码、配置文件或日志。

## 数据库迁移

首次创建数据库后执行已有迁移，不要删除已有表或用户数据：

```powershell
cd D:\微信小游戏\backend
& "C:\Users\Administrator\go\bin\goose.exe" -dir database\migrations mysql "root:你的真实密码@tcp(127.0.0.1:3306)/go_service?parseTime=true" up
```

如果出现 `Error 1045 (28000): Access denied`，说明 MySQL 用户名或密码不正确，需要先确认本机 MySQL 的 root 密码。

## 主要接口

- `POST /api/v1/auth/wechat-login`、`POST /api/v1/auth/refresh`
- `GET/PATCH /api/v1/users/me`、`POST /api/v1/users/me/avatar`
- `GET /api/v1/player/bootstrap`
- `POST/GET /api/v1/player/campaign/runs...`
- `POST/GET /api/v1/player/daily/runs...`
- `POST/GET /api/v1/player/endless/runs...`
- `/api/v1/player/friend/rooms...` 和 `/api/v1/player/matchmaking...`
- `GET /api/v1/player/leaderboards/{mode}`，支持 campaign、daily、endless、friend、overall；成绩只通过对应的服务端校验 run/对战接口写入
- 商城：skins、cosmetics、preferences
- `GET /health`、`GET /ready`

所有受保护接口从 JWT 获取用户身份，不信任请求体中的 `user_id`、`score`、`coins`、`winner` 或 `reward`。错误响应保留数字 `code`，并增加 `request_id` 和 `data: null`。头像上传只接受 JPG/PNG/WEBP，服务端会裁剪为 256×256 WEBP；生产环境的 `GO_SERVICE_AVATAR_PUBLIC_BASE_URL` 必须是 HTTPS 图片域名。

## 测试与构建

```powershell
cd D:\微信小游戏\backend
D:\bin\go.exe test ./...
D:\bin\go.exe vet ./...
D:\bin\go.exe build ./...
```

Memurai/Redis 集成测试：

```powershell
$env:GO_SERVICE_RUN_REDIS_TESTS = "1"
D:\bin\go.exe test ./internal/modules/player -run "Test(FriendRoomRepositoryRedisLifecycle|MatchmakingRepositoryRedisLifecycle)" -v
```

## 生产部署提示

生产环境需要 HTTPS、微信平台合法 request 域名、反向代理、MySQL 定期备份和 Redis/Memurai 监控。当前正式地址为 `https://calc-api.pdurl.cn`，线上验收记录见 [`docs/release-acceptance.md`](docs/release-acceptance.md)。MySQL 可使用 `mysqldump --single-transaction` 备份；Redis 中的 Run、房间、匹配和实时进度是带 TTL 的临时状态。

生产部署不要求本机开发使用 Docker；Windows 本机步骤见 [`docs/local-windows.md`](docs/local-windows.md)。如果生产服务器直接运行 Go 二进制，可按 [`docs/production-native.md`](docs/production-native.md) 配置 systemd 和 Nginx。

## 当前限制

- Run 和房间目前以 Redis 为主，尚未迁移为独立的 `game_runs`、`friend_rooms` 历史表；Redis 丢失后不能恢复完整对局历史。
- 排行榜当前按 MySQL 查询并在进程内分页，集群部署前应增加 Redis 快照或专用排行榜存储。
- 广告奖励尚未接入第三方广告服务端回调，因此后端不会信任客户端的广告完成字段。
- 人机对手在匹配状态轮询时推进；如果需要完全脱离轮询，应将 bot 推进改成 Asynq 定时任务。
