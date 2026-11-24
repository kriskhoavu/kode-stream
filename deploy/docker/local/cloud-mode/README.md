# Local Cloud Stack

This stack mirrors the [Cloud deployment](../../../cloud-deployment.md) on one machine. OAuth2Proxy is the browser
entry, Keycloak serves the login page, and Postgres holds the state. Kode Stream runs in Cloud mode on a private
Docker network port.

## Run

| Mode                      | Command                                                          | Also starts                                     |
|---------------------------|------------------------------------------------------------------|-------------------------------------------------|
| Agent-Backed (default)    | `make up STACK=cloud`                                            | A foreground Cloud Agent for a local repository |
| Agentless Remote Snapshot | `KODE_STREAM_CLOUD_WORKSPACE_MODE=agentless make up STACK=cloud` | Nothing. It exits after the health check        |

`make up STACK=cloud` runs `run.sh`, which:

1. Starts the Compose stack and waits for `http://kode-stream.localhost:4318/api/health`.
2. In Agent-Backed mode, builds `./bin/kode-stream`, mints a 30-minute agent token with the development cookie
   secret, and starts the agent on the current repository.

- `KODE_STREAM_AGENT_REPO=/path/to/repo` and `KODE_STREAM_AGENT_NAME="MacBook Agent"` override the agent.
- `run.sh` refuses `KODE_STREAM_STORAGE_OPTION=datadir`.
- `Ctrl-C` stops the agent. The containers keep running.
- The `kode-stream-postgres` volume holds metadata. The `kode-stream-cloud-data` volume holds diagnostics.

No UI registers a Remote Snapshot workspace. Seed the provider and workspace state first. Then check that metadata,
tree and file reads are commit-pinned, and that write, Git, terminal, AI, runtime and verification actions are
unavailable.

## Open It

| URL                                 | What                                       |
|-------------------------------------|--------------------------------------------|
| `http://kode-stream.localhost:4318` | The app, through OAuth2Proxy               |
| `http://keycloak.localhost:8081`    | Keycloak admin console (`admin` / `admin`) |

Use `kode-stream.localhost`, not `127.0.0.1`. The OAuth2Proxy redirect URL uses that host name.

| Username | Password | Email                | Kode Stream role |
|----------|----------|----------------------|------------------|
| `admin`  | `admin`  | `admin@example.com`  | admin            |
| `editor` | `editor` | `editor@example.com` | viewer           |
| `viewer` | `viewer` | `viewer@example.com` | viewer           |

`KODE_STREAM_ADMIN_USERS=admin@example.com` makes `admin` an admin. Everyone else is a viewer.

## Connected-Agent Smoke

1. Sign in, then mint a connect token from the browser console:

   ```js
   fetch("/api/agents/connect-token", {
     method: "POST",
     headers: { "Content-Type": "application/json" },
     body: JSON.stringify({ name: "Local Agent", platform: navigator.platform }),
   }).then((r) => r.json()).then(console.log);
   ```

2. Start the agent with the returned `token` or `deepLink`, unchanged:

   ```bash
   ./bin/kode-stream agent start --connect "<token or deepLink>" --cloud-url http://kode-stream.localhost:4318 --repo .
   ```

3. Expect `Cloud Agent connected`, then heartbeat acknowledgements. The repository appears as a `cloud_agent`
   workspace.

`./bin/kode-stream agent doctor --cloud-url http://kode-stream.localhost:4318 --repo .` prints the URL, repository and
deep link. `agent status` reports only the local process.

## Stop Or Reset

```bash
make down STACK=cloud
make clean STACK=cloud FORCE=1   # also deletes the volumes
```

Keycloak imports `keycloak/kode-stream-realm.json` only on first start. Reset after you change it.
