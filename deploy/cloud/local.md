# Local Cloud Auth Stack

This local stack mirrors the VM deployment shape:

- OAuth2Proxy is the browser entry point.
- Keycloak provides the login page.
- Kode Stream runs in Cloud mode on a private Docker network port.

## Run

```bash
docker compose -f deploy/cloud/local-compose.yaml up -d --build
```

Open:

```text
http://kode-stream.localhost:4318
```

Health check through OAuth2Proxy:

```bash
curl -fsS http://kode-stream.localhost:4318/api/health
```

Agent CLI surface:

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

Connected-agent smoke after logging in through the UI and creating a connect link:

```bash
./bin/kode-stream agent start --connect "kodestream://connect?token=<token_from_uri>" --cloud-url http://kode-stream.localhost:4318 --repo .
```

```bash
  ./bin/kode-stream agent start \
    --connect "kodestream://connect?token=<token_from_uri>" \
    --cloud-url http://127.0.0.1:4318 \
    --repo .

```

  fetch("/api/agents/connect-token", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name: "Local Agent", platform: navigator.platform })
  }).then(r => r.json()).then(console.log)

  Then use the raw token field, not a manually trimmed URI value:

  ./bin/kode-stream agent start \
    --connect "<full-token-with-dot-signature>" \
    --cloud-url http://kode-stream.localhost:4318 \
    --repo .

  Or use the full deepLink exactly as returned:

  ./bin/kode-stream agent start \
    --connect "kodestream://connect?token=<full-token-with-dot-signature>" \
    --cloud-url http://kode-stream.localhost:4318 \
    --repo .


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
docker compose -f deploy/cloud/local-compose.yaml down
```

Reset local containers:

```bash
docker compose -f deploy/cloud/local-compose.yaml down -v
```

Use the reset command after changes to `deploy/cloud/keycloak/kode-stream-realm.json`; Keycloak imports the local realm
only when the development server starts.

## Notes

Use `kode-stream.localhost`, not `127.0.0.1`, for browser login. The Keycloak realm import uses that hostname in the
OAuth redirect URI so OAuth2Proxy, Keycloak, and the browser agree on the same local issuer and callback URLs.
