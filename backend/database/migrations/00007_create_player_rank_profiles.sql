-- +goose Up
CREATE TABLE player_rank_profiles (
    user_id BIGINT UNSIGNED NOT NULL,
    season_id VARCHAR(32) NOT NULL,
    rating INT NOT NULL DEFAULT 1000,
    tier VARCHAR(16) NOT NULL DEFAULT 'bronze',
    division TINYINT UNSIGNED NOT NULL DEFAULT 3,
    stars TINYINT UNSIGNED NOT NULL DEFAULT 0,
    placement_matches INT UNSIGNED NOT NULL DEFAULT 0,
    ranked_matches INT UNSIGNED NOT NULL DEFAULT 0,
    wins INT UNSIGNED NOT NULL DEFAULT 0,
    losses INT UNSIGNED NOT NULL DEFAULT 0,
    draws INT UNSIGNED NOT NULL DEFAULT 0,
    best_tier VARCHAR(16) NOT NULL DEFAULT 'bronze',
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (user_id, season_id),
    KEY idx_rank_profiles_season_rating (season_id, rating, user_id),
    CONSTRAINT fk_rank_profiles_user
        FOREIGN KEY (user_id) REFERENCES users (id)
        ON DELETE CASCADE
);

-- +goose Down
DROP TABLE player_rank_profiles;
