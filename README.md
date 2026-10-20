# NoteFlow

A Go/Gin API for private notes and persistent reminders. PostgreSQL stores users,
sessions, notes, and alarms. Optional Redis shares rate limits between instances.

The API includes registration, login, refresh-token rotation, immediate session
revocation on logout, note CRUD, alarm CRUD, cursor pagination, and a worker that
marks due alarms as `fired`. Clients retrieve reminders by polling the API.
Email/SMS delivery, a frontend, admin roles, and password recovery are **not implemented**.

## Quick start

Requirements: Docker with Compose, Make, and OpenSSL. Go is only needed to run
outside Docker or execute tests locally; the minimum version is in `go.mod`.

```sh
git clone https://github.com/Harsh-DevOps117/go.git
cd go
make setup
make up
curl --fail http://localhost:3000/health/ready
```

`make setup` generates random local credentials in a mode-600, Git-ignored `.env`.
It never overwrites an existing file. `.env.example` documents the variables.
The application reads its process environment; it does not implicitly load files.
Compose and `make run` load `.env` explicitly.

The API binds to localhost through Compose. PostgreSQL and Redis are also exposed
only on localhost for development. `make down` stops the stack and keeps notes in
the database volume. For a production deployment, supply managed secrets, TLS,
backups, and private database networking; see [deployment notes](docs/deployment.md).

### Run Go locally

```sh
make setup
docker compose up -d postgres redis --wait
make run
```

The entry point is `cmd/server/main.go`. Gin routes and HTTP middleware live in
`internal/api`; business rules live in `internal/core`. Partial updates use `PATCH`.

## Use the API

All resource routes start with `/api/v1`. Request bodies use
`Content-Type: application/json`. Successful responses use `{"data": ...}`;
deletion/logout returns HTTP 204 with no body.

```sh
curl --fail-with-body http://localhost:3000/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"Harsh","email":"harsh@example.com","password":"a-long-example-password"}'

curl --fail-with-body http://localhost:3000/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"harsh@example.com","password":"a-long-example-password"}'
```

Copy `data.access_token` from login into `ACCESS_TOKEN` in your shell:

