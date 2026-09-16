# 用户资料内容审核设计

**日期：** 2026-09-16
**范围：** `D:\微信小游戏\backend`  仅后端

## 目标

为“三火算术练习”增加服务端最终裁决的昵称和头像内容审核，覆盖微信登录首次资料、受控资料同步、头像上传以及排行榜、好友房、排位记录等公开资料输出。审核失败不能覆盖旧资料，不能泄露 AppSecret、Token、原始图片或审核敏感词。

## 方案选择

采用服务端审核层和微信官方内容安全 HTTP API：

- 文本通过微信 `cgi-bin/token` 获取服务端接口 Token，再调用 `wxa/msg_sec_check`。
- 图片通过微信图片安全接口直接提交已解码的图片字节，不先生成公开 URL；这样满足审核完成前头像不可公开访问。
- 服务端缓存微信接口 Token，接近过期时刷新；请求有超时和有限重试，遇到上游不可用默认不发布资料。
- 不采用只在本地维护关键词黑名单的方案，因为无法覆盖变形、图片内容和微信审核规则。
- 不采用要求公开临时图片地址的异步方案，因为会扩大未审核图片暴露面；保留 `pending` 状态接口语义，便于未来接入合规异步审核提供商。

## 状态和公开规则

资料审核状态统一使用：

- `approved`：允许公开展示；
- `pending`：审核中，不生成新的公开头像 URL；
- `rejected`：拒绝公开，使用安全默认资料；
- `unreviewed`：历史资料尚未整改，使用安全默认资料。

迁移会将已有资料标记为 `unreviewed`，新建资料只有通过审核才写为 `approved`。公开昵称和头像必须经过统一安全输出函数；任何非 `approved` 状态都输出“算术玩家”和安全默认头像。旧房间 Redis 快照也在每次组装响应时重新过滤，因此不需要清空整个 Redis。

## 业务流程

### 微信登录

1. 服务器使用环境变量中的 AppID/AppSecret 调用 `jscode2session`，只取 openid，不记录完整请求 URL。
2. 新用户的昵称先归一化并调用文本审核；通过后保存，拒绝或异常时使用“算术玩家”，并记录审核事件。
3. 新用户头像只接受微信安全 HTTPS 地址，按现有登录兼容规则保存；头像状态由审核结果决定，非通过状态不公开。
4. 已有用户只允许在资料仍为默认/未审核状态时进行一次受控授权同步；普通登录不能成为改名绕过入口。
5. AppSecret、微信接口 Token、access_token 和 refresh_token 不出现在日志、响应和仓库。

### 用户资料更新

- `GET /api/v1/users/me` 继续只返回公开资料 DTO，不返回内部审核字段。
- `PATCH /api/v1/users/me` 暂时禁止修改 `nickname`，返回业务码 `NICKNAME_EDIT_DISABLED`。
- 头像字段仍只接受预设头像或当前用户拥有的后端 HTTPS 头像地址。
- 客户端不能通过该接口修改金币、段位、排行榜分数或进度。

### 头像上传

1. 先校验 Bearer 身份、频率、大小、魔数、真实格式和可解码内容。
2. 处理为 256x256 WEBP 后提交审核；审核前不调用公开文件发布流程。
3. `approved` 才保存新文件、更新数据库并返回公网 URL。
4. `pending` 返回 `moderation_status: pending`，保留旧头像且不生成新公开 URL。
5. `rejected` 删除临时数据、保留旧头像，返回 `AVATAR_REJECTED`。
6. 数据库更新失败时删除已保存的新文件；任何失败都不能修改旧头像。

## 模块边界

### 内容审核模块

新增 `internal/modules/moderation`，只依赖审核提供商接口和审核事件存储接口，负责：

- Unicode NFKC、零宽字符、控制字符和空白归一化；
- 文本/图片审核结果映射；
- 统一状态、原因码、提供商 request ID；
- 结构化审计事件；
- 审核上游错误的 fail-closed 行为。

### 微信平台客户端

新增 `internal/platform/wechat/content_safety.go`，负责微信接口协议、服务端 Token 缓存和脱敏错误处理。`Client` 不向调用者暴露 AppSecret；测试使用 `httptest.Server` 验证请求参数、响应错误和日志脱敏。

### 用户模块

用户服务注入审核接口，负责把审核决定和资料更新绑定起来。`ProfileResponse` 可保留内部状态字段但使用 `json:"-"`，供好友房和玩家服务安全输出，不能改变对外资料响应结构。

### 玩家公开输出

