# 管理后台真实数据落地 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 `vue-admin-template` 管理后台通过已实现的 Go 管理员 API 访问 MySQL 中的小程序真实用户，并完成本地联调和生产落地验收。

**Architecture:** 管理后台与 Go API 使用同源 `/admin/` 和 `/api/v1/admin/*`；本地开发服务器通过代理转发 API，生产环境由 Go 镜像直接提供 Vue 构建产物。用户数据只从现有 `users`、`user_identities` 和审核状态字段读取，微信小游戏继续通过现有 HTTPS API 创建和更新用户。

**Tech Stack:** Vue 2、Element UI、Go、Gin、MySQL、Redis/Memurai、Goose、JWT、Docker Compose。

**Spec:** `docs/superpowers/specs/2026-09-16-admin-user-management-design.md`

## Global Constraints

- 管理员后台地址固定为 `/admin/`，管理员 API 固定为 `/api/v1/admin/*`。
- 管理员密码只通过 `GO_SERVICE_ADMIN_PASSWORD` 传给后端 seed 命令，不写入前端、源码、命令行参数或日志。
- 微信 AppSecret、TapTap 密钥、数据库密码和 JWT secret 只存在后端环境变量。
- 审核业务码 `NICKNAME_REJECTED`、`AVATAR_REJECTED`、`MODERATION_PENDING`、`NICKNAME_EDIT_DISABLED` 保持不变。
- 后台展示安全昵称和安全头像，不显示被拒原始资料；审核失败不能覆盖原资料。
- 不使用 mock 用户数据作为生产数据；列表、统计、详情和状态操作必须来自后端数据库。
- 保留现有工作区改动，不执行 reset、clean、stash 或宽泛删除。

---

### Task 1: 完成本地后台到 Go API 的联调配置

**Files:**
- Modify: `frontend/admin-vue/vue.config.js`
- Modify: `frontend/admin-vue/src/views/login/index.vue`
- Modify: `frontend/admin-vue/tools/admin_vue_smoke_test.js`
- Modify: `frontend/admin-vue/README-zh.md`

**Interfaces:**
- Consumes: `frontend/admin-vue/src/api/admin.js` 的相对 API 路径和现有 Go API `http://127.0.0.1:8080`。
- Produces: 本地 `http://127.0.0.1:9528/admin/` 到 `http://127.0.0.1:8080/api/v1/admin/*` 的代理；登录失败可见提示；可复制的本地联调说明。

- [ ] **Step 1: 写失败的本地代理和登录反馈断言**

在 `admin_vue_smoke_test.js` 增加断言：`vue.config.js` 存在 `/api` 到 `http://127.0.0.1:8080` 的代理；登录页面调用 `this.$message.error` 或等价的 Element UI 错误提示；登录页不包含固定密码。

- [ ] **Step 2: 运行冒烟测试确认失败**

运行：

```powershell
Set-Location D:\微信小游戏\frontend\admin-vue
npm run test:admin
```

预期：由于代理和登录失败提示尚未补齐而失败。

- [ ] **Step 3: 增加开发代理和登录错误提示**

在 `vue.config.js` 的 `devServer` 中增加：

```js
proxy: {
  '/api': {
    target: 'http://127.0.0.1:8080',
    changeOrigin: true
  }
}
```

在登录 catch 分支保留 loading 复位，并显示：

```js
}).catch(error => {
  this.loading = false
  this.$message.error(error.message || '登录失败，请检查后端服务')
})
```

- [ ] **Step 4: 补充本地联调说明**

在 `README-zh.md` 说明：先启动 MySQL、Memurai/Redis、Go API 和 `admin-seed`，再运行 `npm run dev`；本地页面使用 `http://127.0.0.1:9528/admin/`，生产页面使用 `https://calc-api.pdurl.cn/admin/`。密码只从本地环境变量输入。

- [ ] **Step 5: 运行前端验证**

运行：

```powershell
Set-Location D:\微信小游戏\frontend\admin-vue
npm run test:admin
npm run lint
npm run build:prod
```

预期：全部退出码为 0；构建只允许已有 Element UI 体积 warning，不得出现编译错误。

