# Security

Do not post active credentials, access tokens, or private user data in issues.
Use GitHub's private vulnerability reporting for this repository if enabled;
otherwise contact the maintainer privately before disclosing exploit details.

The previous repository committed an `.env` file. Removing it from the current
tree does not remove it from Git history. Rotate every old credential used in a
real environment. Any history cleanup must be coordinated separately with all
repository collaborators; this rebuild does not rewrite published history.

`make setup` creates new random local credentials. Keep `.env` outside commits and
container build contexts. JWT key changes invalidate existing access tokens;
revoking rows in `sessions` invalidates the corresponding access and refresh tokens.
Refresh tokens are stored only as SHA-256 hashes; passwords use bcrypt cost 12.

Authorization is enforced on every database query and by ownership foreign keys.
The server deliberately avoids caching private resources. Rate limiting is an
abuse mitigation, not a substitute for secure network configuration or monitoring.

See [deployment notes](docs/deployment.md) for production boundaries and currently
unimplemented account-management features.
