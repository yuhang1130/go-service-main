# Use one Go module with multiple explicit Role binaries

The service shares domain and application code in one repository while building API, Job, and Consumer as independent binaries. This preserves reuse without creating a runtime role switch or coupling deployment lifecycles.
