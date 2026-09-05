# Use Transactional Outbox and Inbox for at-least-once messaging

Business state and Outbox events commit in one MySQL transaction, while the Job Role relays events to RocketMQ outside that transaction. Consumers commit Inbox final state together with database-only business changes, accepting broker duplicates rather than depending on exactly-once delivery.
