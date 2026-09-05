CREATE TABLE scheduler_job_lock (
    job_name VARCHAR(128) NOT NULL,
    lease_owner VARCHAR(128) NOT NULL,
    lease_until DATETIME(3) NOT NULL,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    PRIMARY KEY (job_name),
    KEY idx_scheduler_job_lock_lease_until (lease_until)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

CREATE TABLE scheduler_job_run (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    run_id CHAR(36) NOT NULL,
    job_name VARCHAR(128) NOT NULL,
    scheduled_at DATETIME(3) NOT NULL,
    started_at DATETIME(3) NOT NULL,
    finished_at DATETIME(3) NULL,
    status VARCHAR(32) NOT NULL,
    instance_id VARCHAR(128) NOT NULL,
    error_summary VARCHAR(1000) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_scheduler_job_run_run_id (run_id),
    KEY idx_scheduler_job_run_job_created (job_name, created_at),
    KEY idx_scheduler_job_run_status_started (status, started_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;