### Task 2: 启动本机后端依赖并准备真实管理员账号

**Files:**
- Inspect only: `backend/.env.example`
- Inspect only: `backend/database/migrations/00014_create_admin_accounts.sql`
- Inspect only: `backend/cmd/admin-seed/main.go`
- Inspect only: `backend/README.md`

**Interfaces:**
- Consumes: MySQL 3306、Memurai/Redis 6379、现有迁移和 `admin-seed`。
- Produces: 已执行数据库迁移、已创建独立管理员账号、可启动的本地 Go API 依赖。

- [ ] **Step 1: 检查依赖状态**

运行：

```powershell
Get-NetTCPConnection -State Listen -LocalPort 3306,6379 -ErrorAction SilentlyContinue
Get-Service -Name MySQL*,MariaDB*,Redis*,Memurai* -ErrorAction SilentlyContinue
```

MySQL 必须监听 3306，Memurai/Redis 必须监听 6379；缺少任何一个就停止联调并报告缺少的本地依赖，不猜测密码。

- [ ] **Step 2: 启动 Memurai 并执行 Goose 迁移**

在确认 MySQL 连接信息后执行：

```powershell
Start-Service Memurai
Set-Location D:\微信小游戏\backend
$dbUser = $env:GO_SERVICE_DATABASE_USER
$dbPassword = $env:GO_SERVICE_DATABASE_PASSWORD
$dbName = $env:GO_SERVICE_DATABASE_NAME
& "$env:USERPROFILE\go\bin\goose.exe" -dir database\migrations mysql "${dbUser}:${dbPassword}@tcp(127.0.0.1:3306)/${dbName}?parseTime=true" up
```

迁移完成后确认 `00014_create_admin_accounts.sql` 已应用，不修改历史迁移、不写入默认账号。

- [ ] **Step 3: 用环境变量创建管理员**

只在当前 PowerShell 进程设置账号和密码，然后运行：

```powershell
$env:GO_SERVICE_ADMIN_USERNAME = 'admin'
$securePassword = Read-Host '输入管理员密码' -AsSecureString
$passwordPointer = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($securePassword)
try {
  $env:GO_SERVICE_ADMIN_PASSWORD = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($passwordPointer)
  D:/bin/go.exe run ./cmd/admin-seed
} finally {
  [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($passwordPointer)
  Remove-Item Env:GO_SERVICE_ADMIN_PASSWORD -ErrorAction SilentlyContinue
  Remove-Item Env:GO_SERVICE_ADMIN_USERNAME -ErrorAction SilentlyContinue
}
```

SecureString 只在当前进程短暂转换为 seed 命令需要的环境变量，不落盘；不得把密码写进 `.env`、脚本、前端或 Git。

- [ ] **Step 4: 验证管理员 seed 结果**

使用相同账号再次执行 seed 必须得到“账号已存在”错误；确认不会静默覆盖密码，也不输出明文密码。

### Task 3: 启动 Go API/worker 并验证真实用户链路

**Files:**
- Inspect only: `backend/internal/app/bootstrap.go`
- Inspect only: `frontend/wechat_game/src/services/api_client.js`
- Create: `backend/tools/admin_live_smoke.ps1`

**Interfaces:**
- Consumes: 已迁移数据库、管理员账号、本机 MySQL/Redis 和现有 API 路由。
- Produces: 可重复执行的无密钥 live smoke，验证健康检查、管理员登录、真实用户列表和小程序生产 API 地址。

- [ ] **Step 1: 写 live smoke 的失败版本**

创建 PowerShell 脚本，接受 `-BaseUrl` 参数，默认 `http://127.0.0.1:8080`；不接受密码参数，密码通过当前环境变量读取；先请求 `/health`，再 POST `/api/v1/admin/auth/login`，然后用 access token GET `/api/v1/admin/users?page=1&page_size=20`，只输出 HTTP 状态、items 数量和 stats，不输出 token 或用户敏感字段。

- [ ] **Step 2: 在 API 未启动时运行并确认失败原因清晰**

运行：

```powershell
Set-Location D:\微信小游戏\backend
./tools/admin_live_smoke.ps1
```

