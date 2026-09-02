# Development

Build-and-test guide for contributors. This is neither durable reference nor an operator procedure, so it sits at
the top of `docs/` rather than in [`domain/`](domain/README.md) or [`deploy/`](../deploy/README.md).

## Prerequisites

| Tool   | Version                       |
|--------|-------------------------------|
| Go     | 1.25.0 (pinned by `go.mod`)   |
| Node   | 22                            |
| Docker | Only for the Compose stacks   |

```bash
npm ci --no-audit --no-fund
```

## Verify

Run these before every commit. They are the same checks CI runs in `.github/workflows/release.yml`.

```bash
npm run typecheck && npm test && npm run build && go test ./...
```

| Command           | Covers                                                     |
|-------------------|------------------------------------------------------------|
| `npm run typecheck` | `tsc --noEmit` over the React frontend                   |
| `npm test`          | Vitest suites under `web/`                               |
| `npm run build`     | Typecheck plus the Vite production bundle                |
| `go test ./...`     | Every backend package                                    |

## Suites that need backing services

`go test ./...` skips what it cannot reach. Point these at a running Postgres to exercise the database storage
driver and the Cloud-mode server paths:

```bash
KODE_STREAM_DATABASE_URL='postgres://kode_stream:kode_stream@127.0.0.1:5432/kode_stream?sslmode=disable' \
  go test ./internal/storage/... ./internal/server/...
```

## Run it

`./run.sh` starts the local server from source. To run a containerized topology instead, use the procedures under
[`deploy/`](../deploy/README.md) — [Local mode](../deploy/docker/local/local-mode/README.md) for the single-user
server, [Cloud mode](../deploy/docker/local/cloud-mode/README.md) for the stack that mirrors the VM deployment
shape.

## Where things go

`docs/` is durable reference and `deploy/` is operator procedures; [docs/README.md](README.md) and
[deploy/README.md](../deploy/README.md) each state their own rule. Plans live in `plans/platform/PM-<NNN>/`.
