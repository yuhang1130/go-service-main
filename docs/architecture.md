# Architecture

This Service Repository is one Go module with three separately compiled Roles: API, Job, and Consumer. Each `cmd/<role>` delegates to an explicit composition root in `internal/bootstrap`; there is no runtime role flag or dependency-injection container.

## Dependency direction

```text
cmd -> bootstrap -> adapters -> application -> domain
                         \-> foundation
```

- `internal/features/<feature>/domain` owns business entities, value objects, state transitions, and invariants.
- `internal/features/<feature>/application` owns use cases and defines the ports those use cases consume.
- `internal/adapters` implements transport and infrastructure concerns without becoming the owner of business rules.
- `internal/foundation` contains small cross-cutting primitives shared across Features and Roles.
- `internal/bootstrap` is the only place that constructs concrete dependency graphs and owns process lifecycle.

Domain and application packages must not import transport or infrastructure SDKs. Repository interfaces are shaped by their consuming use cases rather than by database tables.

## Process lifecycle

Each Role explicitly constructs a `lifecycle.Manager` from named services. A service declares only its name, dependencies, startup, shutdown, and readiness functions. The manager validates missing dependencies and cycles before startup, starts independent services concurrently by dependency layer, and stops successfully started services in reverse layer order.

There is no global service registry or runtime package scanning. The current Role graphs are:

```text
API:      mysql     redis

Job:      mysql ------\
          rocketmq -----> scheduler

Consumer: mysql -> rocketmq
```

If one layer fails, services already started in that layer and all preceding layers are rolled back. On process cancellation, readiness is cleared before HTTP draining completes; external clients are closed only after the management/application servers have stopped accepting work.

Every lifecycle service contributes a readiness check. `/readyz` snapshots and runs those checks concurrently under one bounded deadline, without holding the registry lock or exposing dependency errors in the response.

## Resilience

- The API installs a per-client in-memory token-bucket limiter in the HTTP adapter. It returns HTTP 429, stable code `TOO_MANY_REQUESTS`, and `Retry-After`; the existing Redis-backed username/IP login limiter remains a separate identity control.
- The Job RocketMQ producer wraps Outbox publication with a closed/open/half-open circuit breaker. An open producer makes Job readiness fail, while the durable Outbox retry path retains the event for a later attempt.

## Role boundaries

- API accepts synchronous requests and maps protocol DTOs to application commands and queries.
- Job discovers durable work, performs bounded scheduled maintenance, and compensates recoverable failures.
- Consumer executes event-triggered work under at-least-once delivery semantics.

A Feature may be used by several Roles, but each Role constructs its own dependencies and can be built, deployed, scaled, stopped, or rolled back without starting another Role.

Long operations are persisted and delivered through Transactional Outbox. Consumer database changes and Inbox final state commit in one transaction; external side effects do not occur inside that transaction. Cron callbacks remain short, bounded, and idempotent.

Business Features that publish events commit business state and an Outbox event atomically. The Job Role relays those events outside the transaction, and the Consumer commits database changes together with the corresponding Inbox final state.

The repository currently provides the event-delivery foundation but does not
register a concrete business Consumer handler. A Consumer with no handlers
fails startup instead of reporting a misleading ready state. Add an explicit
Feature registration before deploying that Role.
