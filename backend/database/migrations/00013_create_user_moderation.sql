-- +goose Up

ALTER TABLE users
    ADD COLUMN nickname_moderation_status VARCHAR(16) NOT NULL DEFAULT 'unreviewed',
    ADD COLUMN avatar_moderation_status VARCHAR(16) NOT NULL DEFAULT 'unreviewed',
    ADD COLUMN moderation_updated_at DATETIME(3) NULL;

CREATE TABLE user_moderation_events (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    resource_type VARCHAR(16) NOT NULL,
    resource_id VARCHAR(128) NOT NULL DEFAULT '',
    source VARCHAR(32) NOT NULL,
    moderation_status VARCHAR(16) NOT NULL,
    reason_code VARCHAR(64) NOT NULL DEFAULT '',
    provider_request_id VARCHAR(128) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL,
    PRIMARY KEY (id),
    KEY idx_moderation_events_user_created (user_id, created_at),
    KEY idx_moderation_events_status_created (moderation_status, created_at),
    CONSTRAINT fk_moderation_events_user
        FOREIGN KEY (user_id) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down

DROP TABLE user_moderation_events;

ALTER TABLE users
    DROP COLUMN nickname_moderation_status,
    DROP COLUMN avatar_moderation_status,
    DROP COLUMN moderation_updated_at;
