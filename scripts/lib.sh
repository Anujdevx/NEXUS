#!/usr/bin/env bash
# Shared helpers for the NEXUS scripts. Source this file; do not run it.
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUN_DIR="$ROOT/.run"
SERVICES=(identity-access graph-engine iot-broker predictive-analytics traffic-control environmental-monitor emergency-dispatch citizen-reporting media-storage notification-gateway)
# service -> its database container (compose service name inside its own stack)
db_of() {
  case "$1" in
    graph-engine) echo neo4j ;;
    predictive-analytics|media-storage) echo "" ;;
    *) echo db ;;
  esac
}

c_info()  { printf '\033[1;36m[nexus]\033[0m %s\n' "$*"; }
c_ok()    { printf '\033[1;32m[ ok ]\033[0m %s\n' "$*"; }
c_warn()  { printf '\033[1;33m[warn]\033[0m %s\n' "$*"; }
c_fail()  { printf '\033[1;31m[fail]\033[0m %s\n' "$*" >&2; }

compose_of() { echo "$ROOT/services/$1/docker-compose.yml"; }
GATEWAY_COMPOSE="$ROOT/gateway/docker-compose.yml"

# compose reads the root .env (secrets, ports) for every stack, however the stack is started from these scripts
export COMPOSE_ENV_FILES="$ROOT/.env"

load_env() {
  if [ -f "$ROOT/.env" ]; then set -a; . "$ROOT/.env"; set +a; fi
  if [ -f "$RUN_DIR/ports.env" ]; then set -a; . "$RUN_DIR/ports.env"; set +a; fi
  GATEWAY_PORT="${GATEWAY_PORT:-8000}"
}

port_free() { ! lsof -iTCP:"$1" -sTCP:LISTEN >/dev/null 2>&1; }

# first free port among the arguments
pick_port() {
  for p in "$@"; do if port_free "$p"; then echo "$p"; return 0; fi; done
  return 1
}

# host port a running nexus container publishes for a container port, or empty
published_port() { docker port "$1" "$2" 2>/dev/null | head -1 | sed 's/.*://' || true; }

wait_http() { # url, seconds
  local url="$1" t="${2:-90}" i=0
  while [ "$i" -lt "$t" ]; do
    if curl -fsS -o /dev/null --max-time 2 "$url" 2>/dev/null; then return 0; fi
    sleep 1; i=$((i+1))
  done
  return 1
}

# wait until a container reports healthy (or, with no healthcheck, running)
wait_healthy() { # container, seconds
  local c="$1" t="${2:-120}" i=0 st
  while [ "$i" -lt "$t" ]; do
    st="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$c" 2>/dev/null || echo missing)"
    case "$st" in healthy|running) return 0 ;; esac
    sleep 1; i=$((i+1))
  done
  return 1
}
