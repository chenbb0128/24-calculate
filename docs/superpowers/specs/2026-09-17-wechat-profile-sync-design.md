# 微信资料同步接口设计

## 目标

在不允许登录参数绕过审核的前提下，为已经登录的微信用户增加一次受保护的微信资料同步入口，使昵称和头像在重新授权后可以安全同步到当前账号。

## 范围与约束

- 只修改 `backend` 目录；不修改 `frontend` 或 Godot。
- 继续使用统一的 `code/message/data` 响应包装。
- AppID、AppSecret 只从服务端环境变量读取，绝不进入响应、日志或前端。
- 继续复用现有 `moderation` 服务、`users` 审核状态字段、`user_moderation_events` 审核流水和安全资料输出函数。
- 不新增数据库迁移；一次性 code 防重和接口限流使用已有 Redis。
- 不保存原始违规昵称、原始头像或微信 access token；Redis 只保存不可逆摘要和短期状态。

## 现有问题

`/api/v1/auth/wechat-login` 同时完成登录和资料同步。它为了防止登录参数改名，已经拒绝已审核正常昵称的再次修改，但这也让重新授权后的合法昵称没有独立的同步入口。该保护逻辑不能删除；登录接口需要改为只允许新账号首次资料初始化，已有账号的资料变更统一走新接口。

## 推荐架构

### 路由与职责

新增受用户认证中间件保护的路由：

```text
POST /api/v1/users/me/wechat-profile/sync
```

请求由用户模块的 handler 接收，但微信身份校验、资料审核和更新由一个独立的资料同步服务方法完成。服务方法接收中间件解析出的当前用户 ID，不接收任何客户端用户 ID。

实现边界：

- `internal/modules/user`：请求/响应 DTO、同步编排、头像下载与处理、资料更新和公开资料结果。
- `internal/modules/auth`：保留微信 code2Session 登录逻辑，并收紧已有账号的登录资料更新；为同步服务提供微信登录客户端能力。
- `internal/platform/redis`：提供按摘要的一次性 code 防重和按用户的同步限流能力。
- `internal/app/bootstrap.go`：把微信客户端、审核服务、Redis 防重/限流能力注入用户服务。
- `internal/modules/user`、`internal/modules/auth` handler/routes：注册新路由并保持统一错误包装。

### 请求处理顺序

1. `RequireUser` 校验 Bearer access token，获得当前用户 ID。
2. 校验 JSON 字段：`code` 必填，昵称和头像可为空；拒绝超长 code 和明显无效输入。
3. 调用服务端微信客户端的 `code2Session`，只取 openid，不向客户端返回微信响应或 AppSecret。
4. 通过 `provider=wechat` 和 openid 查询绑定账号；查询不到或绑定的用户 ID 与当前 token 不一致时返回 401/403，不修改资料。
5. 对 `sha256(code)` 做 Redis 原子消费。已经成功处理过的 code 直接返回“凭证已使用”，并且不会再次审核或更新资料。原始 code 永远不写入 Redis。
6. 在用户级别执行同步限流；被限流时返回 429，不调用审核上游。
7. 有昵称时先执行 `NormalizeNickname`，再调用 `ModerateText`。只有 `approved` 才允许更新昵称；`pending`、`rejected`、`unavailable` 都保留当前公开昵称。
8. 有头像时只接受微信安全头像域名（`qlogo.cn` 及其子域名）的 HTTPS URL。后端下载时禁用跨域重定向，限制响应大小，校验图片真实内容和尺寸，再裁剪为 256x256 WEBP，调用 `ModerateImage`，只有通过后才写入头像存储。
9. 昵称和头像分别计算更新结果，在一次数据库更新中保留旧资料和未通过审核的字段。新头像保存和数据库更新仍采用“先临时保存、数据库成功后发布、失败删除临时文件”的原子流程。
10. 返回当前用户的安全资料、同步状态和两个字段是否实际更新。

### 同步状态

单个接口包含两个独立字段结果，但响应只有一个 `sync_status`。采用最严格结果聚合：

```text
unavailable > rejected > pending > approved
```

