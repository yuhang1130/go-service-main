---
status: accepted
---

# Use Redis Streams for durable task dispatch

Redis Streams replaces RocketMQ as the low-latency dispatch channel for media-delivery Tasks, while MySQL remains the source of truth. Task creation and a shared `task_dispatch_outbox` record commit atomically; the creator attempts publication after commit, and the Job Role retries unpublished records and later recovers nonterminal business Tasks. Consumers conditionally claim execution ownership in MySQL and then acknowledge the Stream entry, so duplicate delivery is harmless and MySQL leases—not Redis pending-entry age—govern long-running execution.

Collection, transformation, upload, and infrastructure dispatch use separate Streams and independently built Consumer Role binaries. Local and development environments share the existing Redis instance and do not rely on Redis persistence for task durability; losing Redis data may delay work until the next MySQL-backed compensation scan. RocketMQ and the old generic event Inbox/Outbox tables are removed through a new versioned migration, while transactional publication and idempotent at-least-once execution guarantees remain mandatory.

Implementation is tracked in [the Redis Stream task-dispatch infrastructure plan](../plans/2026-09-10-redis-stream-task-dispatch-infrastructure.md).
