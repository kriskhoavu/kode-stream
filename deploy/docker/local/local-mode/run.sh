#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd)"
COMPOSE_FILE="${KODE_STREAM_LOCAL_COMPOSE_FILE:-$ROOT_DIR/deploy/docker/local/local-mode/compose.yaml}"
export KODE_STREAM_STORAGE_OPTION="${KODE_STREAM_STORAGE_OPTION:-datadir}"
export KODE_STREAM_LOCAL_WORKSPACE="${KODE_STREAM_LOCAL_WORKSPACE:-$ROOT_DIR}"

if [[ "$KODE_STREAM_STORAGE_OPTION" != "datadir" && "$KODE_STREAM_STORAGE_OPTION" != "database" ]]; then
  printf 'KODE_STREAM_STORAGE_OPTION must be datadir or database in Local mode; got %s.\n' "$KODE_STREAM_STORAGE_OPTION" >&2
  exit 2
fi
if [[ ! -d "$KODE_STREAM_LOCAL_WORKSPACE" ]]; then
  printf 'KODE_STREAM_LOCAL_WORKSPACE must be an existing directory; got %s.\n' "$KODE_STREAM_LOCAL_WORKSPACE" >&2
  exit 2
fi

printf '[kode-stream-local] Starting Local Docker stack with %s storage\n' "$KODE_STREAM_STORAGE_OPTION"
printf '[kode-stream-local] Workspace mount: %s -> /workspace\n' "$KODE_STREAM_LOCAL_WORKSPACE"
docker compose -f "$COMPOSE_FILE" up -d --build
printf '[kode-stream-local] Open http://127.0.0.1:%s\n' "${KODE_STREAM_LOCAL_PORT:-4317}"
