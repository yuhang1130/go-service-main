# Configuration

Configuration precedence is code defaults, role YAML, then `APP_*` environment variables. Each Role loads only its own root configuration and fails startup when a capability used by that Role is missing. Every Role requires MySQL. API requires Redis for local identity, while Job and all task Consumers require Redis Streams.

Repository Role YAML files contain only non-secret settings. Before local startup,
load the development values from `.env.example` into your shell or provide an
equivalent ignored local environment file. Do not copy a working DSN or password
back into `configs/*.yaml`.

Use `APP_CONFIG_FILE` to select a role configuration file. Every field exposed in Role YAML has an explicit `APP_` override. Task dispatch settings use the `APP_TASK_DISPATCH_*` prefix. `STREAM_PREFIX` is shared by publishers and consumers; each Consumer fixes one `STREAM` and `CONSUMER_GROUP`. `READ_BLOCK`, `RECLAIM_INTERVAL`, `CLAIM_MIN_IDLE`, `HANDLER_TIMEOUT`, `CONCURRENCY`, and `MAX_MESSAGE_BYTES` control delivery without changing the Role at runtime.

Identity settings are `APP_IDENTITY_ACCESS_TOKEN_TTL`, `APP_IDENTITY_REFRESH_TOKEN_TTL`, and `APP_IDENTITY_CAPTCHA_TTL`. Initial credentials are secret-only settings: `APP_IDENTITY_BOOTSTRAP_USER` and `APP_IDENTITY_BOOTSTRAP_PASSWORD` must be supplied together and create a ROOT account only when no active account exists. `APP_IDENTITY_DEFAULT_PASSWORD` is required before an administrator can create another account. Both password settings must contain at least eight characters and must not be committed.

Login requests are rate-limited by direct client IP and normalized username through Redis, preventing one noisy account from blocking every user behind the same reverse proxy. Configure the fixed window with `APP_IDENTITY_LOGIN_RATE_LIMIT` and `APP_IDENTITY_LOGIN_RATE_WINDOW`; both values must be positive. The API does not trust forwarded proxy headers by default, so explicitly review the trusted-proxy policy before relying on per-origin-IP limits behind a reverse proxy.

The API also has a transport-level token-bucket limiter for all routes. Configure it with `APP_RESILIENCE_HTTP_RATE_LIMIT_ENABLED`, `APP_RESILIENCE_HTTP_RATE_LIMIT_RPS`, `APP_RESILIENCE_HTTP_RATE_LIMIT_BURST`, and `APP_RESILIENCE_HTTP_RATE_LIMIT_CLIENT_TTL`. It is keyed by the direct client IP because forwarded proxy headers are not trusted. The local API configuration enables it at 100 requests/second with a burst of 200; tune the values and trusted-proxy boundary together for production.

The Job Outbox publisher uses a circuit breaker controlled by `APP_RESILIENCE_CIRCUIT_BREAKER_FAILURE_THRESHOLD`, `APP_RESILIENCE_CIRCUIT_BREAKER_OPEN_TIMEOUT`, and `APP_RESILIENCE_CIRCUIT_BREAKER_HALF_OPEN_MAX`. Rejected `XADD` calls are ordinary publish failures: the Outbox row is marked for retry and is never treated as published.

Local and development Roles share one Redis instance. Task durability never depends on Redis persistence: MySQL is the source of truth, and later business-specific compensation jobs will recreate missing dispatches from nonterminal Tasks. Production Redis persistence and isolation remain deployment decisions.

The API file adapter is selected with `APP_FILE_STORAGE_TYPE`: `local`, `s3`, or `aliyun_oss`. `APP_FILE_STORAGE_MAX_FILE_BYTES` must be positive and no larger than `APP_SERVER_MAX_BODY_BYTES`. Returned content URLs are intentionally public and use opaque generated keys so avatars and notice media can render without adding an authorization header.

Local storage uses `APP_FILE_STORAGE_ROOT` and defaults to `.tmp/uploads`. Production must point it at a persistent writable mount.

S3-compatible storage uses `APP_FILE_STORAGE_S3_ENDPOINT`, `APP_FILE_STORAGE_S3_REGION`, `APP_FILE_STORAGE_S3_BUCKET`, `APP_FILE_STORAGE_S3_ACCESS_KEY`, `APP_FILE_STORAGE_S3_SECRET_KEY`, and `APP_FILE_STORAGE_S3_USE_PATH_STYLE`. The endpoint may be empty for AWS S3. AccessKey and SecretKey may both be empty to use the AWS default credential chain; otherwise both are required. Path-style addressing is useful for MinIO, RustFS, and similar self-hosted implementations.

Aliyun OSS uses `APP_FILE_STORAGE_ALIYUN_OSS_ENDPOINT`, `APP_FILE_STORAGE_ALIYUN_OSS_BUCKET`, `APP_FILE_STORAGE_ALIYUN_OSS_ACCESS_KEY`, and `APP_FILE_STORAGE_ALIYUN_OSS_SECRET_KEY`. All four values are required when `APP_FILE_STORAGE_TYPE=aliyun_oss`.

SSE uses the configured Redis instance for cross-replica Pub/Sub and online-user presence. Reverse proxies must disable response buffering for `/api/v1/sse/connect`, preserve the `Authorization` header, and allow responses longer than the normal request timeout. No SSE payload is written to application logs.

Database-backed system configuration uses a versioned Redis cache. Every configuration mutation advances the cache generation, and a reader may refill only the generation it originally observed. Old generations expire automatically, so cache invalidation does not scan Redis keys and an in-flight stale read cannot repopulate the active generation.

`APP_MYSQL_DSN` is the GORM application DSN. Database schema changes are reviewed and executed manually from the versioned SQL files under `migrations/`; no migration URL or automatic migration runner is part of application configuration.

Migration `20260905000100` changes soft-delete uniqueness to generated active-key columns. Its down migration is valid only while no duplicate deleted business keys have been created. After the new semantics have been used, preserve history and roll forward rather than deleting or renaming records merely to force a rollback.

Production secrets belong in environment or platform secret injection. They must not appear in YAML, logs, health responses, build information, or error messages.

The runtime image contains the repository's non-secret Role YAML files under `/configs`; environment variables supply required connection settings and override other values. `APP_CONFIG_FILE` may point to a mounted alternative, but secrets should still be injected through the deployment platform rather than baked into that file.
