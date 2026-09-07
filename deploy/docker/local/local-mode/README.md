# Local Docker Stack

Run Kode Stream in Local mode inside Docker. The browser connects to the container on `127.0.0.1:4317`; a host
directory is mounted at `/workspace` for registration in the app.

## Storage Options

| Option              | Command                                                                   | Active app state                              |
|---------------------|---------------------------------------------------------------------------|-----------------------------------------------|
| `datadir` (default) | `./deploy/docker/local/local-mode/run.sh`                                     | YAML/JSONL under the named Docker volume      |
| `database`          | `KODE_STREAM_STORAGE_OPTION=database ./deploy/docker/local/local-mode/run.sh` | SQLite database under the named Docker volume |

Both options keep app-owned state in the `kode-stream-local-data` Docker volume. Changing option does not migrate
runtime state; use the app’s explicit storage sync before switching when state must be preserved.

## Workspace Mount

By default the repository root is mounted at `/workspace`. To mount another directory:

```bash
KODE_STREAM_LOCAL_WORKSPACE=/absolute/path/to/repository make up
```

Register `/workspace` or a subdirectory in Kode Stream. The container must have write access to the mounted directory
for Markdown edits and Git operations.

## Capability Boundary

Local Docker mode is still Local mode, but local process integrations run in the container. `git` is installed in the
image; repository credentials are not, and must be made available to the container before any Git operation that
authenticates. Host terminal, host AI CLI, native file dialogs, and host path-reveal behavior are not automatically
available through the container boundary.

`/api/health` reports database status only, so a `datadir` container answers 503 there and never reports healthy even
while it serves normally. `make health` probes the app root for that reason.

## Stop Or Reset

```bash
make down
make clean FORCE=1
```

`make clean` removes the Local app-state volume; it does not delete the mounted workspace directory.
