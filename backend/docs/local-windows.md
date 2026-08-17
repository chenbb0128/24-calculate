# Windows 本机运行与双玩家测试

本项目的本机方案使用 MySQL、Memurai 和 Go，不需要 Docker。

## 1. 启动依赖

确认 MySQL 8+ 和 Memurai 服务已经启动。可以在 PowerShell 查看：

```powershell
Get-Service MySQL*,Memurai*
```

如果服务是停止状态，需要用管理员 PowerShell 启动：

```powershell
Start-Service Memurai
Start-Service MySQL80
```

MySQL 服务名如果不同，以 `Get-Service` 显示的名称为准。

## 2. 数据库迁移

在 `D:\微信小游戏\backend` 执行，把命令中的密码替换为本机真实密码：

```powershell
cd D:\微信小游戏\backend
& "C:\Users\Administrator\go\bin\goose.exe" -dir database\migrations mysql "root:真实密码@tcp(127.0.0.1:3306)/go_service?parseTime=true" up
```

本次好友对战、商店、偏好设置使用已有表和 Memurai，不需要新增迁移。

## 3. 启动后端

```powershell
$env:GO_SERVICE_APP_ENV = "development"
$env:GO_SERVICE_DATABASE_PASSWORD = "本机MySQL密码"
$env:GO_SERVICE_REDIS_PASSWORD = ""
$env:GO_SERVICE_JWT_SECRET = "local-development-secret-change-me-32"
cd D:\微信小游戏\backend
D:\bin\go.exe run ./cmd/api
```

看到 `api server started` 后保持窗口打开。

## 4. 不使用两个微信账号测试

开发环境额外提供：

```text
POST /api/v1/auth/dev-login
Body: {"slot":1}
```

`slot` 支持 1 到 9，服务端会创建或复用 `dev_player_1` 到 `dev_player_9`。该接口只在 development/test/staging 注册，production 不注册。

在微信开发者工具的 Console 中分别设置两个测试窗口：

```javascript
wx.setStorageSync('twenty_four_dev_login_slot', 1);
wx.removeStorageSync('twenty_four_auth');
```

第二个窗口把 `1` 改成 `2`，然后重新编译。前端已经打开本机后端地址，会自动使用对应的开发账号登录。

## 5. 手工验收顺序

1. 两个窗口分别登录测试账号 1 和 2。
2. 账号 1 创建好友房间，账号 2 输入房间号加入。
3. 两边分别点击“准备”，确认第二名玩家准备后房间自动进入 `countdown`，并返回同一个 `match_id`、`room_seed`、`question_hash` 和 `start_at`；不需要再手动点击开始。
4. 倒计时结束后完成一题，确认对手进度会更新；断开/恢复连接时 15 秒内可以继续对局。
6. 双方完成整局，确认服务端计算胜负、金币只发放一次、好友排行榜出现成绩。
7. 在商店购买或装备外观，重新编译/重新登录后确认仍然存在。
8. 修改音频设置，重新登录后确认设置保留。

## 6. 上线前必须替换

- 前端 `src/services/api_client.js` 的 `API_BASE_URL` 换成 HTTPS 后端地址。
- 微信后台配置该 HTTPS 域名为 request 合法域名。
- `GO_SERVICE_APP_ENV=production`，并配置真实 JWT、微信 AppID/AppSecret、数据库密码。
- production 不使用 `twenty_four_dev_login_slot`。
- 上线前执行 `/health` 和 `/ready`，并配置 MySQL 定期备份。
