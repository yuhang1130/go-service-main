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

## Role boundaries

- API accepts synchronous requests and maps protocol DTOs to application commands and queries.
- Job discovers durable work, performs bounded scheduled maintenance, and compensates recoverable failures.
- Consumer executes event-triggered work under at-least-once delivery semantics.

A Feature may be used by several Roles, but each Role constructs its own dependencies and can be built, deployed, scaled, stopped, or rolled back without starting another Role.

Long operations are persisted and delivered through Transactional Outbox. Consumer database changes and Inbox final state commit in one transaction; external side effects do not occur inside that transaction. Cron callbacks remain short, bounded, and idempotent.

Business Features that publish events commit business state and an Outbox event atomically. The Job Role relays those events outside the transaction, and the Consumer commits database changes together with the corresponding Inbox final state.
