#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="${KODE_STREAM_CLOUD_COMPOSE_FILE:-$ROOT_DIR/docker/cloud/local-compose.yaml}"
CLOUD_URL="${KODE_STREAM_CLOUD_URL:-http://kode-stream.localhost:4318}"
REPO_PATH="${KODE_STREAM_AGENT_REPO:-$ROOT_DIR}"
AGENT_NAME="${KODE_STREAM_AGENT_NAME:-Local Agent}"
AGENT_PLATFORM="${KODE_STREAM_AGENT_PLATFORM:-$(uname -s | tr '[:upper:]' '[:lower:]')}"
ADMIN_SUBJECT="${KODE_STREAM_CLOUD_ADMIN_SUBJECT:-admin}"
ADMIN_EMAIL="${KODE_STREAM_CLOUD_ADMIN_EMAIL:-admin@example.com}"
COOKIE_SECRET="${KODE_STREAM_COOKIE_SECRET:-local-kode-stream-cookie-secret}"
ADMIN_USERS="${KODE_STREAM_ADMIN_USERS:-$ADMIN_EMAIL}"
BIN_PATH="${KODE_STREAM_BIN:-$ROOT_DIR/bin/kode-stream}"
STORAGE_OPTION="${KODE_STREAM_STORAGE_OPTION:-database}"

log() {
  printf '[kode-stream-cloud] %s\n' "$*"
}

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'Missing required command: %s\n' "$1" >&2
    exit 1
  fi
}

wait_for_health() {
  local url="$1/api/health"
  local attempts="${KODE_STREAM_CLOUD_HEALTH_ATTEMPTS:-90}"
  local delay="${KODE_STREAM_CLOUD_HEALTH_DELAY_SECONDS:-2}"

  log "Waiting for Cloud health at $url"
  for ((i = 1; i <= attempts; i++)); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      log "Cloud health check passed"
      return 0
    fi
    sleep "$delay"
  done

  printf 'Cloud health check did not pass after %s attempts.\n' "$attempts" >&2
  docker compose -f "$COMPOSE_FILE" ps >&2 || true
  exit 1
}

generate_agent_token() {
  python3 - "$COOKIE_SECRET" "$ADMIN_SUBJECT" "$ADMIN_EMAIL" "$AGENT_NAME" "$AGENT_PLATFORM" <<'PY'
import base64
import datetime
import hashlib
import hmac
import json
import sys

secret, subject, email, name, platform = sys.argv[1:6]

def raw_urlsafe(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).decode("ascii").rstrip("=")

def stable_cloud_user_id(value: str) -> str:
    return raw_urlsafe(hashlib.sha256(value.encode("utf-8")).digest())[:22]

user_id = stable_cloud_user_id(subject)
token = {
    "userId": user_id,
    "userEmail": email,
    "agentId": stable_cloud_user_id(f"{user_id}:{name}"),
    "name": name,
    "platform": platform,
    "expiresAt": (
        datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(minutes=30)
    ).isoformat(timespec="microseconds").replace("+00:00", "Z"),
}
payload = raw_urlsafe(json.dumps(token, separators=(",", ":")).encode("utf-8"))
signature = raw_urlsafe(hmac.new(secret.encode("utf-8"), payload.encode("ascii"), hashlib.sha256).digest())
print(f"{payload}.{signature}")
PY
}

require_command docker
require_command curl
require_command go
require_command python3

if [[ "$STORAGE_OPTION" != "database" ]]; then
  printf 'Cloud mode requires KODE_STREAM_STORAGE_OPTION=database; got %s.\n' "$STORAGE_OPTION" >&2
  exit 2
fi

cd "$ROOT_DIR"
export KODE_STREAM_STORAGE_OPTION="database"
export KODE_STREAM_STORAGE_DRIVER="${KODE_STREAM_STORAGE_DRIVER:-postgres}"
export KODE_STREAM_PUBLIC_URL="$CLOUD_URL"
export KODE_STREAM_COOKIE_SECRET="$COOKIE_SECRET"
export KODE_STREAM_ADMIN_USERS="$ADMIN_USERS"

log "Starting local Cloud stack with Docker Compose"
log "Storage option: $KODE_STREAM_STORAGE_OPTION ($KODE_STREAM_STORAGE_DRIVER)"
docker compose -f "$COMPOSE_FILE" up -d --build

wait_for_health "$CLOUD_URL"

log "Building agent binary at $BIN_PATH"
mkdir -p "$(dirname "$BIN_PATH")"
go build -o "$BIN_PATH" ./cmd/kode-stream

CONNECT_TOKEN="$(generate_agent_token)"

log "Cloud app: $CLOUD_URL"
log "Keycloak: http://keycloak.localhost:8081"
log "Agent repo: $REPO_PATH"
log "Starting Cloud Agent in the foreground"
log "Press Ctrl-C to stop the agent. Docker services stay running; stop them with: docker compose -f $COMPOSE_FILE down"

exec "$BIN_PATH" agent start \
  --connect "$CONNECT_TOKEN" \
  --cloud-url "$CLOUD_URL" \
  --repo "$REPO_PATH"
