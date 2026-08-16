-- name: GetPlayerProfileForUpdate :one
SELECT user_id, progress_json, created_at, updated_at
FROM player_profiles
WHERE user_id = ?
LIMIT 1
FOR UPDATE;

-- name: EnsurePlayerProfile :exec
INSERT INTO player_profiles (
    user_id,
    progress_json,
    created_at,
    updated_at
) VALUES (?, ?, ?, ?)
ON DUPLICATE KEY UPDATE user_id = VALUES(user_id);

-- name: UpdatePlayerProfileProgress :exec
UPDATE player_profiles
SET progress_json = ?, updated_at = ?
WHERE user_id = ?;

-- name: GetPlayerLevelCompletionByKey :one
SELECT id, user_id, level_id, idempotency_key, score, stars, reward_coins, best_score, unlocked_level, created_at
FROM player_level_completions
WHERE user_id = ? AND idempotency_key = ?
LIMIT 1;

-- name: CreatePlayerLevelCompletion :exec
INSERT INTO player_level_completions (
    user_id,
    level_id,
    idempotency_key,
    score,
    stars,
    reward_coins,
    best_score,
    unlocked_level,
    created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);
