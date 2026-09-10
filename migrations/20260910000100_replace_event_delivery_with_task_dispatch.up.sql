CREATE TABLE task_dispatch_outbox (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    dispatch_id CHAR(36) NOT NULL,
    task_type VARCHAR(128) NOT NULL,
    task_id VARCHAR(128) NOT NULL,
    task_version INT NOT NULL,
    stream_name VARCHAR(128) NOT NULL,
    status VARCHAR(32) NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    next_attempt_at DATETIME(3) NOT NULL,
    lease_owner VARCHAR(128) NOT NULL DEFAULT '',
    lease_until DATETIME(3) NULL,
    last_error VARCHAR(1000) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL,
    published_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_task_dispatch_outbox_dispatch_id (dispatch_id),
    KEY idx_task_dispatch_outbox_delivery (status, next_attempt_at, lease_until),
    KEY idx_task_dispatch_outbox_task (task_type, task_id, created_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

DROP TABLE IF EXISTS event_inbox;
DROP TABLE IF EXISTS event_outbox;
