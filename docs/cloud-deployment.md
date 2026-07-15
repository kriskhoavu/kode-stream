# Cloud Deployment

Cloud mode runs Kode Stream as a hosted control plane behind OAuth2Proxy. The public endpoint is OAuth2Proxy, which
redirects users to Keycloak and forwards authenticated identity headers to Kode Stream. Kode Stream itself stays on a
private VM/container port, stores metadata under `KODE_STREAM_DATA_DIR`, and routes workspace commands to the user's
Cloud Agent. Cloud v1 does not require a database service.

## Required Environment

| Variable                                    | Purpose                                                        |
|---------------------------------------------|----------------------------------------------------------------|
| `KODE_STREAM_MODE=cloud`                    | Enables hosted policy, metadata, and agent routing.            |
| `KODE_STREAM_AUTH_MODE=oauth2_proxy`        | Trusts identity headers from the private OAuth2Proxy upstream. |
| `KODE_STREAM_BIND_ADDR=0.0.0.0`             | Binds inside the VM or container.                              |
| `KODE_STREAM_DATA_DIR=/var/lib/kode-stream` | Persistent file-backed metadata volume.                        |
| `KODE_STREAM_PUBLIC_URL`                    | Public HTTPS URL used for browser and agent links.             |
| `KODE_STREAM_COOKIE_SECRET`                 | Random secret for signed fallback Cloud session cookies.       |
| `KODE_STREAM_ADMIN_USERS`                   | Comma-separated bootstrap admins by email or subject.          |

`KODE_STREAM_AUTH_MODE` defaults to `oauth2_proxy` in Cloud mode. App-owned OIDC remains available for development or
alternate deployments by setting `KODE_STREAM_AUTH_MODE=app_oidc` and providing `KODE_STREAM_OIDC_ISSUER`,
`KODE_STREAM_OIDC_CLIENT_ID`, and `KODE_STREAM_OIDC_CLIENT_SECRET`.

## OAuth2Proxy And Keycloak

Expose OAuth2Proxy to the internet and keep the Kode Stream app port private. OAuth2Proxy should use Keycloak as its
OIDC provider, then proxy authenticated requests to `http://kode-stream:4317/` or the equivalent private VM address.

Required OAuth2Proxy behavior:

- Set user identity headers. Kode Stream accepts `X-Auth-Request-User` and `X-Auth-Request-Email`, or OAuth2Proxy's
  forwarded user headers such as `X-Forwarded-User` and `X-Forwarded-Email`.
- Optionally set `X-Auth-Request-Preferred-Username` or `X-Forwarded-Preferred-Username`.
- Optionally pass the access token or authorization header for future validation. PM-032 does not introspect or validate
  opaque/JWT tokens inside Kode Stream.
- Preserve `Host` and `X-Forwarded-Proto`.
- Support WebSocket upgrade for `/api/agents/channel`.
- Allow unauthenticated `GET /api/health` for deployment health checks.
- Use long idle timeouts for terminal, AI, runtime, and verification streams.
- Forward `/api/health` for health checks.

Only expose Kode Stream directly for local smoke tests. Direct access to `http://127.0.0.1:4317` or a Docker port mapped
straight to Kode Stream bypasses OAuth2Proxy and is not the Cloud login entry point.

## Metadata

Mount `KODE_STREAM_DATA_DIR` as a persistent volume. Back up the whole directory before upgrades. Cloud stores users,
workspaces, agent state, audit logs, and published summaries there. It does not clone repositories, store SSH keys, or
run hosted workspace terminals.

## Smoke Check

```bash
npm run build
go build -o ./bin/kode-stream ./cmd/kode-stream
docker build -t kode-stream:cloud .
docker compose -f deploy/cloud/compose.yaml up -d
curl -fsS http://127.0.0.1:4318/api/health
```

Open the public OAuth2Proxy URL, for example `https://kode-stream.example.com`, to see the login page. In the sample
compose file, local port `4318` is OAuth2Proxy and the app port is not published. After login, connect a Cloud Agent and
register a workspace from the agent. Command-capable actions should be unavailable until the owner agent is connected.

## Troubleshooting

| Symptom                              | Check                                                                                                     |
|--------------------------------------|-----------------------------------------------------------------------------------------------------------|
| Browser still shows Local mode       | Confirm requests go through OAuth2Proxy and the app container has `KODE_STREAM_MODE=cloud`.               |
| Login page is not shown              | Confirm the browser is using the OAuth2Proxy URL, not the private Kode Stream app port.                   |
| OIDC login fails                     | Confirm Keycloak issuer, OAuth2Proxy client id, client secret, redirect URL, and `X-Forwarded-Proto`.     |
| User is unauthorized after login     | Confirm OAuth2Proxy forwards `X-Auth-Request-User` or `X-Auth-Request-Email` to Kode Stream.              |
| Agent cannot connect                 | Confirm `/api/agents/channel` supports WebSocket upgrade and the connect token has not expired.           |
| Deep link does nothing               | Confirm `kodestream://connect` is registered by the installed Cloud Agent package.                        |
| Role denial                          | Confirm `KODE_STREAM_ADMIN_USERS` and the user's OIDC email or subject. Viewers cannot mutate workspaces. |
| WebSocket closes during commands     | Increase reverse proxy idle/read timeouts.                                                                |
| Private deployment cannot be reached | Use an operator VPN such as Tailscale for browser and agent outbound access.                              |
| Command button disabled              | Confirm the workspace owner Cloud Agent is connected and the user's role has the required capability.     |

## Upgrade And Rollback

Back up `KODE_STREAM_DATA_DIR`, deploy the new image, run `/api/health`, then smoke Cloud Agent connection and an
agent-backed workspace. Roll back by restoring the previous image and metadata backup.
