# Cloud Deployment

Cloud mode runs Kode Stream as a hosted control plane. It stores metadata under `KODE_STREAM_DATA_DIR` and routes
workspace commands to the user's Cloud Agent. Cloud v1 does not require a database service.

## Required Environment

| Variable | Purpose |
| --- | --- |
| `KODE_STREAM_MODE=cloud` | Enables hosted auth, policy, metadata, and agent routing. |
| `KODE_STREAM_BIND_ADDR=0.0.0.0` | Binds inside the VM or container. |
| `KODE_STREAM_DATA_DIR=/var/lib/kode-stream` | Persistent file-backed metadata volume. |
| `KODE_STREAM_PUBLIC_URL` | Public HTTPS URL used for OIDC redirects and agent links. |
| `KODE_STREAM_COOKIE_SECRET` | Random secret for signed Cloud session cookies. |
| `KODE_STREAM_OIDC_ISSUER` | OIDC provider issuer URL. |
| `KODE_STREAM_OIDC_CLIENT_ID` | OIDC client id. |
| `KODE_STREAM_OIDC_CLIENT_SECRET` | OIDC client secret. |
| `KODE_STREAM_ADMIN_USERS` | Comma-separated bootstrap admins by email or subject. |

## Reverse Proxy

Terminate TLS at the proxy and forward to the app container on port `4317`.

Proxy requirements:

- Preserve `Host`, `X-Forwarded-Proto`, and identity headers used by the OIDC integration.
- Support WebSocket upgrade for `/api/agents/channel`.
- Use long idle timeouts for terminal, AI, runtime, and verification streams.
- Forward `/api/health` for health checks.

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
curl -fsS http://127.0.0.1:4317/api/health
```

After login, connect a Cloud Agent and register a workspace from the agent. Command-capable actions should be unavailable
until the owner agent is connected.
