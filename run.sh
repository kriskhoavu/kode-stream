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
Usage: ./run.sh {start|stop|restart|status|smoke-storage}

  KODE_STREAM_PORT             default 4317
  KODE_STREAM_STORAGE_OPTION   database or datadir, default datadir
  KODE_STREAM_DATA_DIR         app-state directory

Log: $LOG_FILE
EOF
}

check_storage_option() {
  case "${KODE_STREAM_STORAGE_OPTION:-datadir}" in
    database|datadir) ;;
    *) echo "KODE_STREAM_STORAGE_OPTION must be database or datadir; got '$KODE_STREAM_STORAGE_OPTION'." >&2; exit 2 ;;
  esac
}

pid_value() {
  if [[ -f "$PID_FILE" ]]; then cat "$PID_FILE"; fi
}

is_running() {
  local pid
  pid="$(pid_value)"
  [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null
}

port_owner() {
  command -v lsof >/dev/null 2>&1 || return 0
  lsof -nP -iTCP:"$PORT" -sTCP:LISTEN -t 2>/dev/null | head -n 1 || true
}

is_kode_stream_process() {
  [[ "$(ps -p "$1" -o command= 2>/dev/null || true)" == *"kode-stream serve"* ]]
}

stop_pid() {
  local pid="$1"
  echo "Stopping Kode Stream PID $pid ..."
  kill "$pid"
  for _ in {1..30}; do
    kill -0 "$pid" 2>/dev/null || break
    sleep 0.2
  done
  if kill -0 "$pid" 2>/dev/null; then
    echo "Process did not exit after SIGTERM; sending SIGKILL."
    kill -9 "$pid" 2>/dev/null || true
  fi
  rm -f "$PID_FILE"
  echo "Stopped."
}

stop_app() {
  if is_running; then
    stop_pid "$(pid_value)"
    return 0
  fi
  rm -f "$PID_FILE"
  local owner
  owner="$(port_owner)"
  if [[ -z "$owner" ]]; then
    echo "Kode Stream is not running."
  elif is_kode_stream_process "$owner"; then
    stop_pid "$owner"
  else
    echo "Port $PORT is in use by PID $owner, but it is not a Kode Stream process."
  fi
}

start_app() {
  check_storage_option
  mkdir -p "$RUN_DIR" "$ROOT_DIR/bin"
  stop_app

  npm run build
  go build -o "$BIN" ./cmd/kode-stream

  local owner
  owner="$(port_owner)"
  if [[ -n "$owner" ]]; then
    echo "Port $PORT is in use by PID $owner. Set KODE_STREAM_PORT or stop that process." >&2
    exit 1
  fi

  echo "Starting Kode Stream on http://127.0.0.1:$PORT (storage: ${KODE_STREAM_STORAGE_OPTION:-datadir}) ..."
  nohup "$BIN" serve -port "$PORT" >"$LOG_FILE" 2>&1 &
  echo "$!" >"$PID_FILE"

  sleep 1
  if ! is_running; then
    echo "Kode Stream failed to start. Recent logs:" >&2
    tail -n 40 "$LOG_FILE" >&2 || true
    rm -f "$PID_FILE"
    exit 1
  fi
  echo "Started PID $(pid_value). Log: $LOG_FILE"
}

status_app() {
  local pid
  pid="$(pid_value)"
  if ! is_running; then
    pid="$(port_owner)"
    if [[ -z "$pid" ]] || ! is_kode_stream_process "$pid"; then
      echo "Kode Stream is not running."
      return 0
    fi
  fi
  echo "Kode Stream is running with PID $pid on http://127.0.0.1:$PORT. Log: $LOG_FILE"
}

smoke_storage() {
  PORT="${KODE_STREAM_SMOKE_PORT:-4319}"
  for option in database datadir; do
    export KODE_STREAM_STORAGE_OPTION="$option"
    start_app
    curl -fsS "http://127.0.0.1:$PORT/api/health" >/dev/null
    echo "Smoke health passed for $option."
  done
  stop_app
}

case "${1:-}" in
  start) start_app ;;
  stop) stop_app ;;
  restart) stop_app; start_app ;;
  status) status_app ;;
  smoke-storage) smoke_storage ;;
  *) usage; exit 2 ;;
esac
