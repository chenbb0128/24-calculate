-- +goose Up

CREATE TABLE endless_runs (
    run_id VARCHAR(64) NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    run_seed BIGINT NOT NULL,
    status VARCHAR(16) NOT NULL,
    current_question_index INT UNSIGNED NOT NULL DEFAULT 0,
    score INT UNSIGNED NOT NULL DEFAULT 0,
    mistakes INT UNSIGNED NOT NULL DEFAULT 0,
    best_combo INT UNSIGNED NOT NULL DEFAULT 0,
    time_limit_ms INT UNSIGNED NOT NULL DEFAULT 60000,
    started_at DATETIME(6) NOT NULL,
    last_activity_at DATETIME(6) NOT NULL,
    deadline_at DATETIME(6) NOT NULL,
    expires_at DATETIME(6) NOT NULL,
    submitted_at DATETIME(6) NULL,
    reward_claimed TINYINT(1) NOT NULL DEFAULT 0,
    version INT UNSIGNED NOT NULL DEFAULT 1,
    state_version INT UNSIGNED NOT NULL DEFAULT 2,
    start_idempotency_key VARCHAR(128) NULL,
    state_json LONGTEXT NOT NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (run_id),
    UNIQUE KEY uk_endless_runs_user_start_key (user_id, start_idempotency_key),
    KEY idx_endless_runs_user_status (user_id, status, created_at),
    KEY idx_endless_runs_expires (expires_at),
    CONSTRAINT fk_endless_runs_user
        FOREIGN KEY (user_id) REFERENCES users (id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE endless_run_questions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    run_id VARCHAR(64) NOT NULL,
    question_index INT UNSIGNED NOT NULL,
    puzzle_id VARCHAR(64) NOT NULL,
    question_hash CHAR(64) NOT NULL,
    numbers_json VARCHAR(128) NOT NULL,
    target TINYINT UNSIGNED NOT NULL DEFAULT 24,
    rules_json TEXT NOT NULL,
    solution_hash CHAR(64) NOT NULL,
    difficulty VARCHAR(16) NOT NULL,
    served_at DATETIME(6) NOT NULL,
    answered_at DATETIME(6) NULL,
    solution_steps_json TEXT NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_endless_run_question (run_id, question_index),
    KEY idx_endless_question_puzzle (puzzle_id),
    CONSTRAINT fk_endless_questions_run
        FOREIGN KEY (run_id) REFERENCES endless_runs (run_id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE endless_run_attempts (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    run_id VARCHAR(64) NOT NULL,
    question_index INT UNSIGNED NOT NULL,
    idempotency_key VARCHAR(128) NOT NULL,
    solved TINYINT(1) NOT NULL DEFAULT 0,
    validated TINYINT(1) NOT NULL DEFAULT 1,
    elapsed_ms INT UNSIGNED NOT NULL DEFAULT 0,
    mistakes INT UNSIGNED NOT NULL DEFAULT 0,
    score INT UNSIGNED NOT NULL DEFAULT 0,
    score_delta INT UNSIGNED NOT NULL DEFAULT 0,
    combo INT UNSIGNED NOT NULL DEFAULT 0,
    solution_steps_json TEXT NOT NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_endless_attempt_idempotency (run_id, idempotency_key),
    KEY idx_endless_attempt_question (run_id, question_index, created_at),
    CONSTRAINT fk_endless_attempts_run
        FOREIGN KEY (run_id) REFERENCES endless_runs (run_id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down

DROP TABLE endless_run_attempts;
DROP TABLE endless_run_questions;
DROP TABLE endless_runs;
