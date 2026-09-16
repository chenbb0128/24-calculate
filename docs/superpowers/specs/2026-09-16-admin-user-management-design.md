# 小程序用户管理后台接入设计

## 1. 目标与范围

把 `frontend/admin` 的演示数据接入真实 Go 后端，形成可用的管理员用户管理 MVP：

- 独立管理员账号登录；
- JWT 携带管理员角色并由后端强制校验；
- 用户分页列表、搜索、平台/状态筛选；
- 用户详情查看；
- 启用和禁用用户；
- 前端保留现有详情抽屉、批量操作、确认弹窗和响应式 UI；
- 保留并验证已有微信昵称/头像内容安全审核链路。

本阶段不包含管理员账号管理、密码找回、细粒度权限配置、审核运营工作台和统计报表扩展。

## 2. 部署与边界

生产环境由同一个反向代理提供后台页面和 API：

- 后台页面：`/admin/`；
- API：`/api/v1/admin/*`；
- 前端使用相对路径调用 API，不依赖生产 CORS；
- 管理员使用 Bearer JWT，令牌保存在当前浏览器会话范围内；
- 微信 AppSecret、TapTap 密钥、数据库密码和 JWT 密钥只存在后端环境变量，不进入前端包体、响应或日志。

本地开发仍可用静态 HTTP 服务启动 `frontend/admin`，通过可配置的 API base URL 连接本地 Go API。

## 3. 管理员身份与 JWT

### 3.1 管理员数据

新增迁移 `00014_create_admin_accounts.sql`，创建 `admin_accounts`：

- `id`：自增主键；
- `username`：唯一，长度 3–64；
- `password_hash`：bcrypt 哈希；
- `role`：当前固定为 `admin`，为后续角色扩展保留字段；
- `status`：`1` 启用、`0` 禁用；
- `last_login_at`：最近成功登录时间，可为空；
- `created_at`、`updated_at`。

迁移不写入任何默认账号或默认密码。

### 3.2 首个管理员

新增 `cmd/admin-seed`：

- 从 `GO_SERVICE_ADMIN_USERNAME` 和 `GO_SERVICE_ADMIN_PASSWORD` 读取账号密码；
- 使用 bcrypt 生成哈希后写入数据库；
- 账号已存在时失败，不静默覆盖密码；
- 密码不写日志，不作为命令行参数传递；
- 命令只创建管理员，不依赖微信登录或前端页面。

### 3.3 JWT claims 与中间件

在现有 claims 增加 `role`：

- 普通登录和开发登录签发 `role=user`；
- 管理员登录签发 `role=admin`；
- 旧的普通用户 token 在过渡期允许缺少 `role`，按普通用户处理；
- `RequireUser` 只允许普通用户角色；
- `RequireAdmin` 只允许 `admin` 角色；
- 管理员 token 不能访问 `/users/me`、玩家进度、排行榜等普通用户接口。

管理员登录、刷新、退出沿用现有 JWT 签名和 refresh token 一次性消费机制。refresh token 中的主体 ID 根据 `role` 分别从 `admin_accounts` 或 `users` 查询，避免把管理员 ID 当成游戏用户。

管理员登录接口：

```text
POST /api/v1/admin/auth/login
POST /api/v1/admin/auth/refresh
POST /api/v1/admin/auth/logout
```

登录错误不区分“账号不存在”和“密码错误”；登录按客户端 IP 限流；禁用管理员不能登录或刷新。

## 4. 管理员用户 API

### 4.1 用户列表

```text
GET /api/v1/admin/users
```

查询参数：

- `q`：匹配用户 ID、用户名或当前公开昵称；
- `status`：`all`、`active`、`disabled`；
- `platform`：`all`、`wechat`、`taptap`、`password`；
- `page`：从 1 开始；
- `page_size`：服务端限制最大值。

返回当前用户资料、平台、账号状态、创建时间、更新时间及审核状态。列表和统计使用服务端筛选与分页，避免前端只过滤当前页导致结果不准确。

响应数据形状：

```json
{
  "items": [],
  "page": 1,
  "page_size": 20,
  "total": 0,
  "stats": {
    "total": 0,
    "new_today": 0,
    "active": 0,
    "disabled": 0
  }
}
```

平台由 `user_identities.provider` 推导；没有第三方身份的账号标记为 `password`。查询按创建时间倒序、用户 ID 作为稳定次序。

### 4.2 用户详情

```text
GET /api/v1/admin/users/:id
```

返回后台需要的当前资料、账号状态、平台身份摘要和内容审核状态：

