-- name: GetPlayerProfile :one
SELECT user_id, progress_json, created_at, updated_at
FROM player_profiles
WHERE user_id = ?
LIMIT 1;

-- name: CreatePlayerProfile :exec
INSERT INTO player_profiles (
    user_id,
    progress_json,
    created_at,
    updated_at
) VALUES (?, ?, ?, ?);
