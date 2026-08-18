# 无尽模式服务端权威流程

无尽模式只在服务端保存当前题目、已服务题目、答题记录和结算结果。客户端提交的 `elapsed_ms`、`score_delta`、`combo`、`mistakes` 和总分只用于协议兼容或诊断，不参与结算。

## 创建对局

```http
POST /api/v1/player/endless/runs
Authorization: Bearer <access_token>
Idempotency-Key: endless-start-20260818
Content-Type: application/json

{"protocol_version":1,"idempotency_key":"endless-start-20260818"}
```

返回的 `puzzle` 是第一道服务端题目；`solution_steps` 只保存在服务端，不会出现在响应中。`expires_at` 是恢复数据的保留期限，`deadline_at` 是本局 60 秒的服务端截止时间。

## 逐题提交

```http
POST /api/v1/player/endless/runs/er_xxx/next-question
Authorization: Bearer <access_token>
Content-Type: application/json

{
  "protocol_version": 1,
  "idempotency_key": "er_xxx-q-0001",
  "question_index": 0,
  "puzzle_id": "ENDLESS-Q0001",
  "elapsed_ms": 12500,
  "solved": true,
  "mistakes": 0,
  "score_delta": 80,
  "combo": 1,
  "solution_steps": [
    {"first_indices":[0],"second_indices":[1],"first":4,"second":5,"operator":"×"}
  ]
}
```

`solved=true` 时服务端会检查四个数字是否各用一次、操作符、操作顺序、中间结果、除零和最终 24。验证成功后服务端递增题号并生成下一道题。`solved=false` 只记录错误并返回同一道当前题，不会自动跳题。

同一个 `idempotency_key` 重试会返回完全相同的结果；相同幂等键搭配不同请求内容会返回 409。

## 恢复和提交

```http
GET /api/v1/player/endless/runs/er_xxx
Authorization: Bearer <access_token>
```

```http
POST /api/v1/player/endless/runs/er_xxx/submit
Authorization: Bearer <access_token>
Content-Type: application/json

{"protocol_version":1,"idempotency_key":"er_xxx-submit-1","run_id":"er_xxx"}
```

提交接口只使用服务端已经保存的 attempts、score、mistakes 和 combo。结算结果写入 `player_leaderboard_submissions` 并使用玩家进度中的服务端事件标记防止重复奖励。

## 生产部署

先执行 `database/migrations/00011_create_endless_runs.sql`，再发布 API。生产应用会使用 MySQL 保存 Run、题目和 attempts，Redis 仅用于分布式锁及其他已有实时状态；因此 API 进程重启不会丢失无尽对局。
