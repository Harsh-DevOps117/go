# Contributing

Run `make check` and `make test-integration` before opening a pull request. The
integration target requires Docker and uses disposable containers. Do not point
test environment variables at a production database.

Keep HTTP concerns in `internal/api` and business rules in `internal/core`. Every
resource query must constrain both its ID and authenticated owner. A foreign-owned
resource must remain indistinguishable from a missing resource. Never accept a
user ID from a request body as the acting identity.

Add regression tests for behavior changes, especially authorization, concurrent
writes, and token lifecycle changes. Prefer PostgreSQL integration tests for
database behavior rather than mocking SQL. Document new endpoints and response
fields in `docs/openapi.json` and the README.

Apply `gofmt`. Write comments explaining why an invariant exists or how a race is
avoided; avoid comments that repeat the next line. Return errors to the HTTP layer
instead of returning untyped success/error maps from services.

Add numbered SQL migration files and increment `database.LatestVersion` when the
schema changes. Do not edit an already deployed migration. Keep migrations
transaction-safe and consider data preservation and rolling deployments.

Never commit `.env`, credentials, database exports, or tokens. `make setup` creates
fresh local credentials without overwriting an existing configuration.
