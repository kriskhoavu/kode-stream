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
