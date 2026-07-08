# Infrastructure Design: PM-001

## Goals

- Package Kode Stream as a local app.
- Keep the managed repositories clean.
- Support Homebrew distribution later.
- Provide repeatable local build and test commands.

## Local Runtime

| Component           | Decision                                    |
|---------------------|---------------------------------------------|
| Backend             | Go binary                                   |
| Frontend            | React/Vite static assets                    |
| Production serving  | Go binary embeds and serves frontend assets |
| Development serving | Vite dev server proxies API to Go backend   |
| Config location     | OS user data directory                      |
| Cache location      | OS user data directory                      |
| Managed repo writes | Not allowed in v1                           |

## Storage Layout

```text
{user-data}/kode-stream/
  repositories.yaml
  plan-index.yaml
  logs/
```

## Build Commands

| Command                       | Purpose                            |
|-------------------------------|------------------------------------|
| `go test ./...`               | Backend unit and integration tests |
| `npm run typecheck`           | Frontend TypeScript check          |
| `npm test`                    | Frontend unit tests                |
| `npm run build`               | Frontend production build          |
| `go build ./cmd/kode-stream` | Build local binary                 |
| `kode-stream serve`          | Start local app                    |

## Binary Packaging

- Embed frontend build output in the Go binary.
- Serve the app on localhost.
- Print the local URL when the server starts.
- Use a configurable port.
- Fail clearly if the selected port is unavailable.

## Native OS Integration

| Capability       | Purpose                                      |
|------------------|----------------------------------------------|
| Directory picker | Select repository paths and plan directories |
| Reveal path      | Open a registered path in the file manager   |

- macOS uses Finder-compatible commands.
- Windows uses Explorer-compatible commands.
- Linux uses available desktop tools when present.
- Native actions are convenience features and do not change repository contents.

## Homebrew Path

| Step                 | Output                      |
|----------------------|-----------------------------|
| Build release binary | `kode-stream`              |
| Archive binary       | tarball or zip              |
| Publish checksum     | SHA256                      |
| Formula install      | `brew install kode-stream` |
| Formula upgrade      | `brew upgrade kode-stream` |

## Security

- Do not store Git credentials.
- Reuse the user's existing Git installation.
- Do not transmit repository data outside localhost.
- Keep read-only mode enforced in backend handlers.
- Treat Markdown content as untrusted when rendering preview.
- Do not allow file reads outside configured plan directories.
- Do not follow symlinks that escape configured plan directories.
- Directory picker and reveal actions must not bypass indexed file access rules.
- Repository edit and delete actions write only Kode Stream app data.

## Performance Constraints

- Use cached metadata for board and list views.
- Avoid full Markdown reads during board rendering.
- Load Markdown content on demand in the workspace.
- Keep scan work off the UI request path when possible.
- Prefer incremental implementation, but keep the cache model ready for 10,000 plans and 100,000 files.

## Observability

- Log scan start, scan finish, plan count, warning count, and errors.
- Show last scan time in the UI.
- Show scan warnings in repository details.
- Keep logs local.

## Verification

- Build the embedded frontend.
- Build the Go binary.
- Start the binary and open the app through Playwright MCP.
- Confirm no files are changed under the registered repository after scan.
