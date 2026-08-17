-- +goose Up
CREATE TABLE ranked_match_results (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    match_id VARCHAR(128) NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    season_id VARCHAR(32) NOT NULL,
    outcome VARCHAR(8) NOT NULL,
    rating_before INT NOT NULL,
    rating_delta INT NOT NULL,
    rating_after INT NOT NULL,
    idempotency_key VARCHAR(128) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_ranked_results_match_user (match_id, user_id),
    UNIQUE KEY uk_ranked_results_user_idempotency (user_id, idempotency_key),
    KEY idx_ranked_results_user_season_created (user_id, season_id, created_at),
    CONSTRAINT fk_ranked_results_user
        FOREIGN KEY (user_id) REFERENCES users (id)
        ON DELETE CASCADE
);

-- +goose Down
DROP TABLE ranked_match_results;
