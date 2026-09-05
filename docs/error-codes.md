# Error codes

Error codes are stable uppercase strings scoped to a concern, for example `ORDER_NOT_FOUND` or `ORDER_STATE_CONFLICT`. HTTP status describes the protocol outcome; the error code describes the application outcome.

Do not expose dependency-specific errors. Map duplicate keys, optimistic conflicts, timeouts, and unavailable dependencies to explicit application errors at the adapter boundary.