预期：明确提示 API 未监听或管理员环境变量未设置，不打印密码。

- [ ] **Step 3: 启动 API 和 worker**

在两个 PowerShell 窗口分别运行：

```powershell
Set-Location D:\微信小游戏\backend
D:/bin/go.exe run ./cmd/api
```

```powershell
Set-Location D:\微信小游戏\backend
D:/bin/go.exe run ./cmd/worker
```

API 启动日志不得包含 AppSecret、JWT secret、数据库密码或 token。

- [ ] **Step 4: 运行 live smoke 并验证真实数据库响应**

设置管理员环境变量后运行脚本；预期 `/health` 为 200、登录为 200、用户列表为 200，返回 `items` 和 `stats` 字段。若数据库为空，返回空列表是有效结果；不得注入 mock 用户。

- [ ] **Step 5: 本地浏览器验收**

启动前端：

```powershell
Set-Location D:\微信小游戏\frontend\admin-vue
npm run dev
```

打开 `http://127.0.0.1:9528/admin/`，输入 seed 设置的密码，验证真实用户列表、搜索、平台/状态筛选、详情抽屉、审核状态和启用/禁用操作。

### Task 4: 验证小程序生产用户数据和生产发布链路

**Files:**
- Inspect only: `frontend/wechat_game/src/services/api_client.js`
- Inspect only: `.github/workflows/production-deploy.yml`
- Inspect only: `backend/deployments/Dockerfile.production`
- Inspect only: `backend/deployments/docker-compose.production.yml`
- Inspect only: `backend/deployments/server/deploy-24-calculate`

**Interfaces:**
- Consumes: 微信小游戏现有 `https://calc-api.pdurl.cn` API 地址、CI 构建流程和生产迁移流程。
- Produces: 生产后台与小程序共享同一数据库和 API，发布后后台可见真实用户。

- [ ] **Step 1: 验证小程序 API 地址和用户登录接口**

确认 `API_BASE_URL` 为 `https://calc-api.pdurl.cn`，并确认微信登录请求为 `/api/v1/auth/wechat-login`；不得把管理 token 或 AppSecret 放进小游戏包体。

- [ ] **Step 2: 验证生产构建接入后台产物**

确认 workflow 先构建 `frontend/admin-vue/dist`，再复制到 `backend/deployments/admin-dashboard`，Dockerfile 将其复制到 `/app/admin`，Compose 设置 `GO_SERVICE_ADMIN_STATIC_DIR=/app/admin`。

- [ ] **Step 3: 发布后执行迁移、重建 API/worker 并初始化管理员**

按 `backend/deployments/PRODUCTION.md` 执行生产数据库备份、迁移、API/worker 重建和一次性 admin seed；所有密码只通过服务器环境变量输入。

- [ ] **Step 4: 生产验收**

依次访问 `/health`、`/ready`、`/admin/`；登录后台并确认能看到小程序真实注册用户。再用小程序登录一个新账号，刷新后台列表，确认用户数量、昵称审核状态和平台字段变化。

### Task 5: 全量验证和交付边界检查

**Files:**
- No production file changes expected unless a failing verification identifies a concrete defect.

- [ ] **Step 1: 运行后端全量验证**

```powershell
Set-Location D:\微信小游戏\backend
D:/bin/go.exe test ./...
D:/bin/go.exe build ./...
D:/bin/go.exe vet ./...
```

- [ ] **Step 2: 运行前端全量验证**

```powershell
Set-Location D:\微信小游戏\frontend\admin-vue
npm run test:admin
npm run lint
npm run build:prod
```

- [ ] **Step 3: 检查敏感信息和生成目录**

确认前端源码、构建产物和小程序源码不含真实 AppSecret、JWT secret、数据库密码或管理员密码；确认 `node_modules`、`dist`、日志和本地环境文件不会被提交。

- [ ] **Step 4: 检查工作区并交付**

运行 `git status --short` 和 `git diff --check`，只报告本次后台相关变更；不重置或覆盖已有小游戏改动。生产发布前明确记录迁移、seed、API 重启和真实用户验收结果。