- `nickname_moderation_status`；
- `avatar_moderation_status`；
- `moderation_updated_at`；
- 当前安全展示昵称和头像；
- 创建、更新时间。

不返回原始被拒昵称、原始上传图片、微信 AppSecret、微信 access token、provider 原始响应或审核请求敏感信息。审核事件仍由现有 `user_moderation_events` 保存不含原始资料的审计记录。

### 4.3 启用与禁用

```text
POST /api/v1/admin/users/:id/disable
POST /api/v1/admin/users/:id/enable
```

操作要求：

- 目标用户不存在返回统一 404；
- 重复启用/禁用保持幂等，返回当前状态；
- 禁用阻止后续普通登录、refresh 和普通用户接口访问；
- 启用清除账号级阻断标记；
- 账号级阻断标记使用 Redis，并以 access token TTL 为上限，配合数据库状态作为最终事实来源；
- 操作不修改昵称、头像或其审核状态；
- 后台自己的管理员 token 仍严格按 `admin` 角色校验。

## 5. 内容审核兼容性

后台接入不得绕过既有 moderation service：

- 微信昵称和头像继续由微信内容安全接口审核；
- `NICKNAME_REJECTED`、`AVATAR_REJECTED`、`MODERATION_PENDING`、`NICKNAME_EDIT_DISABLED` 业务码保持不变；
- 审核拒绝、待审核、上游不可用均不能覆盖当前已保存的安全资料；
- 历史违规昵称清理命令继续负责替换违规公开昵称并清理相关缓存；
- 后台详情显示当前安全资料和审核状态，不显示被拒原文；
- 后台状态操作不改变审核事件和安全默认值。

## 6. 后端模块边界

新增 `internal/modules/admin`，分为以下职责：

- `auth_service.go`：管理员账号校验、token pair、refresh、logout；
- `user_service.go`：后台用户列表、详情、状态变更；
- `repository.go`：管理员和后台用户 SQL 查询；
- `handler.go`、`routes.go`、`dto.go`：HTTP 入参、响应和路由；
- `errors.go`：后台错误映射。

现有 `auth`、`user`、`moderation` 模块只做必要的接口扩展，不复制审核逻辑，不把后台账号塞进玩家用户表。

## 7. 前端接入

`frontend/admin/app.js` 增加真实 API adapter：

- 登录页或未认证状态显示管理员登录表单；
- 登录成功保存当前会话 token，页面刷新后使用 refresh token 恢复会话；
- 列表筛选、分页、刷新调用真实接口；
- 详情抽屉调用详情接口；
- 启用/禁用调用对应状态接口；
- 批量禁用逐项调用状态接口并汇总成功/失败结果；
- 收到 401 清理会话并回到登录态；
- 后端审核状态和错误业务码转换为明确的管理员提示；
- 去除生产路径对 `SEED_USERS` 和 `localStorage` 用户状态的依赖，但保留纯函数单元测试和必要的离线回退测试。

前端不包含任何 AppSecret，也不把原始审核资料写入浏览器存储。

## 8. 错误与安全响应

沿用现有统一响应格式、数字错误码、`request_id` 和 `data: null`。新增后台接口至少覆盖：

- 未登录：401；
- token 过期/无效：401；
- 角色不符：403；
- 用户不存在：404；
- 状态参数非法：400；
- 数据库或 Redis 不可用：503。

错误详情、日志和测试输出均不得包含密码、AppSecret、JWT 原文、微信 access token 或原始违规资料。

## 9. 测试与验收

后端：

- JWT role claim、旧用户 token 兼容、RequireUser/RequireAdmin；
- 管理员 seed 的密码哈希、重复账号拒绝和缺少环境变量；
- 管理员登录、刷新、退出、禁用账号；
- 列表筛选、分页、统计、平台推导；
- 详情审核状态和敏感字段不泄露；
- 启用/禁用幂等及账号级阻断；
- 不回归四个审核业务码和“失败不覆盖原资料”；
- 数据库迁移和 sqlc 生成结果一致；
- `D:/bin/go.exe test ./...` 全量通过。

前端：

- `node frontend/admin/tools/smoke_test.js`；
- `node --check frontend/admin/app.js`；
- 登录、401 回登录、服务端筛选分页、详情和状态操作的 adapter 测试；
- 保留现有无 `innerHTML`、无 `eval` 的安全渲染约束。

完成前核对：数据库迁移已执行，AppSecret 只在后端环境变量，历史违规昵称清理和缓存清理链路未被破坏，且 Go 全量测试通过。
