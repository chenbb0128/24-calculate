-- name: ListUsersForModeration :many
SELECT id, username, password_hash, nickname, avatar, status,
       nickname_moderation_status, avatar_moderation_status,
       moderation_updated_at, created_at, updated_at
FROM users
WHERE id > ?
ORDER BY id ASC
LIMIT ?;

-- name: UpdateUserModeration :exec
UPDATE users
SET nickname = ?,
    avatar = ?,
    nickname_moderation_status = ?,
    avatar_moderation_status = ?,
    moderation_updated_at = ?,
    updated_at = ?
WHERE id = ?;

-- name: RecordModerationEvent :exec
INSERT INTO user_moderation_events (
    user_id,
    resource_type,
    resource_id,
    source,
    moderation_status,
    reason_code,
    provider_request_id,
    created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?);
