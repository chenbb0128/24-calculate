# 小程序用户管理后台接入 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 frontend/admin 演示后台接入真实 Go API，完成独立管理员登录、角色授权、用户列表/详情/启停管理，并保持现有内容安全审核链路不回归。

**Architecture:** 新增独立 admin 业务模块和 admin_accounts 表；现有 JWT 增加 role claim，普通用户和管理员复用同一签名与 refresh 机制，由 RequireUser/RequireAdmin 隔离路由。后台用户查询使用服务端分页、筛选和审核状态只读返回，前端使用同域 API adapter 替换 localStorage demo adapter。

**Tech Stack:** Go 1.26、Gin、MySQL 8、Redis、sqlc、Goose、JWT HS256、bcrypt、原生 HTML/CSS/JavaScript。

**Spec:** docs/superpowers/specs/2026-09-16-admin-user-management-design.md

## Global Constraints

- 生产后台页面使用 /admin/，后台 API 使用 /api/v1/admin/*，前端通过相对路径调用 API。
- AppSecret、TapTap 密钥、数据库密码、JWT 密钥、密码和 JWT 原文不得进入前端包体、业务响应、日志或测试输出。
- 管理员账号必须存放在独立 admin_accounts 表，迁移不得写入默认账号或默认密码。
- bcrypt 密码只从 GO_SERVICE_ADMIN_PASSWORD 读取，不作为命令行参数传递。
- 普通用户新 token 使用 role=user，管理员 token 使用 role=admin；旧的缺少 role 的普通用户 token 只允许按普通用户兼容处理。
- 管理员 token 不能访问普通用户、玩家进度、排行榜或用户资料接口。
- 微信昵称和头像继续经过微信内容安全审核；NICKNAME_REJECTED、AVATAR_REJECTED、MODERATION_PENDING、NICKNAME_EDIT_DISABLED 必须保持不变。
- 审核失败、待审核或上游不可用不能覆盖原资料；历史违规昵称清理命令必须继续替换公开昵称并清理缓存。
- 后台详情只返回当前安全资料和审核状态，不返回原始被拒昵称、原始图片、微信 access token 或审核 provider 敏感字段。
- 所有 HTTP 响应继续使用统一响应格式、数字错误码、request_id 和 data: null。
- 每个后端任务完成后运行对应 Go 测试；最终必须运行 D:/bin/go.exe test ./...、node frontend/admin/tools/smoke_test.js 和 node --check frontend/admin/app.js。
- 不使用 git reset --hard、git checkout -- 或覆盖工作区已有用户改动；只提交当前任务明确修改的文件。

---

### Task 1: Extend JWT claims and role-aware authentication middleware

**Files:**
- Modify: backend/internal/platform/jwt/claims.go
- Modify: backend/internal/platform/jwt/manager.go
- Modify: backend/internal/platform/jwt/manager_test.go
- Modify: backend/internal/http/middleware/auth.go
- Modify: backend/internal/http/middleware/auth_test.go
- Modify: backend/internal/modules/user/routes.go
- Modify: backend/internal/modules/player/routes.go

**Interfaces:**
- Consumes: existing jwt.Manager, Claims, RequireAuth, AccessTokenRevocationChecker.
- Produces: jwt.RoleUser, jwt.RoleAdmin, Manager.IssueAccessTokenWithRole, Manager.IssueRefreshTokenWithRole, middleware.RequireUser, middleware.RequireAdmin.

- [ ] **Step 1: Write failing JWT role tests**

Add table-driven tests to manager_test.go that assert:

~~~go
token, claims, err := manager.IssueAccessTokenWithRole(7, jwt.RoleAdmin)
if err != nil || claims.Role != jwt.RoleAdmin {
    t.Fatalf("IssueAccessTokenWithRole() claims = %+v, err = %v", claims, err)
}
parsed, err := manager.ParseAccessToken(token)
if err != nil || parsed.Role != jwt.RoleAdmin || parsed.UserID != 7 {
    t.Fatalf("ParseAccessToken() = %+v, err = %v", parsed, err)
}
~~~

Also assert that a token with an empty role remains parseable for the legacy ordinary-user migration path, while a role other than user or admin is rejected by the role-specific middleware.

- [ ] **Step 2: Run the focused JWT tests and verify failure**

Run:

~~~powershell
cd D:\微信小游戏\backend
D:/bin/go.exe test ./internal/platform/jwt -run 'TestManager.*Role|TestManager.*Legacy' -count=1
~~~

Expected: FAIL because Claims.Role and the role-aware issue methods do not exist yet.

- [ ] **Step 3: Implement role-aware claims and issue methods**

Add Role string json:"role" to Claims, define RoleUser and RoleAdmin, and implement role-aware issue methods. Keep existing IssueAccessToken(userID) and IssueRefreshToken(userID) as wrappers that issue role=user, so existing ordinary auth call sites remain source-compatible. Keep parsing backward-compatible for an empty role; role enforcement belongs to middleware.

- [ ] **Step 4: Write failing middleware role tests**

Extend auth_test.go with requests that use:
- a role=user token against RequireAdmin, expecting HTTP 403;
- a role=admin token against RequireUser, expecting HTTP 403;
- a legacy empty-role token against RequireUser, expecting the handler to run;
- a valid admin token against RequireAdmin, expecting the handler to run.

- [ ] **Step 5: Implement role-specific middleware and switch player routes**

Refactor shared bearer parsing and revocation checks into an internal helper. Implement RequireUser and RequireAdmin without removing RequireAuth; RequireUser accepts role=user and the empty legacy role, while RequireAdmin accepts only role=admin. Change user.RegisterRoutes and player.RegisterRoutes to use RequireUser, preventing an admin token from entering ordinary user handlers.

- [ ] **Step 6: Run focused tests and commit**

Run:

~~~powershell
cd D:\微信小游戏\backend
D:/bin/go.exe test ./internal/platform/jwt ./internal/http/middleware ./internal/modules/user ./internal/modules/player -count=1
~~~

Expected: PASS. Commit only the JWT, middleware, and route files:

~~~powershell
git add backend/internal/platform/jwt backend/internal/http/middleware/auth.go backend/internal/http/middleware/auth_test.go backend/internal/modules/user/routes.go backend/internal/modules/player/routes.go
git commit -m "feat: add role-aware jwt authorization"
~~~

### Task 2: Add admin account migration, sqlc queries, repository, and seed command

**Files:**
- Create: backend/database/migrations/00014_create_admin_accounts.sql
- Create: backend/database/queries/admin_accounts.sql
- Create: backend/internal/modules/admin/repository.go
- Create: backend/internal/modules/admin/seed.go
- Create: backend/internal/modules/admin/seed_test.go
- Create: backend/cmd/admin-seed/main.go
- Modify: generated files under backend/internal/store/sqlc/ via sqlc generate
- Modify: backend/README.md
- Modify: backend/.env.example

**Interfaces:**
- Consumes: MySQL config and store conventions from internal/store, bcrypt from golang.org/x/crypto/bcrypt, generated sqlc types.
- Produces: AdminAccountStore, AdminRepository, SeedAdmin(ctx, store, username, password) error, and the admin_accounts schema consumed by the admin auth service.

- [ ] **Step 1: Write the migration and query contract tests first**

Create seed_test.go with an in-memory fake store that verifies:

~~~go
err := SeedAdmin(context.Background(), fake, "root-admin", "correct-horse-battery-staple")
if err != nil {
    t.Fatal(err)
}
if fake.created.Username != "root-admin" || fake.created.Role != "admin" || fake.created.Status != 1 {
    t.Fatalf("created = %+v", fake.created)
}
if bcrypt.CompareHashAndPassword([]byte(fake.created.PasswordHash), []byte("correct-horse-battery-staple")) != nil {
    t.Fatal("password was not bcrypt-hashed")
}
~~~

Add cases for empty username/password, username outside 3–64 characters, password outside 8–72 bytes, and duplicate username. The tests must assert that the plain password never appears in the stored hash error or returned error text.

- [ ] **Step 2: Run the seed tests and verify failure**

Run:

~~~powershell
cd D:\微信小游戏\backend
D:/bin/go.exe test ./internal/modules/admin -run 'TestSeedAdmin' -count=1
~~~

Expected: FAIL because the admin package and seed function are not present.

- [ ] **Step 3: Add migration and sqlc query files**

Create 00014_create_admin_accounts.sql with admin_accounts columns id, username, password_hash, role, status, last_login_at, created_at, and updated_at; add a unique username key and a down migration that drops only this table. Do not insert data.

Create admin_accounts.sql queries for:
- GetAdminAccountByID;
- GetAdminAccountByUsername;
- CreateAdminAccount;
- TouchAdminLastLogin.

Use the existing sqlc MySQL configuration and regenerate internal/store/sqlc rather than hand-editing generated code.

- [ ] **Step 4: Implement repository and seed service**

Implement AdminRepository around generated db.Queries. Implement SeedAdmin with exact validation, bcrypt.GenerateFromPassword(..., bcrypt.DefaultCost), fixed role admin, active status 1, UTC timestamps, and duplicate-entry mapping. The seed service must refuse an existing username instead of updating it.

- [ ] **Step 5: Implement the thin cmd/admin-seed entrypoint**

Read GO_SERVICE_ADMIN_USERNAME and GO_SERVICE_ADMIN_PASSWORD with os.LookupEnv, load database configuration using the existing config loader, open MySQL, construct admin.NewRepository, call SeedAdmin, and exit non-zero on any error. Never log the password or the full DSN.

- [ ] **Step 6: Document seed usage and run tests**

Document this exact PowerShell flow in backend/README.md and backend/.env.example without real credentials:

~~~powershell
$env:GO_SERVICE_ADMIN_USERNAME = "admin"
$env:GO_SERVICE_ADMIN_PASSWORD = "replace-with-a-strong-password"
D:/bin/go.exe run ./cmd/admin-seed
~~~

Run:

~~~powershell
cd D:\微信小游戏\backend
D:/bin/go.exe test ./internal/modules/admin ./internal/store/sqlc -count=1
~~~

Commit the migration, query, repository, seed, generated code, and documentation:

~~~powershell
git add backend/database/migrations/00014_create_admin_accounts.sql backend/database/queries/admin_accounts.sql backend/internal/modules/admin backend/cmd/admin-seed backend/internal/store/sqlc backend/README.md backend/.env.example
git commit -m "feat: add isolated admin accounts and seed command"
~~~

### Task 3: Add admin authentication service and routes

**Files:**
- Create: backend/internal/modules/admin/auth_service.go
- Create: backend/internal/modules/admin/auth_dto.go
- Create: backend/internal/modules/admin/auth_handler.go
- Create: backend/internal/modules/admin/auth_routes.go
- Create: backend/internal/modules/admin/auth_service_test.go
- Create: backend/internal/modules/admin/auth_handler_test.go
- Modify: backend/internal/platform/redis/client.go only where the existing refresh-token interface needs an adapter
- Modify: backend/internal/modules/auth/errors.go only if a shared invalid-token mapping is required

**Interfaces:**
- Consumes: AdminRepository, jwt.Manager.Issue*TokenWithRole, existing refresh TokenStore, bcrypt, and middleware.RequireAdmin.
- Produces: POST /api/v1/admin/auth/login, /refresh, /logout and AdminAuthHandler for bootstrap wiring.

- [ ] **Step 1: Write service tests for login, refresh, and logout**

Use fakes for the admin store and refresh token store. Cover:

~~~go
pair, err := service.Login(ctx, LoginInput{Username: "admin", Password: "secret-pass"}, "127.0.0.1")
if err != nil {
    t.Fatal(err)
}
claims, err := manager.ParseAccessToken(pair.AccessToken)
if err != nil || claims.Role != jwt.RoleAdmin {
    t.Fatalf("claims = %+v, err = %v", claims, err)
}
~~~

Also assert wrong password and missing username return the same invalid-credentials error, disabled admins cannot log in or refresh, refresh tokens are consumed once, and logout revokes the supplied refresh token.

- [ ] **Step 2: Run the service tests and verify failure**

Run:

~~~powershell
cd D:\微信小游戏\backend
D:/bin/go.exe test ./internal/modules/admin -run 'TestAdminAuth' -count=1
~~~

Expected: FAIL because the admin auth service does not exist.

- [ ] **Step 3: Implement admin auth service**

Implement AdminAuthService with methods:

~~~go
Login(ctx context.Context, input LoginInput, ip string) (auth.TokenResponse, error)
Refresh(ctx context.Context, input auth.RefreshInput) (auth.TokenResponse, error)
Logout(ctx context.Context, input auth.LogoutInput) error
~~~

Use the existing IP rate-limit store, compare bcrypt hashes, reject status 0, issue a user-independent JWT pair with role=admin, save/consume refresh JTIs using the existing one-time store, and update last_login_at only after successful credential validation. Do not copy WeChat or player login logic into this service.

- [ ] **Step 4: Add handler and routes**

Bind JSON using the existing response/error helpers. Register the three routes under /admin/auth; login is public, refresh is public but token-validated, and logout validates the submitted refresh token. Return the existing TokenResponse shape so the frontend adapter can share token handling.

- [ ] **Step 5: Run handler tests and commit**

Run:

~~~powershell
cd D:\微信小游戏\backend
D:/bin/go.exe test ./internal/modules/admin -run 'TestAdminAuth|TestAdminHandler' -count=1
~~~

Commit:

~~~powershell
git add backend/internal/modules/admin backend/internal/platform/redis/client.go backend/internal/modules/auth/errors.go
git commit -m "feat: add admin authentication endpoints"
~~~

### Task 4: Add admin user list, detail, and status management

**Files:**
- Create: backend/database/queries/admin_users.sql
- Create: backend/internal/modules/admin/user_dto.go
- Create: backend/internal/modules/admin/user_service.go
- Create: backend/internal/modules/admin/user_handler.go
- Create: backend/internal/modules/admin/user_routes.go
- Create: backend/internal/modules/admin/user_service_test.go
- Create: backend/internal/modules/admin/user_handler_test.go
- Modify: backend/internal/store/sqlc via sqlc generate
- Modify: backend/internal/platform/redis/keys.go
- Modify: backend/internal/platform/redis/client.go

**Interfaces:**
- Consumes: generated admin user list/detail/status queries, RequireAdmin, moderation.Status, user.SafePublicNickname, user.SafePublicAvatar, and Redis.
- Produces: GET /api/v1/admin/users, GET /api/v1/admin/users/:id, POST /api/v1/admin/users/:id/disable, POST /api/v1/admin/users/:id/enable.

- [ ] **Step 1: Write failing repository/service tests**

Add fake-store tests for:
- q, status, platform, page, and page_size being normalized and passed to the repository;
- platform precedence wechat, then taptap, then password when identities are present;
- stats fields being returned;
- detail returning moderation statuses but not an attempted/original profile field;
- repeated enable/disable returning the current state without changing profile fields;
- missing user mapping to 404;
- disabling calling BlockAccount, enabling calling UnblockAccount.

- [ ] **Step 2: Run the focused tests and verify failure**

Run:

~~~powershell
cd D:\微信小游戏\backend
D:/bin/go.exe test ./internal/modules/admin -run 'TestAdminUser' -count=1
~~~

Expected: FAIL because the admin user service and query types do not exist.

- [ ] **Step 3: Add sqlc admin-user queries**

Create admin_users.sql queries for a paginated list, total count, summary stats, detail, and a status update. The list must left join user_identities, use server-side filters, exclude raw moderation payloads, order by created_at DESC, id DESC, and use bounded LIMIT/OFFSET. The detail query must include nickname_moderation_status, avatar_moderation_status, and moderation_updated_at.

- [ ] **Step 4: Add account-block Redis keys and methods**

Add:

~~~go
func AccountBlockedKey(role string, id uint64) string
func (c *Client) BlockAccount(ctx context.Context, role string, id uint64, ttl time.Duration) error
func (c *Client) IsAccountBlocked(ctx context.Context, role string, id uint64) (bool, error)
func (c *Client) UnblockAccount(ctx context.Context, role string, id uint64) error
~~~

Use a role-qualified key and a TTL no longer than the configured access token TTL. Never place a username, password, profile text, or token value in the key.

- [ ] **Step 5: Implement service, DTOs, handlers, and routes**

Use separate response types for list and detail. Convert database timestamps to RFC3339. Return current safe nickname/avatar using the existing moderation-aware public helpers, while including only the moderation status fields intended for administrators. Status endpoints must update only users.status and updated_at; they must not call profile update code.

Register all user-management routes under an admin group protected by RequireAdmin and the existing access-token revocation checker plus account-block checker.

- [ ] **Step 6: Run tests and commit**

Run:

~~~powershell
cd D:\微信小游戏\backend
D:/bin/go.exe test ./internal/modules/admin ./internal/platform/redis -count=1
~~~

Commit:

~~~powershell
git add backend/database/queries/admin_users.sql backend/internal/modules/admin backend/internal/store/sqlc backend/internal/platform/redis/keys.go backend/internal/platform/redis/client.go
git commit -m "feat: add admin user management APIs"
~~~

### Task 5: Wire the admin module into the API and update API documentation

**Files:**
- Modify: backend/internal/app/bootstrap.go
- Modify: backend/internal/http/router_test.go
- Modify: backend/internal/http/openapi_test.go
- Modify: backend/docs/openapi.yaml
- Create: backend/internal/modules/admin/routes_test.go

**Interfaces:**
- Consumes: AdminRepository, AdminAuthService, admin user service/handler, JWT manager, Redis client, and httpapi.RouterOptions.APIRoutes.
- Produces: a running API with all /api/v1/admin/* routes registered and no route reachable with a player token.

- [ ] **Step 1: Add failing route registration tests**

Build a test router with fake services and assert:
- POST /api/v1/admin/auth/login is registered;
- GET /api/v1/admin/users with no bearer returns 401;
- the same request with a user token returns 403;
- the same request with an admin token reaches the handler;
- unknown admin paths retain the standard JSON 404 envelope.

- [ ] **Step 2: Run route tests and verify failure**

Run:

~~~powershell
cd D:\微信小游戏\backend
D:/bin/go.exe test ./internal/http ./internal/modules/admin -run 'Test.*Route|Test.*Admin' -count=1
~~~

Expected: FAIL because BootstrapAPI does not construct or register the admin module.

- [ ] **Step 3: Wire dependencies in BootstrapAPI**

Construct the admin repository from the existing db.Queries, tx manager, database and Redis client; construct admin auth and user services; register admin routes after ordinary auth routes. Keep WeChat content moderation construction unchanged and do not move AppSecret handling into the admin package.

- [ ] **Step 4: Document OpenAPI and run route tests**

Add admin login, refresh, logout, list, detail, enable, and disable paths to backend/docs/openapi.yaml. Document the Bearer role requirement, pagination parameters, moderation status fields, and the four unchanged moderation business codes without exposing credentials or provider tokens.

Run:

~~~powershell
cd D:\微信小游戏\backend
D:/bin/go.exe test ./internal/http ./internal/modules/admin -count=1
~~~

Commit:

~~~powershell
git add backend/internal/app/bootstrap.go backend/internal/http/router_test.go backend/internal/http/openapi_test.go backend/internal/modules/admin/routes_test.go backend/docs/openapi.yaml
git commit -m "feat: register admin API routes"
~~~

### Task 6: Replace admin frontend demo adapter with real API integration

**Files:**
- Modify: frontend/admin/index.html
- Create: frontend/admin/api.js
- Modify: frontend/admin/app.js
- Modify: frontend/admin/styles.css
- Modify: frontend/admin/README.md
- Modify: frontend/admin/tools/smoke_test.js
- Create: frontend/admin/tools/api_adapter_test.js

**Interfaces:**
- Consumes: backend token response, paginated list response, detail response, status endpoints, and existing AdminCore pure functions.
- Produces: window.AdminApi, a login gate, authenticated list/detail/status flows, and the same UI behavior without production SEED_USERS/localStorage state.

- [ ] **Step 1: Write failing API adapter tests**

Use a fake fetch implementation to assert exact requests:

~~~js
const api = createAdminApi({ baseUrl: '/api/v1/admin', fetchImpl: fakeFetch });
await api.login('admin', 'secret-pass');
assert.equal(calls[0].url, '/api/v1/admin/auth/login');
assert.equal(calls[0].options.method, 'POST');
~~~

Cover list query serialization, bearer headers, 401 session clearing, detail lookup, enable/disable URLs, and mapping NICKNAME_REJECTED, AVATAR_REJECTED, MODERATION_PENDING, and NICKNAME_EDIT_DISABLED to stable UI error text.

- [ ] **Step 2: Run the adapter test and verify failure**

Run:

~~~powershell
node frontend/admin/tools/api_adapter_test.js
~~~

Expected: FAIL because api.js and createAdminApi do not exist.

- [ ] **Step 3: Implement frontend/admin/api.js**

Implement createAdminApi({ baseUrl, fetchImpl, storage }) with login, refresh, logout, listUsers, getUser, disableUser, and enableUser. Keep access/refresh tokens in session-scoped storage only; never persist profile payloads or secrets. On a 401, attempt one refresh for an authenticated request, then clear tokens and throw a typed authentication error.

- [ ] **Step 4: Add login gate and connect the existing UI**

Add a login form to index.html. Refactor the initial synchronous boot in app.js into an authenticated async boot:
1. restore a session or show login;
2. request server list data with current filters and page;
3. render stats and rows from the server response;
4. load detail data when opening the drawer;
5. call status endpoints after confirmation;
6. reload the affected list/detail state and show a toast.

Keep existing safe DOM construction, focus return, Escape behavior, confirmation dialog, and batch operation UX. Batch disable may call disableUser sequentially, collect failures, and report partial completion without mutating unconfirmed rows locally.

- [ ] **Step 5: Add login/loading/error styles and update smoke tests**

Style the login gate, loading state, API error state, and expired-session prompt without changing the established responsive layout. Update smoke tests to assert no production path reads SEED_USERS or writes user statuses to localStorage, while retaining pure-function tests for filtering and pagination.

- [ ] **Step 6: Run frontend checks and commit**

Run:

~~~powershell
node frontend/admin/tools/api_adapter_test.js
node frontend/admin/tools/smoke_test.js
node --check frontend/admin/app.js
~~~

Commit:

~~~powershell
git add frontend/admin/index.html frontend/admin/api.js frontend/admin/app.js frontend/admin/styles.css frontend/admin/README.md frontend/admin/tools/smoke_test.js frontend/admin/tools/api_adapter_test.js
git commit -m "feat: connect admin dashboard to backend API"
~~~

### Task 7: Verify moderation compatibility, migration, and full integration

**Files:**
- Modify only if verification exposes a regression: existing moderation tests/docs under backend/internal/modules/moderation, backend/internal/modules/user, backend/internal/modules/auth, and backend/docs/backend-completion.md.
- No new feature files are expected in this task.

**Interfaces:**
- Consumes: all implementation tasks, migration 00013_create_user_moderation.sql, migration 00014_create_admin_accounts.sql, moderation cleanup command, backend and frontend test suites.
- Produces: evidence that the admin integration does not bypass moderation, leak secrets, or leave the database schema and generated code out of sync.

- [ ] **Step 1: Run targeted moderation regression tests**

Run:

~~~powershell
cd D:\微信小游戏\backend
D:/bin/go.exe test ./internal/modules/auth ./internal/modules/user ./internal/modules/moderation ./internal/platform/wechat -count=1
~~~

Expected: PASS, including the four business codes and tests that rejected or pending moderation does not overwrite the saved profile.

- [ ] **Step 2: Run source safety and migration checks**

Run the repository’s existing moderation/source scans and inspect:

~~~powershell
rg -n "GO_SERVICE_WECHAT_APP_SECRET|GO_SERVICE_TAPTAP_APP_SECRET|password_hash|access_token|appsecret|AppSecret" backend frontend/admin
Get-ChildItem backend/database/migrations | Sort-Object Name
~~~

Confirm that only backend configuration/examples mention secret variable names, no frontend file contains a credential value, migrations 00013 and 00014 are present in order, and generated sqlc code contains the admin queries.

- [ ] **Step 3: Run full Go and frontend verification**

Run:

~~~powershell
cd D:\微信小游戏\backend
D:/bin/go.exe test ./...
cd D:\微信小游戏
node frontend/admin/tools/api_adapter_test.js
node frontend/admin/tools/smoke_test.js
node --check frontend/admin/app.js
~~~

Expected: every command exits with code 0. Record any unavailable live MySQL/Redis migration execution separately; unit and integration tests must still pass with the repository’s configured test behavior.

- [ ] **Step 4: Review the final diff and commit verification docs if needed**

Run:

~~~powershell
git status --short
git diff --check HEAD~1..HEAD
git log --oneline -12
~~~

Review that existing user changes remain intact, no unrelated files were staged, and the final branch contains the design, implementation, migration, frontend adapter, and verification commits. If documentation needs an exact local startup command, update only backend/README.md or frontend/admin/README.md, rerun the relevant checks, and commit that documentation change.
