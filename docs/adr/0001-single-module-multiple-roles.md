# Use one Go module with multiple explicit Role binaries

The service shares domain and application code in one repository while building API, Job, and specialized Consumer Roles as independent binaries. Each Consumer has a fixed responsibility and entry point; this preserves reuse without creating a runtime role switch or coupling deployment lifecycles.
