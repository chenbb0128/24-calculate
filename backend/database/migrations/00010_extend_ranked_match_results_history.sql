-- +goose Up
ALTER TABLE ranked_match_results
    ADD COLUMN opponent_user_id BIGINT UNSIGNED NULL AFTER user_id,
    ADD COLUMN solved INT UNSIGNED NOT NULL DEFAULT 0 AFTER opponent_user_id,
    ADD COLUMN question_count INT UNSIGNED NOT NULL DEFAULT 0 AFTER solved,
    ADD COLUMN elapsed_ms INT UNSIGNED NOT NULL DEFAULT 0 AFTER question_count,
    ADD COLUMN mistakes INT UNSIGNED NOT NULL DEFAULT 0 AFTER elapsed_ms,
    ADD KEY idx_ranked_results_user_season_created_id (user_id, season_id, created_at, id),
    ADD CONSTRAINT fk_ranked_results_opponent_user
        FOREIGN KEY (opponent_user_id) REFERENCES users (id)
        ON DELETE SET NULL;

-- +goose Down
ALTER TABLE ranked_match_results
    DROP FOREIGN KEY fk_ranked_results_opponent_user,
    DROP INDEX idx_ranked_results_user_season_created_id,
    DROP COLUMN mistakes,
    DROP COLUMN elapsed_ms,
    DROP COLUMN question_count,
    DROP COLUMN solved,
    DROP COLUMN opponent_user_id;
