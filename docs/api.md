# API reference

Everything is served through the gateway (`http://localhost:8000`, or the port `scripts/start-all.sh` printed). Errors are
`{ "error": { "code", "message" } }`; lists are `{ "items": [...] }`. Auth is a JWT (HS256) in `Authorization: Bearer`, with the
role claim `Controller | Responder | Citizen`; a Controller can do anything a Responder can. Service-to-service calls and the
simulator use `X-Service-Key`. Every service also serves `GET /healthz` and `GET /readyz`, and reaches them through the
gateway at `/svc/<service>/healthz` and `/svc/<service>/readyz`.

## Who can do what

| Role | Can |
|---|---|
| **Citizen** | `POST /sos`, `POST /reports`, `POST /media`, read public `GET /alerts`, open the WebSocket (own SOS and assignment, public alerts). Everything else is **403**: hospitals, units, shelters, routes, graph state, topology, hazard state, incidents, the audit trail |
| **Responder** | Everything a Citizen can, plus all operational reads (hospitals, units, shelters, pharmacies, routes, closures, environment, hazard, topology), `GET /incidents`, `PATCH /units/{id}`, `POST /roads/breaks`, and the live stream of road state and their unit's assignment |
| **Controller** (city administrator) | Everything: writes to hospitals, shelters, closures, alerts, flood stage, approvals, the cascade and reset |

Two sign-in modes. **Demo** (`DEMO_MODE=true`, the default): `POST /auth/demo-token` mints a token for any role, so the console switches
roles instantly. **Secure** (`scripts/start-all.sh --secure`, `DEMO_MODE=false`): demo tokens are refused, `GET /auth/config` says so, and the
console shows a sign-in screen; each role signs in with its own account and cannot change role without signing out.

The samples below can be pasted into a shell after:

```bash
B=http://localhost:8000
TOKEN=$(curl -s -X POST $B/api/v1/auth/demo-token -H 'Content-Type: application/json' -d '{"role":"Controller"}' | sed -E 's/.*"token":"([^"]+)".*/\1/')
```

## identity-access `/api/v1/auth`

| | |
|---|---|
| `POST /login` `{email,password}` | `{token, role, expiresAt}`. Rate limited to 10 a minute per IP. Demo users `controller@nexus.local`, `responder@nexus.local`, `citizen@nexus.local`; passwords come from `.env` (`DEMO_*_PASSWORD`) |
| `POST /demo-token` `{role}` | `{token, role, expiresAt}`. Only when `DEMO_MODE=true`. The console uses this so role switching stays instant |
| `GET /me` | `{id, email, role}` |

## graph-engine `/api/v1/graph`, `/routes`, `/topology`

```bash
curl -s -X POST $B/api/v1/routes/plan -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"from":"ct","to":{"lat":30.4516,"lng":78.0886},"closures":["c_kolhu"],"breaks":[{"lat":30.41,"lng":78.08}],"flood":0,"conf":0.9}'
```

| | |
|---|---|
| `GET /graph/network` | `{source, nodes, edges, label}`. `osm-file` (real OpenStreetMap roads from `frontend/public/roads.json`) or `builtin` (schematic) |
| `GET /graph/state` | the live road picture: `{closures, breaks, flood, conf}` |
| `GET /graph/edges/{id}` | one segment: name, midpoint, minutes |
| `POST /routes/plan` | `from`/`to`: a junction id (`"ct"`), `{lat,lng}` or `{nodeId}`. Optional `breaks` (edge ids or `{lat,lng}` points), `closures`, `flood`, `conf`, `hard`, `publish`. Omitted context fields use the live state. Returns `{path, nodes, edges, coords[[lng,lat]], latlng[[lat,lng]], km, minutes, min, steps[{edge,name,km,min,edges,slow,cut}], crosses, blocked, slow, graph}`. Same A* as the console's `roadnet.js`, so routes are identical |
| `POST /routes/reach` `{from, maxMinutes?}` | `{edges:[{id, minutes}]}`: roads by drive time (the "Show reach" layer) |
| `POST /routes/times` `{from, targets:[{id,lat,lng}], hard?}` | `{times:{id: minutes\|null}}`: one Dijkstra for many targets (hospital ranking) |
| `POST /routes/evaluate` | the Evaluation sweep (a port of `simulation.js`). Also at `POST /analytics/evaluation` via predictive-analytics |
| `GET /topology/assets` | utility, hospital and shelter assets and their dependencies. All utility data is **simulated** |
| `GET /topology/blast-radius/{id}` | dry run: `{root, failed, degraded, waves}` |
| `POST /topology/assets/{id}/fail` | Controller. Runs the cascade, publishes `asset.status` per change and one `cascade.computed` |
| `POST /topology/assets/{id}/restore`, `POST /topology/reset` | Controller. Heal one asset, or all |
| `POST /topology/upload` | Controller, multipart field `file`: GeoJSON Point features become assets (`properties.id`, `name`, `type`, `load_percentage`, `failure_threshold`, `depends_on:[{id,criticality}]`). Returns `{assets, edges}` |

