#!/usr/bin/env bash
# Exports every frontend dataset to tools/seed/out/*.json (the single source of truth for all services),
# and with --reset also tells the running backend to reload its seed state (publishes scenario.loaded).
#   scripts/seed.sh            export only
#   scripts/seed.sh --reset    export, then reset the live system to the opening scenario
set -euo pipefail
. "$(cd "$(dirname "$0")" && pwd)/lib.sh"
command -v node >/dev/null || { c_fail "node is required (18+)"; exit 1; }
node "$ROOT/tools/seed/export-data.mjs"
if [ "${1:-}" = "--reset" ]; then
  load_env
  base="http://localhost:${GATEWAY_PORT}"
  tok="$(curl -fsS -X POST "$base/api/v1/auth/demo-token" -H 'Content-Type: application/json' -d '{"role":"Controller"}' | sed -E 's/.*"token":"([^"]+)".*/\1/')"
  curl -fsS -X POST "$base/api/v1/events" -H "Authorization: Bearer $tok" -H 'Content-Type: application/json' \
    -d '{"entity":"scenario","type":"loaded","status":"Reset","source":"seed.sh","origin":"seed-script","payload":{"id":"s260720"}}' >/dev/null
  c_ok "scenario.loaded published: every service reloads its seed state"
fi