```sh
curl --fail-with-body http://localhost:3000/api/v1/notes \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"title":"Learn Gin","content":"Write a tested API."}'

curl --fail-with-body 'http://localhost:3000/api/v1/notes?limit=20' \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

List responses contain `items` and `next_cursor`. Pass a non-null cursor as
`?before=<next_cursor>` to retrieve the next page. Results are ordered by descending
ID; page sizes range from 1 to 100 (default 20). Alarms additionally accept
`?status=scheduled` or `?status=fired`. Missing and foreign-owned IDs both return 404.

| Method | Path | Purpose |
| --- | --- | --- |
| POST | `/api/v1/auth/register` | Create a user (201) |
| POST | `/api/v1/auth/login` | Create a session and tokens |
| POST | `/api/v1/auth/refresh` | Rotate a single-use refresh token |
| POST | `/api/v1/auth/logout` | Revoke the authenticated session (204) |
| GET | `/api/v1/me` | Retrieve the authenticated user |
| GET, POST | `/api/v1/notes` | List or create notes |
| GET, PATCH, DELETE | `/api/v1/notes/{id}` | Read, update, or delete a note |
| GET, POST | `/api/v1/alarms` | List or create alarms |
| GET, PATCH, DELETE | `/api/v1/alarms/{id}` | Read, reschedule, or delete an alarm |
| GET | `/health/live` | Process liveness |
| GET | `/health/ready` or `/health` | Database connectivity |

Logout, `/me`, notes, and alarms require a bearer access token. The complete
request/response schemas are in [the OpenAPI specification](docs/openapi.json).

### Authentication behavior

- Passwords use bcrypt cost 12. Registration requires at least 12 Unicode
  characters and at most 72 bytes; email addresses are trimmed and lowercased.
- Access tokens last up to 15 minutes. Validation pins HS256, issuer, audience,
  expiration, issued-at time, user ID, and a live database session.
- Refresh tokens are cryptographically random opaque strings, not JWTs. Only
  SHA-256 hashes are stored. `POST /api/v1/auth/refresh` accepts
  `{"refresh_token":"..."}` and returns a replacement pair. Discard the old token.
- A refresh token can succeed once, even under concurrent requests. Clients
  should serialize refresh requests. If the rotation response is lost, log in
  again; replaying the consumed token is rejected.
- Sessions expire seven days after login; refresh does not extend that deadline.
  Previously issued access tokens remain valid until their expiry or session
  logout. Logout revokes that session immediately, including its refresh token.
- Tokens are returned in JSON; this API does not set authentication cookies.
  Browser clients need an appropriate token-storage strategy and HTTPS.

### Reminders

Create an alarm with `{"note_id":1,"alarm_time":"<future RFC3339 timestamp>"}`.
Dates are stored in UTC. A worker checks every second and marks due reminders as
`fired`, with `fired_at` recording when they were processed. After downtime, it
catches up in batches of 500. Multiple instances can cooperate safely.

Rescheduling an alarm resets it to `scheduled`. Deleting a note deletes its alarms
in the same database operation. The worker updates database state; it does not send
email, SMS, push notifications, or webhooks.

### Error format and limits

```json
{
  "error": {
    "code": "validation_error",
    "message": "invalid input: title must contain 1 to 200 characters",
    "request_id": "..."
  }
}
```

Errors use 400 (validation), 401 (credentials), 404 (missing/foreign resource),
409 (duplicate email), 413 (body size), 429 (rate limit), or appropriate 5xx status.
Internal database errors are not returned to clients. Every response carries
`X-Request-ID` and `Cache-Control: no-store`.

JSON bodies are limited to 1 MiB; unknown fields and trailing JSON are rejected.
Explicit `null` fields are rejected; omit optional fields instead.
Titles allow 1–200 characters, and note content allows 100,000 bytes. Empty content
is allowed. Omitted PATCH fields stay unchanged; `{"content":""}` clears content.

Limits are 120 API requests per minute per IP and 10 registration/login/refresh
requests per minute per IP. Redis shares budgets across replicas. When Redis is
unavailable, bounded local limits remain active and the server logs the transition.
That fallback is per instance; it does not preserve a global limit during an outage.
Proxy headers are ignored unless `TRUSTED_PROXIES` explicitly identifies your proxy.

## Verify changes

```sh
make check             # formatting, vet, race-enabled unit tests, build
make test-integration  # disposable PostgreSQL + Redis; full race-enabled suite
make audit             # known reachable vulnerabilities in dependencies and Go
docker build -t noteflow .
```

Integration tests skip unless `TEST_DATABASE_URL` / `TEST_REDIS_URL` are supplied.
`make test-integration` provisions disposable containers on random localhost ports,
runs the suite with coverage, and removes those containers. It never uses the
normal Compose database. Tests isolate their PostgreSQL schemas and clean them up.

Coverage includes cross-user authorization, refresh replay and concurrent rotation,
duplicate registration races, logout revocation, JWT validation, pagination,
concurrent partial updates, reminder processing, cascade deletion, HTTP error
semantics, migration repeatability, proxy spoofing, and Redis fallback/shared limits.
CI runs the full suite against PostgreSQL and Redis on pull requests and pushes to
`master` or `main`, then builds the container.

## Configuration

| Variable | Default / requirement |
| --- | --- |
| `APP_ENV` | `development`; also accepts `production` or `test` |
| `PORT` | `3000` |
| `DATABASE_URL` | Required PostgreSQL connection URL |
| `JWT_SECRET` | Required, at least 32 bytes; generate randomly |
| `REDIS_URL` | Optional; empty means local rate limits |
| `AUTO_MIGRATE` | `true` in development/test, `false` in production |
| `TRUSTED_PROXIES` | Empty; comma-separated trusted IPs/CIDRs |
| `POSTGRES_PASSWORD` | Compose/setup variable, not read by the Go application |

## Database migrations and upgrading

SQL migrations are embedded in the binary, versioned in `internal/database/migrations`,
and applied transactionally under a PostgreSQL advisory lock. Startup checks the
schema version. To apply migrations explicitly, run `bin/noteflow migrate` with
the same environment used by the server. The local Compose stack enables automatic
migrations for convenience.

**This rebuild changes the schema and API.** The baseline migration requires a
fresh database. Existing unversioned NoteFlow installations need an explicit data
export/mapping/import; the application will fail instead of silently altering old
tables. Keep backups and old volumes until an upgrade is verified. Old endpoints
such as `/v1/login` and `/notes` are replaced by the documented `/api/v1/...` routes.
The old `SECRET_KEY_ACCESSTOKEN` / `SECRET_KEY_REFRESH` settings are replaced by
`JWT_SECRET` and opaque refresh tokens. Existing sessions must log in again.

## Design

```text
cmd/server              configuration, lifecycle, worker
internal/api            Gin routes, middleware, JSON and HTTP mapping
internal/core           authentication, ownership, validation, persistence operations
internal/database       PostgreSQL connections and embedded SQL migrations
internal/ratelimit      Redis budgets with bounded local fallback
internal/testutil       isolated integration-test databases
```

Resource reads go to PostgreSQL rather than an application cache. This keeps
ownership checks, deletions, and updates consistent and removes the original
cache-hit authorization bug. Composite foreign keys protect note/alarm ownership;
unique email constraints protect concurrent registrations. SQL indexes support
owner-scoped pagination and due-alarm scans.

Comments focus on invariants, concurrency, and security decisions. See
[CONTRIBUTING.md](CONTRIBUTING.md) for change guidelines and [SECURITY.md](SECURITY.md)
for credential handling. Licensed under [MIT](LICENSE).
