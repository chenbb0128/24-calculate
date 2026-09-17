# 后端完善报告

更新时间：2026-09-18

## 已完成

1. 闯关、每日挑战和无尽模式统一使用服务端 Run 合同，保存题目、状态、题目索引、分数、错误数、连击、耗时和过期时间。
2. 增加 Run 恢复接口：
   - `GET /api/v1/player/campaign/runs/{run_id}`
   - `GET /api/v1/player/daily/runs/{run_id}`
   - `GET /api/v1/player/endless/runs/{run_id}`
3. 题目保存 SHA-256 `question_hash`、`solution_count`、`shortest_steps` 和 `source_seed`。答案步骤只保存在服务端 Run 中，开始接口不会返回。
4. 服务端重放四则运算并验证每个数字只使用一次、除数不为零、中间结果规则和最终结果 24，同时兼容前端的 `×`、`÷` 以及历史 `*`、`/`。
5. 每日题目由上海时区日期和 `GO_SERVICE_GAME_DAILY_SEED_SECRET` 生成，同日稳定、跨日变化；同一用户当天重复进入会返回 `completed: true`。
6. Run、好友结算和人机匹配结算使用 Redis 分布式锁；MySQL 进度事务和唯一幂等键共同防止重复奖励、重复扣款和重复提交。
7. 商城购买验证服务端商品价格、解锁条件、金币余额和物品归属，旧前端发送 `{}` 仍兼容；购买记录写入玩家进度并支持幂等重放。
8. 好友房间支持等待、准备、倒计时、运行、结束、过期和取消；房间题目合同、题目顺序、进度单调性、事件幂等和对局结果由服务端验证。
9. 快速匹配支持真实玩家配对、超时匿名对手、取消、归属校验和 Redis 队列；人机进度按服务端时间逐题写入 Redis，不返回 `is_bot`。
10. 排行榜支持 campaign、daily、endless、friend、overall，支持 global/friends、all/weekly/monthly、分页和 `anomaly` 标记；空榜返回正常成功响应。
11. JWT access/refresh、refresh 一次性消费、微信/TapTap 登录限流、请求 ID、统一错误响应、`/health` 和 `/ready` 已保留并完善。TapTap 使用 `/api/v1/auth/taptap-login` 和独立的 `taptap` provider 身份，不与微信 openid 混用。
12. OpenAPI 文档已经覆盖实际路由，并通过自动 YAML 解析测试。
13. 排位系统使用 `player_rank_profiles` 按 `user_id + season_id` 保存服务器段位；排位结算使用 `ranked_match_results` 审计表和数据库事务，重复请求返回第一次结算结果。
14. 排位快速匹配按模式、规则版本、地区、赛季、大段位和小分区隔离；公开好友房和 AI 房强制休闲，只有服务端创建的快速匹配真人房可进入排位结算。
15. `GET /api/v1/player/rank` 和 `leaderboards/ranked` 已接入；排位排行榜始终使用服务端当前赛季，`period=season` 可用。

## 数据与兼容性

- 没有删除已有表、用户或进度数据。
- 现有玩家进度仍保存在 `player_profiles.progress_json`，金币、奖励、商城购买和每日/好友奖励通过 MySQL 行锁事务更新。
- 排行榜幂等记录使用 `player_leaderboard_submissions`，唯一键为 `user_id + mode + idempotency_key`。
- 前端已有的数字错误码、旧完成接口、旧商城 `{}` 请求和历史运算符仍保持兼容。

## 测试结果

以下命令已通过：

```powershell
cd D:\微信小游戏\backend
D:\bin\go.exe test ./...
D:\bin\go.exe vet ./...
D:\bin\go.exe build ./...
```

测试覆盖题目生成和验证、每日 seed、Run 计分和幂等、好友房间生命周期、好友进度、匹配、匿名对手、商城进度和排行榜查询。Redis 集成测试需要本机 Memurai/Redis 运行后再执行。

