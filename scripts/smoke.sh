#!/usr/bin/env bash
# Curl checks for every service through the gateway. Exits non-zero if anything fails.
#   scripts/smoke.sh          health, readiness and read endpoints
#   scripts/smoke.sh --e2e    also raises one SOS and checks the whole chain
set -uo pipefail
. "$(cd "$(dirname "$0")" && pwd)/lib.sh"
load_env
B="http://localhost:${GATEWAY_PORT}"
FAIL=0; E2E=0; [ "${1:-}" = "--e2e" ] && E2E=1
check() { # name, ok?
  if [ "$2" = 1 ]; then c_ok "$1"; else c_fail "$1"; FAIL=$((FAIL+1)); fi
}
code() { curl -s -o /dev/null -w '%{http_code}' --max-time 6 "$@"; }
tok() { curl -fsS --max-time 6 -X POST "$B/api/v1/auth/demo-token" -H 'Content-Type: application/json' -d "{\"role\":\"$1\"}" 2>/dev/null | sed -E 's/.*"token":"([^"]+)".*/\1/'; }

c_info "gateway $B"
for s in "${SERVICES[@]}"; do
  h="$(code "$B/svc/$s/healthz")"; r="$(code "$B/svc/$s/readyz")"
  [ "$h" = 200 ] && ok=1 || ok=0; check "$s /healthz $h" "$ok"
  [ "$r" = 200 ] && ok=1 || ok=0; check "$s /readyz  $r" "$ok"
done

CT="$(tok Controller)"; CZ="$(tok Citizen)"
[ -n "$CT" ] && ok=1 || ok=0; check "identity: demo token (Controller)" "$ok"
[ "$(code -H "Authorization: Bearer $CT" "$B/api/v1/auth/me")" = 200 ] && ok=1 || ok=0; check "identity: /auth/me" "$ok"
[ "$(code "$B/api/v1/hospitals")" = 401 ] && ok=1 || ok=0; check "auth: no token is refused (401)" "$ok"
get() { [ "$(code -H "Authorization: Bearer $CT" "$B$1")" = 200 ] && ok=1 || ok=0; check "GET $1" "$ok"; }
get /api/v1/graph/network
get /api/v1/graph/state
get /api/v1/topology/assets
get /api/v1/topology/blast-radius/sub-01
get /api/v1/hospitals
get /api/v1/units
get /api/v1/shelters
get "/api/v1/pharmacies?med=ors"
get /api/v1/dispatch/registry
get /api/v1/dispatch/logistics
get /api/v1/roads/closures
get /api/v1/environment/rain
get /api/v1/environment/rivers
get /api/v1/environment/flood-zones
get /api/v1/alerts
get /api/v1/incidents
get "/api/v1/events?limit=5"
post() { # path, body -> http code
  curl -s -o /dev/null -w '%{http_code}' --max-time 8 -X POST "$B$1" -H "Authorization: Bearer $CT" -H 'Content-Type: application/json' -d "$2"
}
[ "$(post /api/v1/routes/plan '{"from":"ct","to":"mus"}')" = 200 ] && ok=1 || ok=0; check "POST /routes/plan" "$ok"
[ "$(post /api/v1/hospitals/rank '{"at":{"lat":30.387,"lng":78.131},"need":"Trauma"}')" = 200 ] && ok=1 || ok=0; check "POST /hospitals/rank" "$ok"
[ "$(post /api/v1/hazard/rain-whatif '{"mm":100}')" = 200 ] && ok=1 || ok=0; check "POST /hazard/rain-whatif" "$ok"

if [ "$E2E" = 1 ]; then
  c_info "end to end: one SOS"
  sid="SOS-$(date +%s | tail -c 6)"
  resp="$(curl -s --max-time 8 -X POST "$B/api/v1/sos" -H "Authorization: Bearer $CZ" -H 'Content-Type: application/json' \
    -d "{\"id\":\"$sid\",\"place\":\"sdr\",\"placeName\":\"Sahastradhara\",\"hazard\":\"Trapped\",\"people\":4,\"injured\":\"Yes\",\"lat\":30.387,\"lng\":78.131,\"node\":\"sdr\",\"via\":\"direct\"}")"
  echo "$resp" | grep -q '"received"' && ok=1 || ok=0; check "POST /sos accepted ($sid)" "$ok"
  sleep 2
  ev="$(curl -s --max-time 6 -H "Authorization: Bearer $CT" "$B/api/v1/events?limit=60")"
  for k in sos.request assignment.dispatched hospital.capacity unit.status; do
    printf '%s' "$ev" | python3 -c "import sys,json; e=json.load(sys.stdin)['items']; sys.exit(0 if any(x['entity']=='${k%%.*}' and x['type']=='${k##*.}' for x in e) else 1)" && ok=1 || ok=0; check "event chain has $k" "$ok"
  done
  inc="$(curl -s --max-time 6 -H "Authorization: Bearer $CT" "$B/api/v1/incidents")"
  echo "$inc" | grep -q "\"$sid\"" && echo "$inc" | grep -q '"Unit assigned"' && ok=1 || ok=0; check "incident $sid is Unit assigned" "$ok"
fi

echo
if [ "$FAIL" = 0 ]; then c_ok "smoke test green"; else c_fail "$FAIL check(s) failed"; exit 1; fi
