-- +goose Up

CREATE TABLE player_leaderboard_submissions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    mode VARCHAR(16) NOT NULL,
    idempotency_key VARCHAR(128) NOT NULL,
    score INT UNSIGNED NOT NULL,
    questions INT UNSIGNED NOT NULL DEFAULT 0,
    elapsed_ms INT UNSIGNED NOT NULL DEFAULT 0,
    room_id VARCHAR(128) NOT NULL DEFAULT '',
    outcome VARCHAR(16) NOT NULL DEFAULT '',
    metadata_json LONGTEXT NOT NULL,
    created_at DATETIME(3) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_leaderboard_submissions_user_mode_idempotency (user_id, mode, idempotency_key),
    KEY idx_leaderboard_submissions_mode_score (mode, score),
    KEY idx_leaderboard_submissions_mode_room (mode, room_id),
    CONSTRAINT fk_leaderboard_submissions_user
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down

DROP TABLE player_leaderboard_submissions;
