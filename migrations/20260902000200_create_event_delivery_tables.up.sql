CREATE TABLE event_outbox (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    event_id CHAR(36) NOT NULL,
    event_type VARCHAR(128) NOT NULL,
    event_version INT NOT NULL,
    aggregate_id VARCHAR(128) NOT NULL,
    logical_topic VARCHAR(128) NOT NULL,
    payload JSON NOT NULL,
    status VARCHAR(32) NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    next_attempt_at DATETIME(3) NOT NULL,
    lease_owner VARCHAR(128) NOT NULL DEFAULT '',
    lease_until DATETIME(3) NULL,
    last_error VARCHAR(1000) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL,
    sent_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_event_outbox_event_id (event_id),
    KEY idx_event_outbox_delivery (status, next_attempt_at, lease_until)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

CREATE TABLE event_inbox (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    consumer_group VARCHAR(128) NOT NULL,
    event_id CHAR(36) NOT NULL,
    event_type VARCHAR(128) NOT NULL,
    event_version INT NOT NULL,
    status VARCHAR(32) NOT NULL,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_event_inbox_group_event (consumer_group, event_id),
    KEY idx_event_inbox_status_created (status, created_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;