排行榜、排位历史、好友房、匹配对手和好友对战历史统一调用安全资料函数。SQL 查询返回审核状态，Redis 房间响应再次过滤快照；当前用户和对手都必须过滤。

## 数据库设计

新增 Goose 迁移 `00013_create_user_moderation.sql`：

```text
users.nickname_moderation_status VARCHAR(16) NOT NULL DEFAULT 'unreviewed'
users.avatar_moderation_status   VARCHAR(16) NOT NULL DEFAULT 'unreviewed'
users.moderation_updated_at      DATETIME(3) NULL

user_moderation_events
- id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT
- user_id BIGINT UNSIGNED NOT NULL
- resource_type VARCHAR(16) NOT NULL
- resource_id VARCHAR(128) NOT NULL DEFAULT ''
- source VARCHAR(32) NOT NULL
- moderation_status VARCHAR(16) NOT NULL
- reason_code VARCHAR(64) NOT NULL DEFAULT ''
- provider_request_id VARCHAR(128) NOT NULL DEFAULT ''
- created_at DATETIME(3) NOT NULL
```

事件表不保存完整昵称原文或图片。对用户和时间建立索引；用户外键使用既有删除策略。迁移必须可回滚，生产发布前先执行 `goose up` 并确认版本。

SQLC 的 User 和排行榜行模型同步增加状态字段；已有调用不提供状态时使用数据库默认值，生产代码显式传递审核状态。

## 业务错误

在现有 `AppError` 中增加可选字符串业务码，保持历史数字错误码兼容。错误响应仍然是：

```json
{"code":0,"message":"success","data":{}}
```

审核相关稳定业务码：

- `NICKNAME_EDIT_DISABLED`
- `NICKNAME_REJECTED`
- `AVATAR_REJECTED`
- `CONTENT_REJECTED`
- `MODERATION_PENDING`
- `MODERATION_PROVIDER_UNAVAILABLE`

敏感词、微信原始审核响应和内部原因只写脱敏结构化日志/受限事件字段，不返回给客户端。

## 历史整改

新增后端维护命令 `cmd/moderation-cleanup`，支持 `--dry-run`，默认逐条扫描用户资料并调用审核服务：

- 违规昵称替换为“算术玩家”，头像状态设为 `rejected`；
- 通过的资料更新为 `approved`；
- 审核不可用的资料保持 `unreviewed/pending`，不强制公开；
- 账号、进度、金币、对战和排位记录不删除；
- 每次运行输出扫描、通过、替换、头像撤下、缓存处理和失败数量；
- 使用用户 ID、当前资料值和状态的幂等更新，不产生奖励；
- 当前没有独立公开资料缓存，命令只清理审核模块自有缓存并报告数量，不执行全库 Redis 清空。

## 配置

沿用服务端微信配置，并新增可选审核超时/重试配置：

```text
GO_SERVICE_WECHAT_APP_ID=wx1e7ac815548c561c
GO_SERVICE_WECHAT_APP_SECRET=<server-only-secret>
GO_SERVICE_MODERATION_TIMEOUT=5s
GO_SERVICE_MODERATION_MAX_RETRIES=1
GO_SERVICE_AVATAR_STORAGE_DIR=/var/lib/24-calculate/avatars
GO_SERVICE_AVATAR_PUBLIC_BASE_URL=https://calc-api.pdurl.cn
```

生产校验继续要求真实微信 AppSecret、HTTPS 头像基址、JWT 和数据库/Redis 密钥。示例配置只放占位符，不放真实密钥。

## 测试策略

新增单元和路由测试验证：

1. NFKC 和零宽字符归一化；
2. 敏感昵称不落库，首次微信授权拒绝时使用默认昵称；
3. 普通昵称 PATCH 返回 `NICKNAME_EDIT_DISABLED`；
4. 审核通过、拒绝、pending、上游超时的头像行为；
5. 头像失败不覆盖旧资料；
6. 排行榜、好友房、排位历史不返回非 approved 资料；
7. 微信接口错误和日志不泄露 AppSecret/Token；
8. 历史整改 `--dry-run` 和重复执行结果稳定；
9. 现有 Go 单元测试、`go vet`、`go build` 和 `git diff --check` 全部通过。

真机微信登录、头像上传和生产头像 URL 访问属于部署后的人工验收，不在本地单元测试中伪造通过。

## 不在本次范围

- 不修改 `frontend`、Godot 或前端提示词；
- 不删除用户、进度、金币、对战/排位历史；
- 不改造已有排行榜、好友房或游戏结算协议；
- 不把本地关键词过滤当作官方审核替代；
- 不自动推送或合并 GitHub，完成测试后再单独决定集成。
