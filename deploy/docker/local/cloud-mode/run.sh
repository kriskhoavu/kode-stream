#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd)"
COMPOSE_FILE="${KODE_STREAM_CLOUD_COMPOSE_FILE:-$ROOT_DIR/deploy/docker/local/cloud-mode/compose.yaml}"
CLOUD_URL="${KODE_STREAM_CLOUD_URL:-http://kode-stream.localhost:4318}"
WORKSPACE_MODE="${KODE_STREAM_CLOUD_WORKSPACE_MODE:-agent_backed}"
REPO_PATH="${KODE_STREAM_AGENT_REPO:-$ROOT_DIR}"
AGENT_NAME="${KODE_STREAM_AGENT_NAME:-Local Agent}"
AGENT_PLATFORM="${KODE_STREAM_AGENT_PLATFORM:-$(uname -s | tr '[:upper:]' '[:lower:]')}"
ADMIN_SUBJECT="${KODE_STREAM_CLOUD_ADMIN_SUBJECT:-admin}"
ADMIN_EMAIL="${KODE_STREAM_CLOUD_ADMIN_EMAIL:-admin@example.com}"
COOKIE_SECRET="${KODE_STREAM_COOKIE_SECRET:-local-kode-stream-cookie-secret}"
ADMIN_USERS="${KODE_STREAM_ADMIN_USERS:-$ADMIN_EMAIL}"
BIN_PATH="${KODE_STREAM_BIN:-$ROOT_DIR/bin/kode-stream}"
STORAGE_OPTION="${KODE_STREAM_STORAGE_OPTION:-database}"

log() { printf '[kode-stream-cloud] %s\n' "$*"; }
die() {
  local code="$1"
  shift
  printf '%s\n' "$*" >&2
  exit "$code"
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

# Signs the token the way the server does: the user ID is the subject's hash, the cookie secret is the HMAC key.
generate_agent_token() {
  python3 - "$COOKIE_SECRET" "$ADMIN_SUBJECT" "$ADMIN_EMAIL" "$AGENT_NAME" "$AGENT_PLATFORM" <<'PY'
import base64, datetime, hashlib, hmac, json, secrets, sys

secret, subject, email, name, platform = sys.argv[1:6]

def raw_urlsafe(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).decode("ascii").rstrip("=")

def stable_id(value: str) -> str:
    return raw_urlsafe(hashlib.sha256(value.encode("utf-8")).digest())[:22]

user_id = stable_id(subject)
expires = datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(minutes=30)
token = {
    "id": raw_urlsafe(secrets.token_bytes(24)),
    "workspacePublicationId": raw_urlsafe(secrets.token_bytes(24)),
    "userId": user_id,
    "userEmail": email,
    "agentId": stable_id(f"{user_id}:{name}"),
    "name": name,
    "platform": platform,
    "expiresAt": expires.isoformat(timespec="microseconds").replace("+00:00", "Z"),
}
payload = raw_urlsafe(json.dumps(token, separators=(",", ":")).encode("utf-8"))
signature = raw_urlsafe(hmac.new(secret.encode("utf-8"), payload.encode("ascii"), hashlib.sha256).digest())
print(f"{payload}.{signature}")
PY
}

for cmd in docker curl go python3; do
  command -v "$cmd" >/dev/null 2>&1 || die 1 "Missing required command: $cmd"
done
[[ "$STORAGE_OPTION" == "database" ]] ||
  die 2 "Cloud mode requires KODE_STREAM_STORAGE_OPTION=database; got $STORAGE_OPTION."
[[ "$WORKSPACE_MODE" == "agent_backed" || "$WORKSPACE_MODE" == "agentless" ]] ||
  die 2 "KODE_STREAM_CLOUD_WORKSPACE_MODE must be agent_backed or agentless; got $WORKSPACE_MODE."

cd "$ROOT_DIR"
export KODE_STREAM_STORAGE_OPTION="database"
export KODE_STREAM_STORAGE_DRIVER="${KODE_STREAM_STORAGE_DRIVER:-postgres}"
export KODE_STREAM_PUBLIC_URL="$CLOUD_URL"
export KODE_STREAM_COOKIE_SECRET="$COOKIE_SECRET"
export KODE_STREAM_ADMIN_USERS="$ADMIN_USERS"

log "Starting the local Cloud stack ($KODE_STREAM_STORAGE_DRIVER, $WORKSPACE_MODE)"
docker compose -f "$COMPOSE_FILE" up -d --build
wait_for_health "$CLOUD_URL"

if [[ "$WORKSPACE_MODE" == "agentless" ]]; then
  log "Ready at $CLOUD_URL. No Cloud Agent starts in agentless mode."
  exit 0
fi

log "Building $BIN_PATH"
mkdir -p "$(dirname "$BIN_PATH")"
go build -o "$BIN_PATH" ./cmd/kode-stream
CONNECT_TOKEN="$(generate_agent_token)"

log "App: $CLOUD_URL  Keycloak: http://keycloak.localhost:8081  Repo: $REPO_PATH"
log "Ctrl-C stops the agent. The containers keep running until: docker compose -f $COMPOSE_FILE down"
exec "$BIN_PATH" agent start --connect "$CONNECT_TOKEN" --cloud-url "$CLOUD_URL" --repo "$REPO_PATH"
