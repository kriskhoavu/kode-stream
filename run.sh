#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT_DIR"

PORT="${KODE_STREAM_PORT:-4317}"
BIN="$ROOT_DIR/bin/kode-stream"
RUN_DIR="$ROOT_DIR/.run"
PID_FILE="$RUN_DIR/kode-stream.pid"
LOG_FILE="$RUN_DIR/kode-stream.log"

usage() {
  cat <<EOF
Usage: ./run.sh {start|stop|restart|status}
       ./run.sh smoke-storage

Environment:
  KODE_STREAM_PORT             Port to bind, default: 4317
  KODE_STREAM_STORAGE_OPTION   database or datadir, default: database
  KODE_STREAM_DATA_DIR         Optional app-state directory

Examples:
  KODE_STREAM_STORAGE_OPTION=database ./run.sh restart
  KODE_STREAM_STORAGE_OPTION=datadir ./run.sh restart
  ./run.sh smoke-storage

Logs:
  $LOG_FILE
EOF
}

effective_storage_option() {
  local option="${KODE_STREAM_STORAGE_OPTION:-database}"
  case "$option" in
    database|datadir)
      printf '%s\n' "$option"
      ;;
    *)
      echo "KODE_STREAM_STORAGE_OPTION must be database or datadir; got '$option'." >&2
      exit 2
      ;;
  esac
}

pid_value() {
  if [[ -f "$PID_FILE" ]]; then
    cat "$PID_FILE"
  fi
}

is_running() {
  local pid
  pid="$(pid_value)"
  [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null
}

process_command() {
  local pid="$1"
  ps -p "$pid" -o command= 2>/dev/null || true
}

is_kode_stream_process() {
  local pid="$1"
  local command
  command="$(process_command "$pid")"
  [[ "$command" == *"kode-stream serve"* ]]
}

build_app() {
  echo "Building frontend assets..."
  npm run build

  echo "Building Go binary..."
  mkdir -p "$ROOT_DIR/bin"
  go build -o "$BIN" ./cmd/kode-stream
}

port_owner() {
  if command -v lsof >/dev/null 2>&1; then
    lsof -nP -iTCP:"$PORT" -sTCP:LISTEN -t 2>/dev/null | head -n 1 || true
  fi
}

stop_pid() {
  local pid="$1"
  echo "Stopping Kode Stream PID $pid ..."
  kill "$pid"

  for _ in {1..30}; do
    if ! kill -0 "$pid" 2>/dev/null; then
      rm -f "$PID_FILE"
      echo "Stopped."
      return 0
    fi
    sleep 0.2
  done

  echo "Process did not exit after SIGTERM; sending SIGKILL."
  kill -9 "$pid" 2>/dev/null || true
  rm -f "$PID_FILE"
  echo "Stopped."
}

start_app() {
  mkdir -p "$RUN_DIR"
  local storage_option
  storage_option="$(effective_storage_option)"

  if is_running; then
    stop_pid "$(pid_value)"
  fi

  if [[ -f "$PID_FILE" ]]; then
    echo "Removing stale PID file."
    rm -f "$PID_FILE"
  fi

  local existing_owner
  existing_owner="$(port_owner)"
  if [[ -n "$existing_owner" ]] && is_kode_stream_process "$existing_owner"; then
    stop_pid "$existing_owner"
  fi

  build_app

  local owner
  owner="$(port_owner)"
  if [[ -n "$owner" ]]; then
    echo "Port $PORT is already in use by PID $owner; not starting."
    echo "Set KODE_STREAM_PORT to another port or stop that process."
    exit 1
  fi

  echo "Starting Kode Stream on http://127.0.0.1:$PORT ..."
  echo "Storage option: $storage_option"
  nohup "$BIN" serve -port "$PORT" >"$LOG_FILE" 2>&1 &
  echo "$!" >"$PID_FILE"

  sleep 1
  if ! is_running; then
    echo "Kode Stream failed to start. Recent logs:"
    tail -n 40 "$LOG_FILE" || true
    rm -f "$PID_FILE"
    exit 1
  fi

  echo "Started PID $(pid_value)."
  echo "Log: $LOG_FILE"
  echo "Open: http://127.0.0.1:$PORT"
}

smoke_storage() {
  local original_option="${KODE_STREAM_STORAGE_OPTION:-}"
  local original_port="${KODE_STREAM_PORT:-}"
  local previous_port="$PORT"
  local smoke_port="${KODE_STREAM_SMOKE_PORT:-4319}"
  export KODE_STREAM_PORT="$smoke_port"
  PORT="$smoke_port"
  for option in database datadir; do
    echo "Smoke starting local storage option: $option"
    export KODE_STREAM_STORAGE_OPTION="$option"
    stop_app
    start_app
    curl -fsS "http://127.0.0.1:$smoke_port/api/health" >/dev/null
    echo "Smoke health passed for $option."
  done
  stop_app
  if [[ -n "$original_option" ]]; then
    export KODE_STREAM_STORAGE_OPTION="$original_option"
  else
    unset KODE_STREAM_STORAGE_OPTION
  fi
  if [[ -n "$original_port" ]]; then
    export KODE_STREAM_PORT="$original_port"
    PORT="$original_port"
  else
    unset KODE_STREAM_PORT
    PORT="$previous_port"
  fi
}

stop_app() {
  if is_running; then
    stop_pid "$(pid_value)"
    return 0
  fi

  if [[ -f "$PID_FILE" ]]; then
    echo "Removing stale PID file."
    rm -f "$PID_FILE"
  fi

  local owner
  owner="$(port_owner)"
  if [[ -n "$owner" ]]; then
    if is_kode_stream_process "$owner"; then
      stop_pid "$owner"
      return 0
    fi
    echo "Port $PORT is in use by PID $owner, but it is not a Kode Stream process."
    return 0
  fi

  echo "Kode Stream is not running."
}

status_app() {
  if is_running; then
    echo "Kode Stream is running with PID $(pid_value)."
    echo "Open http://127.0.0.1:$PORT"
    echo "Log: $LOG_FILE"
    return 0
  fi

  local owner
  owner="$(port_owner)"
  if [[ -n "$owner" ]] && is_kode_stream_process "$owner"; then
    echo "Kode Stream is running with PID $owner."
    echo "Open http://127.0.0.1:$PORT"
    echo "Log: unmanaged process; no $LOG_FILE"
  else
    echo "Kode Stream is not running."
  fi
}

case "${1:-}" in
  start)
    start_app
    ;;
  stop)
    stop_app
    ;;
  restart)
    stop_app
    start_app
    ;;
  status)
    status_app
    ;;
  smoke-storage)
    smoke_storage
    ;;
  *)
    usage
    exit 2
    ;;
esac
