-- +goose Up
ALTER TABLE ranked_match_results
    ADD COLUMN tier_before VARCHAR(16) NOT NULL DEFAULT 'bronze' AFTER rating_after,
    ADD COLUMN tier_after VARCHAR(16) NOT NULL DEFAULT 'bronze' AFTER tier_before,
    ADD COLUMN division_before TINYINT UNSIGNED NOT NULL DEFAULT 3 AFTER tier_after,
    ADD COLUMN division_after TINYINT UNSIGNED NOT NULL DEFAULT 3 AFTER division_before,
    ADD COLUMN stars_before TINYINT UNSIGNED NOT NULL DEFAULT 0 AFTER division_after,
    ADD COLUMN stars_after TINYINT UNSIGNED NOT NULL DEFAULT 0 AFTER stars_before,
    ADD COLUMN placement_matches INT UNSIGNED NOT NULL DEFAULT 0 AFTER stars_after,
    ADD COLUMN ranked_matches INT UNSIGNED NOT NULL DEFAULT 0 AFTER placement_matches,
    ADD COLUMN wins INT UNSIGNED NOT NULL DEFAULT 0 AFTER ranked_matches,
    ADD COLUMN losses INT UNSIGNED NOT NULL DEFAULT 0 AFTER wins,
    ADD COLUMN draws INT UNSIGNED NOT NULL DEFAULT 0 AFTER losses,
    ADD COLUMN best_tier VARCHAR(16) NOT NULL DEFAULT 'bronze' AFTER draws,
    ADD CONSTRAINT chk_ranked_results_outcome CHECK (outcome IN ('win', 'lose', 'draw')),
    ADD CONSTRAINT chk_ranked_results_rating_before CHECK (rating_before BETWEEN 0 AND 9999),
    ADD CONSTRAINT chk_ranked_results_rating_after CHECK (rating_after BETWEEN 0 AND 9999);

ALTER TABLE player_rank_profiles
    ADD CONSTRAINT chk_rank_profiles_rating CHECK (rating BETWEEN 0 AND 9999),
    ADD CONSTRAINT chk_rank_profiles_division CHECK (division BETWEEN 1 AND 3),
    ADD CONSTRAINT chk_rank_profiles_stars CHECK (stars BETWEEN 0 AND 4);

-- +goose Down
ALTER TABLE ranked_match_results
    DROP COLUMN best_tier,
    DROP COLUMN draws,
    DROP COLUMN losses,
    DROP COLUMN wins,
    DROP COLUMN ranked_matches,
    DROP COLUMN placement_matches,
    DROP COLUMN stars_after,
    DROP COLUMN stars_before,
    DROP COLUMN division_after,
    DROP COLUMN division_before,
    DROP COLUMN tier_after,
    DROP COLUMN tier_before,
    DROP CONSTRAINT chk_ranked_results_outcome,
    DROP CONSTRAINT chk_ranked_results_rating_before,
    DROP CONSTRAINT chk_ranked_results_rating_after;

ALTER TABLE player_rank_profiles
    DROP CONSTRAINT chk_rank_profiles_rating,
    DROP CONSTRAINT chk_rank_profiles_division,
    DROP CONSTRAINT chk_rank_profiles_stars;
