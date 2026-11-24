# Cloud Deployment

Cloud mode runs Kode Stream as a hosted control plane behind OAuth2Proxy. OAuth2Proxy is the public endpoint. It
sends users to Keycloak and forwards identity headers to Kode Stream, which listens on a private port and stores
metadata in Postgres. What each workspace mode can do is in the
[Cloud runtime reference](../wiki/platform/cloud-runtime-reference.md). To try it on one machine, use the
[local Cloud stack](docker/local/cloud-mode/README.md).

## Environment

| Variable                                    | Purpose                                                                    |
|---------------------------------------------|----------------------------------------------------------------------------|
| `KODE_STREAM_MODE=cloud`                    | Hosted policy, metadata and agent routing                                  |
| `KODE_STREAM_AUTH_MODE=oauth2_proxy`        | The only Cloud auth mode. `app_oidc` fails startup                          |
| `KODE_STREAM_TRUSTED_PROXY_CIDRS`           | CIDRs of the header-injecting proxy. Catch-all ranges are rejected         |
| `KODE_STREAM_BIND_ADDR=0.0.0.0`             | Bind inside the VM or container                                            |
| `KODE_STREAM_DATA_DIR=/var/lib/kode-stream` | Diagnostics and rollback volume                                            |
| `KODE_STREAM_STORAGE_OPTION=database`       | The only Cloud storage option. `datadir` fails startup                      |
| `KODE_STREAM_STORAGE_DRIVER=postgres`       | Postgres                                                                   |
| `KODE_STREAM_DATABASE_URL`                  | Secret-managed Postgres URL                                                |
| `KODE_STREAM_MIGRATIONS=auto`               | Run embedded migrations at startup. Use `manual` for operator-run ones     |
| `KODE_STREAM_PUBLIC_URL`                    | Public HTTPS URL for browser and agent links                               |
| `KODE_STREAM_COOKIE_SECRET`                 | Random secret for signed fallback session cookies and agent tokens         |
| `KODE_STREAM_ADMIN_USERS`                   | Comma-separated bootstrap admins, by email or subject                      |

Provision Postgres with encrypted storage and backups, and allow only the Kode Stream API and operator tooling to
reach it. Agents never connect to Postgres.

## OAuth2Proxy

Expose OAuth2Proxy and keep the app port private. Point OAuth2Proxy at Keycloak as the OIDC provider and proxy to
`http://kode-stream:4317/` or the private VM address. OAuth2Proxy must:

- Set `X-Auth-Request-User` and `X-Auth-Request-Email`, or `X-Forwarded-User` and `X-Forwarded-Email`.
  `X-Auth-Request-Preferred-Username` or `X-Forwarded-Preferred-Username` is optional.
- Preserve `Host` and `X-Forwarded-Proto`.
- Allow unauthenticated `GET /api/health` and `GET /api/agents/channel` (the agent authenticates with its token).
- Support WebSocket upgrade on `/api/agents/channel`, with a long idle timeout.

Kode Stream never validates access tokens. It trusts identity headers from the trusted proxy CIDRs only. Direct access
to the app port bypasses login and is for local smoke tests only.

## Cloud Agent

The agent is the same `kode-stream` binary, so the Homebrew formula installs it.

```bash
kode-stream agent doctor --cloud-url https://kode-stream.example.com --repo /path/to/repo
kode-stream agent start --connect "kodestream://connect?token=..." --cloud-url https://kode-stream.example.com --repo /path/to/repo
kode-stream agent status
```

`agent start` runs in the foreground. It takes a raw token or a `kodestream://connect` link, connects to
`/api/agents/channel`, sends heartbeats and publishes Git metadata for `--repo`. Pass the signed token exactly as
issued. The Cloud UI launches `kodestream://connect?token=<short-lived-token>`, so the installed package must register
that handler and point it at `kode-stream agent start --connect <url>`.

## Upgrade And Rollback

1. Back up Postgres (managed snapshot or `pg_dump`) and `KODE_STREAM_DATA_DIR`.
2. For manual migrations, run the new binary once with `KODE_STREAM_MIGRATIONS=auto` from an operator job.
3. Deploy the new image (`docker build -f deploy/docker/image/kode-stream/Dockerfile -t kode-stream:<version> .`).
4. Check that `/api/health` reports the database ready and the expected `migrationVersion`.
5. Smoke Agent connection, heartbeat and metadata publication.

To roll back, restore the previous image, the database and the data dir backup.

## Troubleshooting

| Symptom                              | Check                                                                                       |
|--------------------------------------|---------------------------------------------------------------------------------------------|
| Browser shows Local mode             | Requests go through OAuth2Proxy, and the app has `KODE_STREAM_MODE=cloud`                    |
| API exits at startup                 | Storage option, driver, `KODE_STREAM_DATABASE_URL`, Postgres access, auth mode, proxy CIDRs |
| `/api/health` shows database failure | Postgres readiness, credentials and migration version                                       |
| No login page                        | The browser uses the OAuth2Proxy URL, not the app port                                      |
| OIDC login fails                     | Keycloak issuer, OAuth2Proxy client ID and secret, redirect URL, `X-Forwarded-Proto`         |
| Unauthorized after login             | OAuth2Proxy forwards the user or email header, from a trusted CIDR                          |
| Role denied                          | `KODE_STREAM_ADMIN_USERS` and the user's email or subject. Viewers cannot change workspaces  |
| Agent cannot connect                 | WebSocket upgrade on `/api/agents/channel`, and the token has not expired                   |
| Agent channel drops                  | Proxy idle and read timeouts, and heartbeats arriving                                       |
| Deep link does nothing               | The installed package registered `kodestream://connect`                                     |
| Private deployment unreachable       | Use an operator VPN for browser and agent access                                            |
| Command request returns 503          | Expected. Agent command delivery is not implemented                                          |
