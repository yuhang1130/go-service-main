# Project instructions

- Keep compatibility with Go 1.26 and run `make ci` before committing.
- Roles are independent binaries; never add a runtime role switch.
- Put business rules in `internal/features/<feature>/domain` and use cases plus consumer-owned ports in `internal/features/<feature>/application`.
- Keep Gin, GORM, Redis, RocketMQ, and gocron types inside `internal/adapters` or `internal/bootstrap`.
- Use explicit constructor wiring. Do not add runtime dependency injection or package scanning.
- GORM models are not domain entities or HTTP DTOs. Do not call `Save`, do not perform unscoped updates, and check `Error` and `RowsAffected`.
- Versioned SQL files under `migrations/` are the only schema source. Operators apply reviewed SQL manually as a separate deployment step; never call `AutoMigrate` from a service or add an automatic migration runner.
- Do not make network, Redis, or MQ calls inside a database transaction.
- API errors use correct HTTP status codes and stable string codes. Never return SQL errors, stack traces, or secrets.
- Logs use `slog` and stdout. Do not add application file rotation, request/response bodies, or message payloads to default logs.
- Consumers are at-least-once. Preserve Inbox idempotency and Transactional Outbox guarantees.
- Never commit real DSNs, tokens, passwords, access keys, or production endpoints.
