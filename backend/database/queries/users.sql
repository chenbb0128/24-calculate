-- name: GetUserByID :one
SELECT id, username, password_hash, nickname, avatar, status,
       nickname_moderation_status, avatar_moderation_status,
       moderation_updated_at, created_at, updated_at
FROM users
WHERE id = ?
LIMIT 1;

-- name: GetUserByUsername :one
SELECT id, username, password_hash, nickname, avatar, status,
       nickname_moderation_status, avatar_moderation_status,
       moderation_updated_at, created_at, updated_at
FROM users
WHERE username = ?
LIMIT 1;

-- name: GetUserByProviderSubject :one
SELECT u.id, u.username, u.password_hash, u.nickname, u.avatar, u.status,
       u.nickname_moderation_status, u.avatar_moderation_status,
       u.moderation_updated_at, u.created_at, u.updated_at
FROM users AS u
INNER JOIN user_identities AS identity_record ON identity_record.user_id = u.id
WHERE identity_record.provider = ? AND identity_record.provider_subject = ?
LIMIT 1;

-- name: CreateUser :execresult
INSERT INTO users (
    username,
    password_hash,
    nickname,
    avatar,
    status,
    nickname_moderation_status,
    avatar_moderation_status,
    moderation_updated_at,
    created_at,
    updated_at
) VALUES (?, ?, ?, ?, ?, COALESCE(NULLIF(?, ''), 'approved'), COALESCE(NULLIF(?, ''), 'approved'), ?, ?, ?);

-- name: CreateUserIdentity :exec
INSERT INTO user_identities (
    user_id,
    provider,
    provider_subject,
    created_at,
    updated_at
) VALUES (?, ?, ?, ?, ?);

-- name: UpdateUserProfile :exec
UPDATE users
SET nickname = ?,
    avatar = ?,
    nickname_moderation_status = COALESCE(NULLIF(?, ''), nickname_moderation_status),
    avatar_moderation_status = COALESCE(NULLIF(?, ''), avatar_moderation_status),
    moderation_updated_at = COALESCE(?, moderation_updated_at),
    updated_at = ?
WHERE id = ?;

-- name: DisableUser :exec
UPDATE users
SET status = 0, updated_at = ?
WHERE id = ?;
