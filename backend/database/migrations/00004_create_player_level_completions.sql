-- +goose Up

CREATE TABLE player_level_completions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    level_id INT UNSIGNED NOT NULL,
    idempotency_key VARCHAR(128) NOT NULL,
    score SMALLINT UNSIGNED NOT NULL,
    stars TINYINT UNSIGNED NOT NULL,
    reward_coins INT UNSIGNED NOT NULL DEFAULT 0,
    best_score SMALLINT UNSIGNED NOT NULL,
    unlocked_level INT UNSIGNED NOT NULL,
    created_at DATETIME(3) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_level_completions_user_idempotency (user_id, idempotency_key),
    CONSTRAINT fk_level_completions_user
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down

DROP TABLE player_level_completions;
