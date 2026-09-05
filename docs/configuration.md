# Configuration

Configuration precedence is code defaults, role YAML, then `APP_*` environment variables. Each Role loads only its own root configuration and fails startup when a capability used by that Role is missing. API, Job, and Consumer require MySQL; API also requires Redis for local identity sessions and captchas, while Job and Consumer require RocketMQ.

Use `APP_CONFIG_FILE` to select a role configuration file. Common overrides include `APP_SERVER_HTTP_PORT`, `APP_SERVER_MANAGEMENT_PORT`, `APP_LOGGING_LEVEL`, `APP_LOGGING_FORMAT`, `APP_MYSQL_DSN`, `APP_REDIS_ADDRESS`, and `APP_ROCKETMQ_HANDLER_TIMEOUT`. RocketMQ credentials may both be empty for an unauthenticated local broker; otherwise AccessKey and SecretKey must be configured together.

Identity settings are `APP_IDENTITY_ACCESS_TOKEN_TTL`, `APP_IDENTITY_REFRESH_TOKEN_TTL`, and `APP_IDENTITY_CAPTCHA_TTL`. Initial credentials are secret-only settings: `APP_IDENTITY_BOOTSTRAP_USER` and `APP_IDENTITY_BOOTSTRAP_PASSWORD` must be supplied together and create a ROOT account only when no active account exists. `APP_IDENTITY_DEFAULT_PASSWORD` is required before an administrator can create another account. Both password settings must contain at least eight characters and must not be committed.

The API file adapter is selected with `APP_FILE_STORAGE_TYPE`: `local`, `s3`, or `aliyun_oss`. `APP_FILE_STORAGE_MAX_FILE_BYTES` must be positive and no larger than `APP_SERVER_MAX_BODY_BYTES`. Returned content URLs are intentionally public and use opaque generated keys so avatars and notice media can render without adding an authorization header.

Local storage uses `APP_FILE_STORAGE_ROOT` and defaults to `.tmp/uploads`. Production must point it at a persistent writable mount.

S3-compatible storage uses `APP_FILE_STORAGE_S3_ENDPOINT`, `APP_FILE_STORAGE_S3_REGION`, `APP_FILE_STORAGE_S3_BUCKET`, `APP_FILE_STORAGE_S3_ACCESS_KEY`, `APP_FILE_STORAGE_S3_SECRET_KEY`, and `APP_FILE_STORAGE_S3_USE_PATH_STYLE`. The endpoint may be empty for AWS S3. AccessKey and SecretKey may both be empty to use the AWS default credential chain; otherwise both are required. Path-style addressing is useful for MinIO, RustFS, and similar self-hosted implementations.

Aliyun OSS uses `APP_FILE_STORAGE_ALIYUN_OSS_ENDPOINT`, `APP_FILE_STORAGE_ALIYUN_OSS_BUCKET`, `APP_FILE_STORAGE_ALIYUN_OSS_ACCESS_KEY`, and `APP_FILE_STORAGE_ALIYUN_OSS_SECRET_KEY`. All four values are required when `APP_FILE_STORAGE_TYPE=aliyun_oss`.

SSE uses the configured Redis instance for cross-replica Pub/Sub and online-user presence. Reverse proxies must disable response buffering for `/api/v1/sse/connect`, preserve the `Authorization` header, and allow responses longer than the normal request timeout. No SSE payload is written to application logs.

Database-backed system configuration uses a versioned Redis cache. Every configuration mutation advances the cache generation, and a reader may refill only the generation it originally observed. Old generations expire automatically, so cache invalidation does not scan Redis keys and an in-flight stale read cannot repopulate the active generation.

`APP_MYSQL_DSN` is the GORM application DSN. Database schema changes are reviewed and executed manually from the versioned SQL files under `migrations/`; no migration URL or automatic migration runner is part of application configuration.

Migration `20260905000100` changes soft-delete uniqueness to generated active-key columns. Its down migration is valid only while no duplicate deleted business keys have been created. After the new semantics have been used, preserve history and roll forward rather than deleting or renaming records merely to force a rollback.

Production secrets belong in environment or platform secret injection. They must not appear in YAML, logs, health responses, build information, or error messages.

The runtime image contains the repository's non-secret Role YAML files under `/configs`; environment variables override them. `APP_CONFIG_FILE` may point to a mounted alternative, but secrets should still be injected through the deployment platform rather than baked into that file.
