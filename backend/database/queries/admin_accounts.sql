-- name: GetAdminAccountByID :one
SELECT id, username, password_hash, role, status, last_login_at, created_at, updated_at
FROM admin_accounts
WHERE id = ?
LIMIT 1;

-- name: GetAdminAccountByUsername :one
SELECT id, username, password_hash, role, status, last_login_at, created_at, updated_at
FROM admin_accounts
WHERE username = ?
LIMIT 1;

-- name: CreateAdminAccount :execresult
INSERT INTO admin_accounts (
    username,
    password_hash,
    role,
    status,
    created_at,
    updated_at
) VALUES (?, ?, ?, ?, ?, ?);

-- name: TouchAdminLastLogin :exec
UPDATE admin_accounts
SET last_login_at = ?, updated_at = ?
WHERE id = ?;
