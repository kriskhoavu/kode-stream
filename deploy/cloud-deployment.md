# Cloud Deployment

Cloud mode runs Kode Stream as a hosted control plane behind OAuth2Proxy. OAuth2Proxy is the public endpoint. It sends
users to Keycloak and forwards identity headers to Kode Stream. Kode Stream listens on a private port and stores
metadata in Postgres. Workspace mode capabilities are in the
[Cloud runtime reference](../wiki/platform/cloud-runtime-reference.md). To try it on one machine, use the
[local Cloud stack](docker/local/cloud-mode/README.md).

## Environment

| Variable                                    | Purpose                                                                      |
|---------------------------------------------|------------------------------------------------------------------------------|
| `KODE_STREAM_MODE=cloud`                    | Hosted policy, metadata and agent routing                                    |
| `KODE_STREAM_AUTH_MODE=oauth2_proxy`        | The only Cloud auth mode. `app_oidc` fails startup                           |
| `KODE_STREAM_TRUSTED_PROXY_CIDRS`           | Required. CIDRs of the header-injecting proxy. Catch-all ranges fail startup |
| `KODE_STREAM_BIND_ADDR=0.0.0.0`             | Bind inside the VM or container                                              |
| `KODE_STREAM_DATA_DIR=/var/lib/kode-stream` | Diagnostics and rollback volume                                              |
| `KODE_STREAM_STORAGE_OPTION=database`       | The only Cloud storage option. `datadir` fails startup                       |
| `KODE_STREAM_STORAGE_DRIVER=postgres`       | Postgres                                                                     |
| `KODE_STREAM_DATABASE_URL`                  | Secret-managed Postgres URL                                                  |
| `KODE_STREAM_MIGRATIONS=auto`               | Run embedded migrations at startup. Use `manual` for operator-run ones       |
| `KODE_STREAM_PUBLIC_URL`                    | Required. Public HTTPS URL for browser and agent links                       |
| `KODE_STREAM_COOKIE_SECRET`                 | Required. Signs fallback session cookies and agent tokens                    |
| `KODE_STREAM_ADMIN_USERS`                   | Required. Comma-separated bootstrap admins, by email or subject              |

Give Postgres encrypted storage and backups. Only the Kode Stream API and operator tooling may reach it. Agents never
connect to Postgres.

## OAuth2Proxy

Expose OAuth2Proxy and keep the app port private. Point OAuth2Proxy at Keycloak as the OIDC provider and proxy to
`http://kode-stream:4317/` or the private VM address. OAuth2Proxy must:

1. Set `X-Auth-Request-User` and `X-Auth-Request-Email`, or `X-Forwarded-User` and `X-Forwarded-Email`. The
   `X-Auth-Request-Preferred-Username` or `X-Forwarded-Preferred-Username` header is optional.
2. Preserve `Host` and `X-Forwarded-Proto`.
3. Skip login for `GET /api/health`, `GET /api/agents/channel` and `POST /api/workspaces/from-agent`. The agent
   authenticates those with its token.
4. Allow WebSocket upgrade on `/api/agents/channel`, with a long idle timeout.

Kode Stream never validates access tokens. It trusts identity headers only from the trusted proxy CIDRs. Direct access
to the app port bypasses login. Use it only for local smoke tests.

## Cloud Agent

The agent is the same `kode-stream` binary, so the Homebrew formula installs it.

```bash
kode-stream agent doctor --cloud-url https://kode-stream.example.com --repo /path/to/repo
kode-stream agent start --connect "kodestream://connect?token=..." --cloud-url https://kode-stream.example.com --repo /path/to/repo
kode-stream agent status
```

`agent start` runs in the foreground. It takes a raw token or a `kodestream://connect` link, unchanged. It connects to
`/api/agents/channel`, sends heartbeats and publishes Git metadata for `--repo`. The Cloud UI launches
`kodestream://connect?token=<short-lived-token>`. The installed package must register that handler to run
`kode-stream agent start --connect <url>`.

## Upgrade And Rollback

1. Back up Postgres (managed snapshot or `pg_dump`) and `KODE_STREAM_DATA_DIR`.
2. If migrations are `manual`, run the new binary once with `KODE_STREAM_MIGRATIONS=auto` from an operator job.
3. Build and deploy the new image:

   ```bash
   npm run build
   docker build -f deploy/docker/image/kode-stream/Dockerfile -t kode-stream:<version> .
   ```

4. Check that `/api/health` reports the database ready and the expected `migrationVersion`.
5. Check agent connection, heartbeat and metadata publication.

To roll back, restore the previous image, the database backup and the data dir backup.

## Troubleshooting

| Symptom                              | Check                                                                                       |
|--------------------------------------|---------------------------------------------------------------------------------------------|
| Browser shows Local mode             | Requests go through OAuth2Proxy, and the app has `KODE_STREAM_MODE=cloud`                   |
| API exits at startup                 | The startup error names the missing or invalid variable. Then check Postgres access         |
| `/api/health` shows database failure | Postgres readiness, credentials and migration version                                       |
| No login page                        | The browser uses the OAuth2Proxy URL, not the app port                                      |
| OIDC login fails                     | Keycloak issuer, OAuth2Proxy client ID and secret, redirect URL, `X-Forwarded-Proto`        |
| Unauthorized after login             | OAuth2Proxy forwards the user or email header, from a trusted CIDR                          |
| Role denied                          | `KODE_STREAM_ADMIN_USERS` and the user's email or subject. Viewers cannot change workspaces |
| Agent cannot connect                 | WebSocket upgrade on `/api/agents/channel`, and the token has not expired                   |
| Agent channel drops                  | Proxy idle and read timeouts, and heartbeats arriving                                       |
| Deep link does nothing               | The installed package registered `kodestream://connect`                                     |
| Private deployment unreachable       | Use an operator VPN for browser and agent access                                            |
| Command request returns 503          | Expected. Agent command delivery is not implemented                                         |
