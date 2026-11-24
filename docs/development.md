# Development

## Prerequisites

| Tool   | Version                     |
|--------|-----------------------------|
| Go     | 1.25.0 (pinned by `go.mod`) |
| Node   | 22 (as in CI)               |
| Docker | Only for the Compose stacks |

```bash
npm ci --no-audit --no-fund
```

## Verify

Run before every commit:

```bash
make verify   # npm run typecheck, npm test, go test ./..., go test -race ./..., npm run build
```

CI (`.github/workflows/release.yml`) runs the same checks except the race pass.

`go test ./...` skips suites it cannot reach. To test the Postgres driver and the Cloud server paths, point them at a
running Postgres:

```bash
KODE_STREAM_DATABASE_URL='postgres://kode_stream:kode_stream@127.0.0.1:5432/kode_stream?sslmode=disable' \
  go test ./internal/storage/... ./internal/server/...
```

## Run It

| Command              | Starts                                                                  |
|----------------------|-------------------------------------------------------------------------|
| `make up`            | The Local-mode container ([Local stack](../deploy/docker/local/local-mode/README.md)) |
| `make up STACK=cloud`| The stack that mirrors the VM deployment ([Cloud stack](../deploy/docker/local/cloud-mode/README.md)) |
| `make dev-up`        | The server from source, in the background (`./run.sh start`)            |
| `npm run dev`        | The Vite frontend dev server                                            |

`make stacks` lists the stacks. `make help` lists every target.

Plans live in `plans/platform/PM-<NNN>/`.
