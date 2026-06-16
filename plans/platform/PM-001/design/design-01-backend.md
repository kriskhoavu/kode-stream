# Backend Design: PM-001

## Goals

- Run a local HTTP API for Plan Manager.
- Register local Git repositories.
- Scan plan folders without writing to managed repositories.
- Serve board, workspace, file, and diff data.
- Cache scan results for fast startup and filtering.

## Scale Targets

| Target                        | Minimum         |
|-------------------------------|-----------------|
| Repositories                  | 100             |
| Plans                         | 10,000          |
| Files                         | 100,000         |
| Initial board load from cache | Under 2 seconds |
| Open plan detail from cache   | Under 500ms     |

## Runtime

| Area                | Decision                                           |
|---------------------|----------------------------------------------------|
| Language            | Go                                                 |
| Server              | Standard `net/http` or a small compatible router   |
| Git access          | Shell out to `git` through a narrow adapter        |
| Config storage      | OS user data directory                             |
| Cache storage       | Local SQLite database or equivalent embedded store |
| Managed repo writes | Not allowed in v1                                  |

## Data Model

### RepositoryConfig

| Field           | Type     | Purpose                                    |
|-----------------|----------|--------------------------------------------|
| id              | string   | Stable app-local repository id             |
| name            | string   | Display name                               |
| path            | string   | Local filesystem path                      |
| baselineBranch  | string   | Default branch for display and validation  |
| planDirectories | string[] | Plan roots such as `plans` or `docs/plans` |
| createdAt       | time     | Registration time                          |
| lastScannedAt   | time     | Last successful scan time                  |

### PlanSummary

| Field          | Type       | Purpose                                                            |
|----------------|------------|--------------------------------------------------------------------|
| id             | string     | Stable id built from repository, branch, service, ticket, and path |
| repositoryId   | string     | Owning repository                                                  |
| branch         | string     | Local branch used during scan                                      |
| service        | string     | Service or folder group                                            |
| ticket         | string     | Ticket id such as `DI-170` or `PM-001`                             |
| title          | string     | Plan title                                                         |
| status         | PlanStatus | Board status                                                       |
| owner          | string?    | Owner from metadata when present                                   |
| author         | string?    | Last Git author when known                                         |
| tags           | string[]   | Plan labels                                                        |
| updatedAt      | time?      | Last Git or file modification time                                 |
| description    | string?    | Short description extracted from README                            |
| metadataSource | string     | `plan.yaml` or `fallback`                                          |

### PlanDocument

| Field | Type    | Purpose                                                        |
|-------|---------|----------------------------------------------------------------|
| id    | string  | Document id                                                    |
| role  | string  | `overview`, `scenario`, `design`, `implementation`, or `other` |
| track | string? | `backend`, `frontend`, `infrastructure`, or `pipeline`         |
| path  | string  | Path relative to plan root                                     |
| label | string  | UI label                                                       |

## API Endpoints

| Method | Endpoint                         | Purpose                      |
|--------|----------------------------------|------------------------------|
| GET    | `/api/repositories`              | List registered repositories |
| POST   | `/api/repositories`              | Register a repository        |
| POST   | `/api/repositories/{id}/scan`    | Run a manual scan            |
| GET    | `/api/plans`                     | List filtered plan summaries |
| GET    | `/api/plans/{id}`                | Load plan detail             |
| GET    | `/api/plans/{id}/files`          | Load file tree               |
| GET    | `/api/plans/{id}/files/{fileId}` | Load file content            |
| GET    | `/api/plans/{id}/diff`           | Load read-only Git diff      |

## Scan Rules

- Validate the repository path before scanning.
- Scan local branches and the current working tree only.
- Do not run `git fetch` in v1.
- Do not create, switch, delete, or push branches.
- Read `plan.yaml` first.
- If `plan.yaml` is missing, infer:
  - service from the first folder under a plan root.
  - ticket from the plan folder name.
  - title from the first README heading.
  - status from implementation-plan status text when possible.
- Map missing or unknown status to `draft`.
- Store scan warnings and expose them in scan results.

## Cache Rules

- Manual Scan rebuilds derived metadata for one repository.
- Board and list APIs read from cached summaries.
- Plan detail APIs read cached metadata and load file content on demand.
- Cache stores plan summaries, document metadata, scan timestamps, warnings, and errors.
- Cache should not store full Markdown file content in PM-001.
- A failed plan parse creates a warning and does not stop the repository scan.

## Backend Boundaries

| Boundary             | Responsibility                             |
|----------------------|--------------------------------------------|
| `RepositoryRegistry` | Store and validate registered repositories |
| `GitAdapter`         | Run allowed read-only Git commands         |
| `PlanScanner`        | Discover plans and parse metadata          |
| `PlanIndex`          | Store and query cached scan results        |
| `FileAccess`         | Read files only through indexed plan paths |
| `PlanAPI`            | Convert backend data into HTTP responses   |

HTTP handlers must not read arbitrary filesystem paths directly.

## Git Adapter

| Operation           | Command Shape                               | Write Risk |
|---------------------|---------------------------------------------|------------|
| Validate repo       | `git rev-parse --show-toplevel`             | None       |
| Validate branch     | `git show-ref --verify refs/heads/{branch}` | None       |
| List local branches | `git for-each-ref refs/heads`               | None       |
| Last author         | `git log -1 --format=%an -- {path}`         | None       |
| Last update         | `git log -1 --format=%cI -- {path}`         | None       |
| Diff                | `git diff -- {path}`                        | None       |

## Error Handling

- Repository validation errors block registration.
- Scan warnings do not block other plans.
- File read errors show a workspace error state for that file.
- Git command failures are returned with a user-safe message.
- The backend logs command details locally for debugging.

## Security Rules

- File reads must stay inside configured plan directories.
- Path traversal must be rejected.
- Symlinks that escape configured plan directories must be rejected.
- PM-001 must not expose write APIs for registered repositories.
- PM-001 must not run Git commands that change refs, branches, remotes, index, or working tree.

## Verification

- Unit test metadata parsing and fallback parsing.
- Unit test status normalization.
- Integration test this repository as a fixture.
- Verify no backend test writes to managed plan folders.
