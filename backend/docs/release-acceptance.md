# 三火算术练习｜正式环境验收记录

## 当前正式配置目标

- 微信小游戏 AppID：`wx1e7ac815548c561c`
- 正式 API：`https://calc-api.pdurl.cn`
- 正式环境不注册 `/api/v1/auth/dev-login`
- 正式环境不注册客户端可控的旧结算接口：
  - `POST /api/v1/player/levels/:level_id/complete`
  - `POST /api/v1/player/daily/complete`
- 正式环境不注册客户端直接提交排行榜分数的接口：
  - `POST /api/v1/player/leaderboards/:mode/submit`
- 无尽、闯关、每日和好友成绩只能通过对应的服务端校验 run/对战接口写入。
- 微信开发者工具项目已开启合法域名校验和代码压缩。

## 线上只读验收记录（部分通过）

最近验收日期：2026-08-18

| 检查项 | 结果 |
| --- | --- |
| `https://calc-api.pdurl.cn/health` | HTTP 200，返回 `status=ok` |
| `https://calc-api.pdurl.cn/ready` | HTTP 200，返回 `status=ready` |
| `http://calc-api.pdurl.cn/health` | 301 跳转到 HTTPS |
| `POST /api/v1/auth/dev-login` | HTTP 404，开发登录未暴露 |
| `POST /api/v1/player/levels/1/complete` | 当前仍返回 HTTP 401，线上疑似仍为旧版二进制，待部署最新版本 |
| `POST /api/v1/player/daily/complete` | 当前仍返回 HTTP 401，线上疑似仍为旧版二进制，待部署最新版本 |
| `POST /api/v1/player/leaderboards/overall/submit` | 当前仍返回 HTTP 401，线上疑似仍为旧版二进制，待部署最新版本 |
| `/swagger/index.html` | HTTP 404，生产未暴露 Swagger |
| 前端正式冷启动登录审计 | 本地通过 `AUTH_SESSION_OK`，每次冷启动重新绑定 `wx.login`，同一会话不重复登录 |
| DNS A 记录 | `116.62.159.237` |
| 443 端口 | 可连接 |

当前结论：健康检查、HTTPS 跳转、开发登录和 Swagger 检查通过；旧客户端可控结算接口检查未通过，不能将线上环境标记为发布完成。部署最新 `api` 二进制后，必须在 `frontend/wechat_game` 目录重新执行 `node tools/production_probe.js https://calc-api.pdurl.cn`，确认三条旧接口全部返回 HTTP 404。

## 排位系统上线前检查

1. 在生产 MySQL 执行 `00007`、`00008`、`00009` 三个未执行迁移，并确认 `player_rank_profiles`、`ranked_match_results` 存在唯一键和检查约束。
2. 用真实账号调用 `GET /api/v1/player/rank`，确认返回当前上海季度、rating、tier、division、stars 和统计字段。
3. 用快速匹配创建两名真人排位对局，确认房间 `ranked=true`、`match_source=matchmaking`、两名玩家同赛季；分别提交后只产生一次段位变化。
4. 重复提交同一个 `idempotency_key`，确认返回第一次 `rank_result`，不重复增减 rating、金币或排行榜记录。
5. 手动好友房、邀请码房和 AI 房即使请求体携带 `ranked=true`，也必须返回 `ranked=false` 且不改变段位。
6. 请求 `GET /api/v1/player/leaderboards/ranked?scope=global&period=season`，确认只读取服务器当前赛季并按 rating、wins、ranked_matches、user_id 排序。

## 上线前仍需人工完成

1. 在微信公众平台把 `calc-api.pdurl.cn` 配置为小游戏 request 合法域名，确认没有配置 `http://`、端口或路径。
2. 使用真实微信账号在真机完成一次登录、bootstrap、闯关、每日挑战、无尽和好友房验收。
3. 用两个真实微信账号或两台设备验证好友房和排行榜；不要在生产包使用 `dev-login`。
4. 确认生产 MySQL 已完成迁移，并验证一次可恢复的定期备份。
5. 把前端广告占位 ID 替换为公众平台真实广告位 ID，并验证广告失败时不会发放奖励。
6. 补齐小游戏隐私协议、用户协议、备案/类目材料和分享图片后，再上传体验版审核。

验收过程中不得把 AppSecret、JWT 密钥、数据库密码或 Redis 密码写入仓库、前端代码、截图或日志。
