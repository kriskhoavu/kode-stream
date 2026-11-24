# Local Docker Stack

Runs Kode Stream in Local mode inside Docker on `127.0.0.1:4317`. A host directory is mounted at `/workspace`.

| Storage             | Command                                       | App state                                             |
|---------------------|-----------------------------------------------|-------------------------------------------------------|
| `datadir` (default) | `make up`                                     | YAML and JSONL in the `kode-stream-local-data` volume |
| `database`          | `KODE_STREAM_STORAGE_OPTION=database make up` | SQLite in the same volume                             |

`make up` runs `run.sh`, which validates the storage option and the mount first. Switching the option does not move
state. To keep it, run the app's storage sync first. See the
[storage reference](../../../../wiki/platform/storage-reference.md).

## Workspace Mount

The repository root is mounted by default. To mount another directory:

```bash
KODE_STREAM_LOCAL_WORKSPACE=/absolute/path/to/repository make up
```

Register `/workspace` or a subdirectory in the app. The container needs write access to it for edits and Git.
`KODE_STREAM_LOCAL_PORT` changes the host port.

## Container Limits

- Git is in the image, but credentials are not. Provide them before any authenticated Git operation.
- The host terminal, host AI CLIs, native file dialogs and path reveal are not available.
- `/api/health` reports database status only. A `datadir` container answers 503 there and never reports healthy,
  although it serves normally. `make health` probes the app root instead.

## Stop Or Reset

```bash
make down
make clean FORCE=1   # deletes the app-state volume, not the mounted workspace
```