只提交一个字段时，状态就是该字段结果；昵称通过但头像拒绝时，昵称仍可更新，整体状态返回 `rejected`，头像保持旧值。没有提交任何资料时不执行同步并返回参数错误。

`pending` 只记录审核流水和内容摘要/短期待处理标记，不公开待审核内容；当前公开昵称和头像保持不变。微信官方同步审核通常是同步的，`pending` 主要用于兼容审核服务返回该状态的情况。

## `/auth/wechat-login` 调整

- 新用户创建时仍可使用首次授权资料，但必须经过现有审核流程。
- 已有账号不再通过 `nickname` 或 `avatar` 请求字段更新资料，包括昵称仍为默认昵称的账号；已有账号资料同步统一调用新接口。
- 保留 openid 账号绑定、登录限流、token 发放和账号禁用检查。
- 对普通已审核账号的保护逻辑不删除，只把资料变更职责移到新接口。

## 头像安全规则

- 只允许 HTTPS 的微信头像来源；不接受客户端任意 URL。
- 下载请求使用有限读取，检查 HTTP 状态、实际图片格式、尺寸上限和空内容。
- 跟随重定向时逐跳校验目标仍是允许的微信头像域名；跨域重定向直接失败。
- 下载内容经现有图片处理函数转码后才进入图片审核和自己的头像存储。
- 审核失败、下载失败、处理失败、存储失败或数据库失败都不覆盖旧头像。
- 公开响应只返回自己的 HTTPS 头像地址或默认头像，不返回微信临时地址。

## 防重、并发与错误

- Redis 一次性 code key 使用 `sha256(code)`，设置短 TTL，并用 `SETNX`/等效原子操作防止并发重复处理。
- 资料同步限流按当前用户 ID 计算，配置一个短时间窗口；限流和防重都不记录原始凭证。
- 用户资料更新使用现有数据库更新方法；在同一用户的并发同步请求中，只有通过防重和审核的请求可以更新。
- openid 不匹配、token 过期、code 重放、审核拒绝、审核不可用、头像下载失败均使用统一响应格式。
- 任何非 `approved` 状态继续通过 `SafePublicNickname` 和 `SafePublicAvatar` 输出默认资料。

## 数据库与清理

不新增表或字段，前提是生产已执行现有 `00013_create_user_moderation.sql`。审核结果继续写入 `user_moderation_events`，不写原始昵称或图片。历史违规清理仍使用：

```text
go run ./cmd/moderation-cleanup -dry-run
go run ./cmd/moderation-cleanup
```

清理命令只处理审核字段和公开资料过滤，不清空整个 Redis，不影响金币、进度或排行榜分数。

## 测试设计

新增服务和 handler 测试覆盖：

1. 新用户首次资料同步通过后保存昵称和头像。
2. 默认昵称账号可以受控同步一次，正常昵称账号审核通过后可以更新。
3. pending、rejected、unavailable 均不覆盖旧资料。
4. openid 与当前 token 用户不一致时拒绝更新。
5. 同一 code 的重复请求和并发请求只处理一次。
6. 同步限流阻止频繁审核请求。
7. 头像只接受合法微信 HTTPS 来源；跨域重定向、空文件、超限、伪造格式和审核失败均保留旧头像。
8. `/auth/wechat-login` 已有账号不能通过昵称或头像参数修改资料。
9. 排行榜、排位、好友房和资料接口继续隐藏非 approved 资料。
10. 新接口和错误响应均符合 `code/message/data` 包装，代码、日志和前端目录不包含 AppSecret。

验证命令：

```text
D:/bin/go.exe test ./...
D:/bin/go.exe vet ./...
D:/bin/go.exe build ./cmd/api
```

## 生产发布

实现完成并通过测试后，生产环境不需要新的数据库迁移，但必须确认：

- `GO_SERVICE_WECHAT_APP_ID` 与正式小游戏一致；
- `GO_SERVICE_WECHAT_APP_SECRET` 已在服务器配置且未写入仓库；
- Redis 可用；
- `00013` 已执行；
- API 和 worker 使用新版本重新部署并重启；
- `/ready` 返回 `code=0` 后再做真机授权测试。

