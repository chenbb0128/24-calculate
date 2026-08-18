# 排位战绩、头像、Run 恢复与登出

本分支新增后端能力，正式环境 API 前缀为 `https://calc-api.pdurl.cn`。

## 排位战绩

所有请求都需要 `Authorization: Bearer <access_token>`。

```http
GET /api/v1/player/ranked/summary?season_id=2026-S3
```

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "season_id": "2026-S3",
    "label": "青铜 III",
    "rating": 1000,
    "stars_label": "0 星",
    "ranked_matches": 0,
    "wins": 0,
    "losses": 0,
    "draws": 0,
    "win_rate": 0,
    "current_streak": 0,
    "best_streak": 0
  }
}
```

```http
GET /api/v1/player/ranked/matches?season_id=2026-S3&limit=20&cursor=<next_cursor>
GET /api/v1/player/ranked/matches/<match_id>
```

列表响应的 `data.matches` 只包含当前用户的记录，按服务端创建时间倒序；`next_cursor` 是不透明游标。单场查询通过 `user_id + match_id` 限制归属，不能读取其他用户的记录。

## 头像上传

```http
POST /api/v1/users/me/avatar
Authorization: Bearer <access_token>
Content-Type: multipart/form-data
```

表单文件字段必须是 `file`。服务端检查文件魔数和可解码内容，仅接受 JPG、PNG、WEBP，原文件不超过 2MB，输出为 256x256 WEBP。响应的 `data` 包含 `avatar_url`、`profile`；数据库更新失败会清理新文件并保留旧头像。

## Run 恢复

```http
GET /api/v1/player/campaign/runs/<run_id>
GET /api/v1/player/daily/runs/<run_id>
GET /api/v1/player/endless/runs/<run_id>
```

服务端按 Run 所属用户校验，返回服务端保存的 `questions`、`puzzles`、`attempts` 和进度。不存在返回 404，未结束且已过期返回 410，已结束的 Run 返回终态。Run 提交使用服务端生成的题目和幂等键，重复提交不会重复发放奖励。

## 登出

```http
POST /api/v1/auth/logout
Authorization: Bearer <access_token>
Content-Type: application/json

{"refresh_token":"<refresh_token>"}
```

成功响应为：

```json
{"code":0,"message":"success","data":{"revoked":true}}
```

服务端校验两个 token 的用户一致，删除 refresh token，并在 Redis 中按 access token 的剩余有效期写入撤销标记。重复登出安全幂等；后续受保护接口会拒绝旧 access token。

## 数据库和生产配置

新增迁移：`database/migrations/00010_extend_ranked_match_results_history.sql`，需要在生产 MySQL 执行 goose up。迁移为排位记录补充对手用户、解题数、题目数、耗时和错误数，并新增分页索引。

认证撤销依赖现有生产 Redis。头像继续使用现有配置：`GO_SERVICE_AVATAR_STORAGE_DIR`、`GO_SERVICE_AVATAR_PUBLIC_BASE_URL`、`GO_SERVICE_AVATAR_MAX_BYTES`、`GO_SERVICE_AVATAR_MAX_DIMENSION` 和 `GO_SERVICE_AVATAR_UPLOAD_COOLDOWN_SECONDS`。AppSecret 只配置在服务端微信配置中，不能写入前端、日志或提交内容。
