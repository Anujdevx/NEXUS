#!/usr/bin/env bash
# Stops everything start-all.sh started, in reverse order: frontend -> services -> databases -> gateway.
#   scripts/stop-all.sh          stop, keep data
#   scripts/stop-all.sh --clean  also delete volumes (databases, queues, uploaded media)
set -uo pipefail
. "$(cd "$(dirname "$0")" && pwd)/lib.sh"
CLEAN=0; [ "${1:-}" = "--clean" ] && CLEAN=1
V=""; [ "$CLEAN" = 1 ] && V="-v"
load_env

if [ -f "$RUN_DIR/frontend.pid" ]; then
  pid="$(cat "$RUN_DIR/frontend.pid")"
  if kill -0 "$pid" 2>/dev/null; then
    pkill -P "$pid" 2>/dev/null; kill "$pid" 2>/dev/null
    c_info "frontend stopped"
  fi
  rm -f "$RUN_DIR/frontend.pid" "$RUN_DIR/frontend.port"
fi

c_info "stopping services"
pids=()
for s in "${SERVICES[@]}"; do
  [ -f "$(compose_of "$s")" ] || continue
  ( docker compose -f "$(compose_of "$s")" down $V --remove-orphans >/dev/null 2>&1 ) &
  pids+=($!)
done
for p in "${pids[@]}"; do wait "$p"; done

c_info "stopping the gateway"
docker compose -f "$GATEWAY_COMPOSE" down $V --remove-orphans >/dev/null 2>&1
[ "$CLEAN" = 1 ] && rm -rf "$RUN_DIR"
c_ok "NEXUS stopped$([ "$CLEAN" = 1 ] && echo ' and cleaned')"