## iot-broker `/api/v1/telemetry`, `/api/v1/events`, `/ws`

| | |
|---|---|
| `POST /events` envelope | 202. The console's mirror of `publish()`. Citizens may only publish `sos` and `telecom` |
| `GET /events?limit=200&key=sos` | newest-first envelopes from the server-side audit trail (`event_log`). Controller and Responder |
| `POST /telemetry/ingest` `{asset_id,type,value,timestamp,source?}` or `{items:[...]}` | `X-Service-Key`. Publishes `telemetry.raw` |
| `GET /telemetry/latest` | the most recent reading per asset |
| `WS /ws/v1/live?token=JWT&client=<id>[&unit=<id>]` | the live envelope stream, filtered by role (see events.md) |

## predictive-analytics `/api/v1/hazard`, `/api/v1/analytics`

| | |
|---|---|
| `POST /hazard/rain-whatif` `{mm, publish?}` | `{mm, segments:[{edge,name,kind,threshold_mm,passable,confidence}], closures:[id], risk}`. Inferred, not observed; thresholds (80 mm low-lying, 150 mm hill roads) are illustrative |
| `GET /hazard/state` | `{mm, source, inferred, risk}` |
| `POST /analytics/evaluation` `{runs, sosLoad}` | `{items:[{conn, policy, ttc:{m,ci}, unserved, quality, delivery}], params, graph, ms}`. Cached for ten minutes |

## traffic-control `/api/v1/roads`

| | |
|---|---|
| `GET /closures` | the documented closures with `active` |
| `POST /closures/{id}/toggle` | Controller. Publishes `road_segment.passability` |
| `POST /breaks` `{edgeId \| lat,lng, reason}` | Controller or Responder. `edgeId` is resolved to a point by graph-engine |
| `DELETE /breaks/{edgeId}` | Controller (repair) |
| `GET /breaks`, `GET /state` | `state` is `{closures:[active ids], breaks}` |

## environmental-monitor `/api/v1/environment`

`GET /rain`, `/rivers`, `/flood-zones`, `/landslides`, `/zoning` (each row keeps its source key; live readings carry `simulated`),
`GET /flood-stage`, `POST /flood-stage` `{stage}` (Controller; publishes `flood.stage`).

## emergency-dispatch `/api/v1/hospitals`, `/units`, `/shelters`, `/pharmacies`, `/dispatch`

```bash
curl -s -X POST $B/api/v1/hospitals/rank -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"at":{"lat":30.387,"lng":78.131},"need":"Trauma"}'
```

| | |
|---|---|
| `GET /hospitals` | registry plus simulated state `{cap, free, inbound, divert}` |
| `POST /hospitals/rank` `{at, need}` | `{items:[{h, min, reachable, capable, free, inbound, cost}], estimator}`: the console's `rankOnNet()` rows. `estimator` is `road`, or `haversine` if graph-engine is unreachable |
| `PATCH /hospitals/{id}` `{free?, inbound?, divert?}` | Controller |
| `GET /units`, `PATCH /units/{id}` `{status, task?, step?}` | PATCH: Controller or Responder |
| `GET /shelters`, `PATCH /shelters/{id}` `{occ?, open?}` | PATCH: Controller |
| `GET /pharmacies?med=ors&lat=&lng=` | `{items:[{p, qty, min, reachable}]}`, ranked like `rankPharmacies()` when lat/lng are given. `p.stock` has every medicine |
| `PATCH /pharmacies/{id}/stock` `{med, qty}` | Controller |
| `POST /dispatch/approve` `{incidentId?, unitId, hospitalId?, shelterId?, people?, from, to, route?}` | Controller |
| `GET /dispatch/assignments` | recent assignments |
| `GET /dispatch/registry`, `GET /dispatch/logistics` | tier-3 stubs (Abhayasūchī, Sambharaṇa): a stub envelope labelled `source:"stub"` |

## citizen-reporting `/api/v1/sos`, `/reports`, `/incidents`

| | |
|---|---|
| `POST /sos` `{place, placeName, hazard, people, injured, lat, lng, node?, via?, id?, client?}` | any role; 30 a minute per IP. 202 `{id, status:"received"}`. Dispatch then ranks hospitals and assigns an ambulance; the assignment arrives on the live stream |
| `GET /incidents`, `PATCH /incidents/{id}` `{status}` | Controller or Responder. `Open`, `Acknowledged`, `Closed` |
| `POST /reports` `{type, lat, lng, desc, media_id}`, `GET /reports` | hazard reports |

## media-storage `/api/v1/media`

`POST /media` (multipart `file`, images, audio, video or PDF, 10 MiB) returns `{id, url}`; `GET /media/{id}` streams it.
MinIO bucket `nexus-media`.

## notification-gateway `/api/v1/alerts`

`POST /alerts` `{area, severity, text_en, text_hi}` (Controller) returns a CAP-style alert (English and Hindi `info` blocks; defaults to the console's wording).
`GET /alerts`. Severity is `Extreme | Severe | Moderate | Minor`. Alerts are also raised automatically from `cascade.computed` and from severe `hazard.risk`.