## 尚未接入的外部能力

- Run、房间和实时匹配状态目前主要存放在带 TTL 的 Redis 中，尚未建立独立的 `game_runs`、`game_run_questions`、`friend_rooms`、`match_results` 历史表。
- 广告奖励尚未接入第三方广告服务端回调；后端不会凭客户端字段发放广告奖励。
- 排行榜尚未接入跨实例 Redis 快照缓存，集群部署前需要补充。
- 人机推进当前由匹配状态读取触发；需要无轮询后台推进时，可将其迁移为 Asynq 定时任务。

## TapTap 发布前配置

- 在 TapTap 开发者中心创建小游戏并取得 MiniApp ID 和服务端密钥；只在后端配置 `GO_SERVICE_TAPTAP_APP_ID`、`GO_SERVICE_TAPTAP_APP_SECRET`，不要放入 `frontend/taptap_game` 包体。
- 生产环境使用默认的 `https://cloud-miniapp.tapapis.cn`，并确认服务器可访问该地址；TapTap 后台将 `https://calc-api.pdurl.cn` 加入请求域名白名单。
- 完成 TapTap 真机登录、bootstrap、闯关、每日、无尽、好友房和排行榜验收后，再用 TapTap 打包工具生成 ZIP 上传。

## 排位系统上线前迁移

新增 Goose 迁移：

- `00007_create_player_rank_profiles.sql`
- `00008_create_ranked_match_results.sql`
- `00009_extend_ranked_match_results.sql`

执行全部未执行迁移后，服务端会按当前上海时区季度创建赛季记录。客户端提交的 rating、tier、division、stars、winner 和 delta 不参与真实结算。

## 内容审核与用户资料安全

已增加 `00013_create_user_moderation.sql`，为用户资料保存昵称/头像审核状态，并以 `user_moderation_events` 保存不含原始文本或图片的审核审计。微信登录使用服务端 AppID/AppSecret 调用 `jscode2session` 和内容安全接口；昵称审核通过后才保存为公开昵称，拒绝、待审核、未审核或上游不可用时使用 `算术玩家`。普通昵称 PATCH 返回 `NICKNAME_EDIT_DISABLED`，避免绕过审核。

`POST /api/v1/users/me/avatar` 要求 Bearer token 和 multipart 字段 `file`，通过图片解析、256×256 WEBP 转码和内容审核后才写入存储；失败不会覆盖旧头像。排行榜、排位/好友历史、好友房和匹配响应均按审核状态二次过滤，不公开审核状态、原因码、provider request ID 或原始资料。

历史整改命令：

```bash
./moderation-cleanup --dry-run
./moderation-cleanup
```

必须先 dry-run 检查统计，再 apply；命令不发奖励、不修改进度、不删除历史、不清空 Redis。
命令输出包含 `scanned`、`updated`、`approved`、`rejected`、`unreviewed`、`failed`、`avatars_hidden` 和 `cache_changes`，便于确认审核上游故障与历史头像撤下数量。

微信隐私授权后的资料同步使用：

`POST /api/v1/users/me/wechat-profile/sync`

该接口要求当前用户的 Bearer access token 和一次新的 `wx.login` code。服务端再次用 AppSecret 换取 openid，并确认 openid 绑定当前账号；code 使用过、账号不匹配或超过限流均会被拒绝。昵称和头像分别经过现有文本/图片审核服务，只有 approved 才会更新公开资料；pending、rejected、unavailable 会保留旧资料或安全默认值。微信头像会先从 allowlist 内的 HTTPS `qlogo.cn` 地址下载、解析、裁剪为 256×256 WEBP，再保存到自己的头像目录，公开接口不会返回微信临时地址。

`/api/v1/auth/wechat-login` 仍然只负责登录。已有账号提交的 `nickname`、`avatar` 参数会被忽略；新账号先以安全默认资料创建，再通过同一套受控同步服务处理首次授权资料。AppSecret 只从服务端配置读取。
