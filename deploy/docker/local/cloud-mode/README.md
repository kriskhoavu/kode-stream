# Local Cloud Stack

This local stack mirrors the VM deployment shape:

- OAuth2Proxy is the browser entry point.
- Keycloak provides the login page.
- Kode Stream runs in Cloud mode on a private Docker network port.

## Run

The stack supports both Cloud workspace access modes. Both start OAuth2Proxy, Keycloak, Kode Stream, and Postgres with
Cloud `database` storage. The mode determines whether the helper also starts a local Cloud Agent.

| Helper mode               | Command                                                                          | What it validates                                                                                                 |
|---------------------------|----------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------|
| Agent-Backed              | `./deploy/docker/local/cloud-mode/run.sh`                                            | Cloud control plane plus a foreground Agent for a local repository                                                |
| Agentless Remote Snapshot | `KODE_STREAM_CLOUD_WORKSPACE_MODE=agentless ./deploy/docker/local/cloud-mode/run.sh` | Cloud control plane without an Agent; supports backend Remote Snapshot verification with preconfigured test state |

## Agent-Backed Run

```bash
./deploy/docker/local/cloud-mode/run.sh
```

By default, this starts the Docker stack, waits for `http://kode-stream.localhost:4318/api/health`, builds
`./bin/kode-stream`, generates a 30-minute local agent token with the development cookie secret, and starts the agent
against the current repository.

Optional overrides:

```bash
KODE_STREAM_STORAGE_OPTION=database \
KODE_STREAM_AGENT_REPO=/path/to/repo \
KODE_STREAM_AGENT_NAME="MacBook Agent" \
./deploy/docker/local/cloud-mode/run.sh
```

Cloud smoke runs always use `database` storage with Postgres. `deploy/docker/local/cloud-mode/run.sh` fails early if
`KODE_STREAM_STORAGE_OPTION=datadir` is supplied. Postgres persists shared metadata in `kode-stream-postgres`; the
separate `kode-stream-cloud-data` volume persists Cloud diagnostics and rollback exports under `KODE_STREAM_DATA_DIR`.

The agent runs in the foreground. Press `Ctrl-C` to stop the agent; Docker services stay running.

## Agentless Remote Snapshot Run

Start only the Cloud control plane:

```bash
KODE_STREAM_CLOUD_WORKSPACE_MODE=agentless ./deploy/docker/local/cloud-mode/run.sh
```

The helper waits for health and exits without building or starting an Agent. The current Remote Snapshot foundation
requires preconfigured provider and workspace test state; the self-service provider connection, registration, and
snapshot-backed UI are not yet available. Verify commit-pinned metadata, tree, and file responses and confirm that
write, Git, terminal, AI, runtime, and verification actions are unavailable. See [Remote Snapshot operations](../../../../docs/domain/cloud/remote-snapshot-operations.md).

Manual stack startup:

```bash
docker compose -f deploy/docker/local/cloud-mode/compose.yaml up -d --build
```

Open:

```text
http://kode-stream.localhost:4318
```

Health check through OAuth2Proxy:

```bash
curl -fsS http://kode-stream.localhost:4318/api/health
```

## Agent CLI

Build local binary if needed:

```bash
go build -o ./bin/kode-stream ./cmd/kode-stream
```

Run doctor/status checks:

```bash
./bin/kode-stream agent doctor --cloud-url http://kode-stream.localhost:4318 --repo .
./bin/kode-stream agent status
```

Expected: doctor prints cloud URL/repo/deep link info. Status is local-process only and may say not running before the
foreground agent is started.

### Connected-Agent Smoke

After logging in through the UI, create a connect token from the browser console:

```js
fetch("/api/agents/connect-token", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ name: "Local Agent", platform: navigator.platform }),
})
  .then((response) => response.json())
  .then(console.log);
```

Use the raw `token` field exactly as returned:

```bash
./bin/kode-stream agent start \
  --connect "<full-token-with-dot-signature>" \
  --cloud-url http://kode-stream.localhost:4318 \
  --repo .
```

Or use the full `deepLink` value exactly as returned:

```bash
./bin/kode-stream agent start \
  --connect "kodestream://connect?token=<full-token-with-dot-signature>" \
  --cloud-url http://kode-stream.localhost:4318 \
  --repo .
```

Do not manually trim or rewrite the signed token value.

Expected: the agent prints `Cloud Agent connected`, heartbeat acknowledgements follow, and the selected Git repo is
published as a `cloud_agent` workspace in Cloud.

Keycloak admin console:

```text
http://keycloak.localhost:8081
```

Admin console credentials are `admin` / `admin`.

## Test Users

| Username | Password | Email                | Kode Stream Role  |
|----------|----------|----------------------|-------------------|
| `admin`  | `admin`  | `admin@example.com`  | admin             |
| `editor` | `editor` | `editor@example.com` | viewer by default |
| `viewer` | `viewer` | `viewer@example.com` | viewer            |

`KODE_STREAM_ADMIN_USERS=admin@example.com` promotes the admin test user. Other users are viewers until Kode Stream has
role mapping beyond the admin allowlist.

## Stop Or Reset

Stop containers:

```bash
docker compose -f deploy/docker/local/cloud-mode/compose.yaml down
```

Reset local containers:

```bash
docker compose -f deploy/docker/local/cloud-mode/compose.yaml down -v
```

Use the reset command after changes to `deploy/docker/local/cloud-mode/keycloak/kode-stream-realm.json`; Keycloak imports the local realm
only when the development server starts.

## Notes

Use `kode-stream.localhost`, not `127.0.0.1`, for browser login. The Keycloak realm import uses that hostname in the
OAuth redirect URI so OAuth2Proxy, Keycloak, and the browser agree on the same local issuer and callback URLs.
