# Local Docker Stack

Run Kode Stream in Local mode inside Docker. The browser connects to the container on `127.0.0.1:4317`; a host
directory is mounted at `/workspace` for registration in the app.

## Storage Options

| Option              | Command                                                          | Active app state                              |
|---------------------|------------------------------------------------------------------|-----------------------------------------------|
| `datadir` (default) | `./docker/local-mode/run.sh`                                     | YAML/JSONL under the named Docker volume      |
| `database`          | `KODE_STREAM_STORAGE_OPTION=database ./docker/local-mode/run.sh` | SQLite database under the named Docker volume |

Both options keep app-owned state in the `kode-stream-local-data` Docker volume. Changing option does not migrate
runtime state; use the app’s explicit storage sync before switching when state must be preserved.

## Workspace Mount

By default the repository root is mounted at `/workspace`. To mount another directory:

```bash
KODE_STREAM_LOCAL_WORKSPACE=/absolute/path/to/repository ./docker/local-mode/run.sh
```

Register `/workspace` or a subdirectory in Kode Stream. The container must have write access to the mounted directory
for Markdown edits and Git operations.

## Capability Boundary

Local Docker mode is still Local mode, but local process integrations run in the container. Git must be available in the
image and repository credentials must be made available to the container before Git operations can authenticate. Host
terminal, host AI CLI, native file dialogs, and host path-reveal behavior are not automatically available through the
container boundary.

## Stop Or Reset

```bash
docker compose -f docker/local-mode/compose.yaml down
docker compose -f docker/local-mode/compose.yaml down -v
```

The second command removes the Local app-state volume; it does not delete the mounted workspace directory.
