#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd)"
COMPOSE_FILE="${KODE_STREAM_LOCAL_COMPOSE_FILE:-$ROOT_DIR/deploy/docker/local/local-mode/compose.yaml}"
STORAGE_OPTION="${KODE_STREAM_STORAGE_OPTION:-datadir}"
WORKSPACE_PATH="${KODE_STREAM_LOCAL_WORKSPACE:-$ROOT_DIR}"

if [[ "$STORAGE_OPTION" != "datadir" && "$STORAGE_OPTION" != "database" ]]; then
  printf 'KODE_STREAM_STORAGE_OPTION must be datadir or database in Local mode; got %s.\n' "$STORAGE_OPTION" >&2
  exit 2
fi

if [[ ! -d "$WORKSPACE_PATH" ]]; then
  printf 'KODE_STREAM_LOCAL_WORKSPACE must be an existing directory; got %s.\n' "$WORKSPACE_PATH" >&2
  exit 2
fi

export KODE_STREAM_STORAGE_OPTION="$STORAGE_OPTION"
export KODE_STREAM_LOCAL_WORKSPACE="$WORKSPACE_PATH"

printf '[kode-stream-local] Starting Local Docker stack with %s storage\n' "$STORAGE_OPTION"
printf '[kode-stream-local] Workspace mount: %s -> /workspace\n' "$WORKSPACE_PATH"
docker compose -f "$COMPOSE_FILE" up -d --build
printf '[kode-stream-local] Open http://127.0.0.1:%s\n' "${KODE_STREAM_LOCAL_PORT:-4317}"
