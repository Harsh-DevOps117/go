# Deployment notes

The supplied Compose stack is a local demonstration, not a complete production
deployment. It includes persistent PostgreSQL storage, readiness checks, a non-root
application container, a read-only application filesystem, dropped capabilities,
and localhost-only published ports.

For a hosted deployment:

1. Terminate HTTPS at a trusted reverse proxy. Set `TRUSTED_PROXIES` to its actual
   IPs/CIDRs; do not trust every client. Keep direct access to the application private.
2. Supply a randomly generated `JWT_SECRET` and database credentials through your
   secret manager. Use PostgreSQL TLS (`sslmode=verify-full`) with the appropriate
   certificate trust, and `rediss://` with authentication for remote Redis.
3. Keep database and Redis ports off the public network. Configure database backups,
   test restoration, and use a separate database/user for each environment.
4. Run `noteflow migrate` as a release step with migration privileges. Run the API
   with `AUTO_MIGRATE=false` and a database role limited to runtime operations.
   Back up before schema changes. Migrations are forward-only; there is no automatic
   destructive rollback command.
5. Probe `/health/live` for liveness and `/health/ready` for database readiness.
   Redis is optional, so an outage degrades rate limits without failing readiness.
   Alert on Redis fallback, worker failures, elevated 5xx responses, and database
   connection saturation. Logs are structured JSON with generated request IDs.
6. Allow at least 15 seconds for termination. The server drains HTTP requests for
   up to 10 seconds, stops the worker, and closes Redis and database connections.

Per-instance database pools allow 20 open connections and 5 idle connections. Size
the database for the number of replicas before scaling. A request context expires
after five seconds; HTTP header, read, write, and idle timeouts also bound connections.
The reminder worker processes at most 500 due reminders per second per iteration.
Measure workload behavior before choosing latency or throughput guarantees.

Redis rate limiting is shared when available and per-instance during an outage.
Fixed windows can permit bursts at window boundaries. For an Internet-facing
service, combine these limits with proxy-level traffic controls and monitoring.
The API has no email verification, password reset, MFA, or account-deletion workflow;
plan those features before offering a public account service.

There is no CORS allowlist because this repository contains an API only. Serve a
future browser frontend from the same origin or add a narrowly configured policy.
Token storage and refresh synchronization belong to that client's security design.

Pin container images by digest in your release process and regularly review
dependency updates. A successful build and test suite are evidence of correctness
for tested cases, not a substitute for operational monitoring or security review.
