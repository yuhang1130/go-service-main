# Runbook

## Startup

Review and manually apply the required versioned SQL files as an independent deployment step, then start the required Role binaries. A service is available only after `/readyz` on its management port returns 200.

For local initialization against an empty identity schema, run `make init-admin`; the interactive script starts a temporary API with one-time credentials, confirms that the ROOT account was created, and stops the process. For deployment, inject `APP_IDENTITY_BOOTSTRAP_USER` and `APP_IDENTITY_BOOTSTRAP_PASSWORD` as one-time secrets. The API creates one ROOT account only when `sys_user` has no active records. Remove the bootstrap password from the runtime environment after the account has been created. Configure `APP_IDENTITY_DEFAULT_PASSWORD` only when administrators need to create users, and rotate that shared initial password operationally.

## Shutdown

The service marks readiness false, stops accepting new work, drains in-flight work until the configured deadline, and then closes external clients. A Consumer must not acknowledge unfinished work.

## Health

- `/livez` checks only that the process is alive.
- `/readyz` checks initialized dependencies required by the Role. Job and Consumer include their RocketMQ client lifecycle; a Job publish failure marks its producer unavailable until a later publish succeeds.
- `/buildinfo` reports version, commit, build time, and Go version without configuration values.

## Manual database changes

The repository does not use an automatic migration runner. Files under `migrations/` are the only schema source and operators execute reviewed `.up.sql` files one at a time in filename order.

This manual rule applies to persistent local, staging, and production databases. CI may initialize a disposable test database with the MySQL client so integration tests can validate the complete schema from scratch; that database is discarded after the workflow.

Before execution:

1. Run `make check-migrations` and `make sql-list`.
2. Confirm the target environment, database name, current schema, backup or recovery point, expected lock impact, and application compatibility.
3. Review the exact SQL and its SHA-256 checksum. Never edit a SQL file after it has been executed in any environment.
4. Use a dedicated, time-limited database change account. Do not place its password in shell history, repository files, or logs.

Execute one reviewed file with the MySQL client, allowing it to prompt for the password:

```bash
mysql --host=<host> --port=<port> --user=<change-user> --password <database> \
  < migrations/<version>_<change>.up.sql
```

After every file, verify the expected tables, columns, indexes, seed data, warnings, and application compatibility before continuing. Record the environment, database, filename, checksum, operator, execution time, result, and verification evidence in the release record. Production rollback is forward-fix by default; execute a `.down.sql` file only after reviewing its data-loss and compatibility impact.
