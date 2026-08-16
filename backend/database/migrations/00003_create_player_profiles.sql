-- +goose Up

CREATE TABLE player_profiles (
    user_id BIGINT UNSIGNED NOT NULL,
    progress_json LONGTEXT NOT NULL,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    PRIMARY KEY (user_id),
    CONSTRAINT fk_player_profiles_user
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down

DROP TABLE player_profiles;
