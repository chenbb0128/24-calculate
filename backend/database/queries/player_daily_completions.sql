-- name: GetPlayerDailyCompletionByDate :one
SELECT id, user_id, date_key, idempotency_key, score, best_score, streak, reward_coins, created_at
FROM player_daily_completions
WHERE user_id = ? AND date_key = ?
LIMIT 1;

-- name: CreatePlayerDailyCompletion :exec
INSERT INTO player_daily_completions (
    user_id,
    date_key,
    idempotency_key,
    score,
    best_score,
    streak,
    reward_coins,
    created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?);
