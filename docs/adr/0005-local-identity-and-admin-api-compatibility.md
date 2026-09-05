# Own local identity and preserve the administration API contract

The API Role owns single-tenant Accounts and revocable Sessions locally because the administration client must be able to log in without an upstream identity service. The `/api/v1` endpoints preserve the frontend contract's `code`, `msg`, and `data` response envelope and stable authentication codes while still returning correct HTTP status codes; internal domain, application, and adapter boundaries remain those of this service repository.
