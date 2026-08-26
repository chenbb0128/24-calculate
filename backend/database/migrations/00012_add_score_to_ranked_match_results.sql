-- +goose Up
ALTER TABLE ranked_match_results
    ADD COLUMN score INT UNSIGNED NOT NULL DEFAULT 0 AFTER mistakes,
    ADD COLUMN verified TINYINT(1) NOT NULL DEFAULT 1 AFTER best_tier;

-- +goose Down
ALTER TABLE ranked_match_results
    DROP COLUMN verified,
    DROP COLUMN score;
