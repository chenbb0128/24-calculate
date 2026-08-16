-- +goose Up

CREATE TABLE player_daily_completions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    date_key CHAR(10) NOT NULL,
    idempotency_key VARCHAR(128) NOT NULL,
    score INT UNSIGNED NOT NULL,
    best_score INT UNSIGNED NOT NULL,
    streak INT UNSIGNED NOT NULL,
    reward_coins INT UNSIGNED NOT NULL DEFAULT 0,
    created_at DATETIME(3) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_daily_completions_user_date (user_id, date_key),
    UNIQUE KEY uk_daily_completions_user_idempotency (user_id, idempotency_key),
    CONSTRAINT fk_daily_completions_user
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down

DROP TABLE player_daily_completions;
