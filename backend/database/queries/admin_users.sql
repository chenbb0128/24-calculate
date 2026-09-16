-- name: ListAdminUsers :many
SELECT u.id,
       u.username,
       u.nickname,
       u.avatar,
       u.status,
       u.nickname_moderation_status,
       u.avatar_moderation_status,
       u.moderation_updated_at,
       u.created_at,
       u.updated_at,
       CASE
           WHEN EXISTS (SELECT 1 FROM user_identities AS identity_record WHERE identity_record.user_id = u.id AND identity_record.provider = 'wechat') THEN 'wechat'
           WHEN EXISTS (SELECT 1 FROM user_identities AS identity_record WHERE identity_record.user_id = u.id AND identity_record.provider = 'taptap') THEN 'taptap'
           ELSE 'password'
       END AS platform
FROM users AS u
WHERE (CAST(sqlc.arg(query) AS CHAR) = ''
       OR CAST(u.id AS CHAR) LIKE CONCAT('%', CAST(sqlc.arg(query) AS CHAR), '%')
       OR u.username LIKE CONCAT('%', CAST(sqlc.arg(query) AS CHAR), '%')
       OR (CASE WHEN u.nickname_moderation_status = 'approved' THEN u.nickname ELSE '算术玩家' END) LIKE CONCAT('%', CAST(sqlc.arg(query) AS CHAR), '%'))
  AND (CAST(sqlc.arg(status) AS CHAR) = 'all'
       OR (CAST(sqlc.arg(status) AS CHAR) = 'active' AND u.status <> 0)
       OR (CAST(sqlc.arg(status) AS CHAR) = 'disabled' AND u.status = 0))
  AND (CAST(sqlc.arg(platform) AS CHAR) = 'all'
       OR (CAST(sqlc.arg(platform) AS CHAR) = 'wechat' AND EXISTS (SELECT 1 FROM user_identities AS identity_record WHERE identity_record.user_id = u.id AND identity_record.provider = 'wechat'))
       OR (CAST(sqlc.arg(platform) AS CHAR) = 'taptap'
           AND NOT EXISTS (SELECT 1 FROM user_identities AS identity_record WHERE identity_record.user_id = u.id AND identity_record.provider = 'wechat')
           AND EXISTS (SELECT 1 FROM user_identities AS identity_record WHERE identity_record.user_id = u.id AND identity_record.provider = 'taptap'))
       OR (CAST(sqlc.arg(platform) AS CHAR) = 'password'
           AND NOT EXISTS (SELECT 1 FROM user_identities AS identity_record WHERE identity_record.user_id = u.id AND identity_record.provider IN ('wechat', 'taptap'))))
ORDER BY u.created_at DESC, u.id DESC
LIMIT ? OFFSET ?;

-- name: CountAdminUsers :one
SELECT COUNT(*)
FROM users AS u
WHERE (CAST(sqlc.arg(query) AS CHAR) = ''
       OR CAST(u.id AS CHAR) LIKE CONCAT('%', CAST(sqlc.arg(query) AS CHAR), '%')
       OR u.username LIKE CONCAT('%', CAST(sqlc.arg(query) AS CHAR), '%')
       OR (CASE WHEN u.nickname_moderation_status = 'approved' THEN u.nickname ELSE '算术玩家' END) LIKE CONCAT('%', CAST(sqlc.arg(query) AS CHAR), '%'))
  AND (CAST(sqlc.arg(status) AS CHAR) = 'all'
       OR (CAST(sqlc.arg(status) AS CHAR) = 'active' AND u.status <> 0)
       OR (CAST(sqlc.arg(status) AS CHAR) = 'disabled' AND u.status = 0))
  AND (CAST(sqlc.arg(platform) AS CHAR) = 'all'
       OR (CAST(sqlc.arg(platform) AS CHAR) = 'wechat' AND EXISTS (SELECT 1 FROM user_identities AS identity_record WHERE identity_record.user_id = u.id AND identity_record.provider = 'wechat'))
       OR (CAST(sqlc.arg(platform) AS CHAR) = 'taptap'
           AND NOT EXISTS (SELECT 1 FROM user_identities AS identity_record WHERE identity_record.user_id = u.id AND identity_record.provider = 'wechat')
           AND EXISTS (SELECT 1 FROM user_identities AS identity_record WHERE identity_record.user_id = u.id AND identity_record.provider = 'taptap'))
       OR (CAST(sqlc.arg(platform) AS CHAR) = 'password'
           AND NOT EXISTS (SELECT 1 FROM user_identities AS identity_record WHERE identity_record.user_id = u.id AND identity_record.provider IN ('wechat', 'taptap'))));

-- name: GetAdminUser :one
SELECT u.id,
       u.username,
       u.nickname,
       u.avatar,
       u.status,
       u.nickname_moderation_status,
       u.avatar_moderation_status,
       u.moderation_updated_at,
       u.created_at,
       u.updated_at,
       CASE
           WHEN EXISTS (SELECT 1 FROM user_identities AS identity_record WHERE identity_record.user_id = u.id AND identity_record.provider = 'wechat') THEN 'wechat'
           WHEN EXISTS (SELECT 1 FROM user_identities AS identity_record WHERE identity_record.user_id = u.id AND identity_record.provider = 'taptap') THEN 'taptap'
           ELSE 'password'
       END AS platform
FROM users AS u
WHERE u.id = sqlc.arg(id)
LIMIT 1;

-- name: GetAdminUserStats :one
SELECT COUNT(*) AS total,
       COUNT(CASE WHEN DATE(created_at) = UTC_DATE() THEN 1 END) AS new_today,
       COUNT(CASE WHEN status <> 0 THEN 1 END) AS active,
       COUNT(CASE WHEN status = 0 THEN 1 END) AS disabled
FROM users;

-- name: UpdateAdminUserStatus :execrows
UPDATE users
SET status = sqlc.arg(status),
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);
