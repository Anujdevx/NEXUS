# NEXUS: Master Build Prompt

> Save this file as `CLAUDE.md` in the root of the `nexus/` repo. Claude Code reads it automatically at the start of every session, and every instruction here applies to all work in this repo.

---

## 0. Your role

You are the lead engineer finishing **NEXUS**, a B.Tech major project, before a demo **tomorrow**. The repo already has the folder structure and boilerplate for 10 Go microservices, a Traefik gateway, a Python simulator and orchestration scripts. Your job, in order:

1. Move the partner's finished frontend into `frontend/` with no visual or behavioural changes.
2. Implement the Go backend so the frontend runs **live** against it, with the frontend's local engine as an automatic fallback.
3. Make `./scripts/start-all.sh` bring the full system up on a MacBook and get the demo path working end to end.

Work in the phases in §9. Commit at the end of every phase. After each phase, print a short status: what works, what doesn't, and what's next.

---

## 1. What NEXUS is (ground truth)

NEXUS is a **coordination layer for multi-agency disaster response**. The pilot district is **Dehradun, Uttarakhand**. It works as a digital twin of the district's response infrastructure: the road network, hospitals, pharmacies, shelters, response units and lifeline utilities. Every agency publishes into one event bus and reads from it. When a road breaks, a flood rises or a utility fails, the effect cascades through the graph. NEXUS reroutes ambulances, re-ranks hospitals and alerts the public.

The control-room console is called **Sūtradhāra** ("holder of the thread"). The event bus is called **Sūtra**.

### Users and roles (already in the frontend)

| Role | Sees | Purpose |
|---|---|---|
| Controller | Every screen | Operating picture, dispatch, approvals, alerts, audit |
| Responder | `assignment, map, responders, sos, settings` | Their assigned mission, route and destination |
| Citizen | `sos` only | One-button SOS; works offline and delivers later |

### The 10 domain modules (from `frontend/src/data/index.js` → `MODULES`)

| Module | Meaning | Domain | Tier |
|---|---|---|---|
| Ārogya | Medical and hospital | Beds, ICU, specialty, ambulance fleet | 1 |
| Mārga | Transport and roads | Passability, bridges, routing engine | 1 |
| Āhvāna | Citizen SOS | SOS, hazard reports, check-ins | 1 |
| Sūtradhāra | Command and coordination | Dashboard, public alerts, audit trail | 1 |
| Rakṣaka | Rescue forces | SDRF, fire, police, boat crews | 2 |
| Āśraya | Shelter and camps | Capacity, live occupancy | 2 |
| Pūrvasūchanā | Early warning | Rainfall, river gauges, seismic, alerts | 2 |
| Prāṇadhārā | Utilities and lifelines | Power, water, telecom status | 3 (stub) |
| Abhayasūchī | Vulnerable registry | Priority triage (consent-gated) | 3 (stub) |
| Sambharaṇa | Logistics and supply | Relief stock, warehouses, fuel | 3 (stub) |

### Data honesty rule

The frontend labels every dataset as **Real** (with a named source in `SRC`) or **Simulated**. The backend must keep this distinction. Every seeded record and every envelope carries `source`. Simulated values must never be presented as real.

---

## 2. Decisions that override the earlier (Gemini) plan

The earlier planning document described a Next.js + TypeScript + Tailwind v4 + React Flow frontend for a generic "smart city". **The frontend the team actually built is different, and it is final.** Where the old plan and this file disagree, this file wins.

| Topic | Old plan | **Final decision** |
|---|---|---|
| Frontend framework | Next.js App Router, TS, Tailwind v4, Framer Motion, React Flow, SWR | **Vite 5 + React 18 + JavaScript + plain CSS + MapLibre GL + lucide-react**, exactly as the partner built it |
| Routing | `/builder`, `/grid-twin`, … | Hash routes `#/overview`, `#/routes`, … (19 screens) |
| State | SWR + WebSockets | One store, `src/state/store.js` (`useSyncExternalStore`), plus a new API/WS adapter |
| Domain | Generic smart-city grid twin | Disaster response twin for Dehradun; utilities cascade is the Prāṇadhārā module |
| Backend | 10 Go services, RabbitMQ, Neo4j, Postgres, Redis, Traefik | **Unchanged.** Service responsibilities are remapped to the modules in §5 |
| Isolated compose per service | Yes | **Kept.** Shared infra (Traefik, RabbitMQ) lives in `gateway/`, and all stacks join one external network |
| Object storage | AWS S3 | **MinIO** locally (S3-compatible, same `AWS_*` env vars) |
| Neo4j GDS | Implied | **Not required.** Routing and cascade run in Go over an in-memory graph loaded from Neo4j |

---

## 3. Non-negotiable rules

1. **Do not redesign the frontend.** Don't change its look, layout, copy, fonts, CSS tokens, routes, keyboard shortcuts or behaviour. The only frontend changes allowed are those in §8: the API adapter, env config, the Vite proxy, a live/offline indicator, and the README.
2. **The frontend must always run without the backend.** If the API is unreachable, the existing local engines (`roadnet.js`, `hazard.js`, `simulation.js`, store ranking) take over silently. The demo can never depend on the backend being up.
3. **Keep return shapes.** Backend responses for routing and ranking must match what `store.js` already consumes, so a swap needs no view changes.
4. **One envelope everywhere.** Every event on RabbitMQ and WebSocket uses the schema in §6.
5. **No secrets in git.** Each service gets a `.env.example`. Real `.env` files are gitignored.
6. **Don't delete user work without git history.** Remove the Next.js scaffold with `git rm`, after committing it, so it stays recoverable.
7. **Every Go service exposes** `GET /healthz` (liveness) and `GET /readyz` (dependencies OK). They use structured JSON logs (`log/slog`) and shut down gracefully on SIGTERM.
8. **Go version** is 1.22+. Prefer the standard library: `net/http` with the 1.22 pattern router. Approved dependencies: `github.com/rabbitmq/amqp091-go`, `github.com/jackc/pgx/v5`, `github.com/neo4j/neo4j-go-driver/v5`, `github.com/redis/go-redis/v9`, `github.com/golang-jwt/jwt/v5`, `github.com/coder/websocket`, `github.com/google/uuid`, `github.com/minio/minio-go/v7`. Ask before adding anything else.

---

## 4. Target repository layout

```text
nexus/
├── CLAUDE.md                      # this file
├── README.md                      # how to run, demo script, architecture summary
├── docs/
│   ├── architecture.md            # bus, services, data flow, diagrams (mermaid)
│   ├── api.md                     # every endpoint with request/response examples
│   └── events.md                  # envelope schema + every entity.type routing key
├── frontend/                      # the partner's Vite app (moved in, Phase 1)
│   ├── src/ (…unchanged…)
│   ├── src/api/                   # NEW: client.js, live.js (WS), config.js
│   ├── public/  scripts/  tests/
│   ├── index.html  vite.config.js  package.json  playwright.config.js
│   └── .env.example               # VITE_API_URL, VITE_WS_URL, VITE_LIVE
├── gateway/
│   ├── docker-compose.yml         # Traefik + RabbitMQ + MinIO, creates network nexus-net
│   └── traefik.yml
├── services/
│   ├── shared/                    # NEW Go module: envelope, bus, httpx, auth, config, log
│   ├── identity-access/  graph-engine/  iot-broker/  predictive-analytics/
│   ├── traffic-control/  environmental-monitor/  emergency-dispatch/
│   ├── citizen-reporting/  media-storage/  notification-gateway/
│   │   └── (each) cmd/main.go, internal/{http,store,bus,domain}/, go.mod,
│   │       Dockerfile, docker-compose.yml, .env.example, migrations/ (if Postgres)
│   └── go.work                    # local dev workspace across all modules + shared
├── tools/
│   ├── seed/                      # NEW: export-data.mjs → *.json from frontend/src/data
│   └── simulator/                 # main.py, requirements.txt (Phase 7)
└── scripts/
    ├── start-all.sh  stop-all.sh
    ├── smoke.sh                   # NEW: curl checks for every service via the gateway
    └── seed.sh                    # NEW: run exporters + load seeds into services
```

**Shared Go module.** Put the shared code in `services/shared` (module path `nexus/shared`). Each service's `go.mod` gets `replace nexus/shared => ../shared`. Set every service's Docker build context to `services/` with `dockerfile: <service>/Dockerfile`, so the shared module is inside the context. Update the existing compose files and Dockerfiles to match. Use multi-stage builds: `golang:1.22-alpine` → `gcr.io/distroless/static` (or `alpine`), with `CGO_ENABLED=0`.

---

## 5. Service map: the 10 Go services ↔ 10 modules

All services listen on container port `8080`. Traefik routes by path prefix. Every route below is served under `http://localhost:8000` through the gateway.

| Service | Owns (modules) | Store | Path prefixes | Consumes (routing keys) | Publishes |
|---|---|---|---|---|---|
| **identity-access** | Roles, auth | Postgres `identity` | `/api/v1/auth` | none | `user.*` |
| **graph-engine** | Mārga routing; infrastructure graph; Prāṇadhārā cascade | Neo4j | `/api/v1/graph`, `/api/v1/routes`, `/api/v1/topology` | `road_segment.*`, `flood.*`, `asset.*` | `route.computed`, `asset.status`, `cascade.computed` |
| **iot-broker** | Sūtra bus edge: ingest + WebSocket fan-out + event log | Postgres `events`, Redis | `/api/v1/telemetry`, `/api/v1/events`, `/ws` | `#` (everything, for fan-out + log) | `telemetry.raw` |
| **predictive-analytics** | Pūrvasūchanā hazard inference; Evaluation sweep | Redis | `/api/v1/hazard`, `/api/v1/analytics` | `rainfall.*`, `river.*`, `telemetry.raw` | `road_segment.passability` (inferred, with confidence), `hazard.risk` |
| **traffic-control** | Mārga closures, breaks, repairs | Postgres `traffic` | `/api/v1/roads` | `scenario.loaded` | `road_segment.passability` |
| **environmental-monitor** | Rainfall, rivers, flood zones, landslides, flood stage | Postgres `environment` | `/api/v1/environment` | `telemetry.raw` (env metrics) | `rainfall.observed`, `river.level`, `flood.stage` |
| **emergency-dispatch** | Ārogya hospitals + allocation; Rakṣaka units; Āśraya shelters; pharmacies; dispatch orchestrator | Postgres `dispatch`, Redis | `/api/v1/dispatch`, `/api/v1/hospitals`, `/api/v1/units`, `/api/v1/shelters`, `/api/v1/pharmacies` | `sos.request`, `incident.*`, `road_segment.*` | `assignment.dispatched`, `hospital.capacity`, `unit.status`, `unit.progress`, `shelter.*`, `pharmacy.stock` |
| **citizen-reporting** | Āhvāna SOS, hazard reports, incidents | Postgres `reports` | `/api/v1/sos`, `/api/v1/reports`, `/api/v1/incidents` | `assignment.dispatched` | `sos.request`, `incident.*`, `hazard_report.created` |
| **media-storage** | Attachments for reports | MinIO bucket `nexus-media` | `/api/v1/media` | none | `media.uploaded` |
| **notification-gateway** | Sūtradhāra public alerts (CAP-style, English + Hindi), SMS fallback stub | Postgres `alerts` | `/api/v1/alerts` | `cascade.computed`, `hazard.risk`, `assignment.dispatched` | `public_alert.cap_alert`, `telecom.connectivity` |

**Stubs (tier 3).** Prāṇadhārā lives in graph-engine as `Asset` nodes of type POWER, WATER and TELECOM. Abhayasūchī and Sambharaṇa get a `GET` endpoint each that returns the stub envelope already defined in `MODULES[].stub`: `/api/v1/dispatch/registry` and `/api/v1/dispatch/logistics`.

### Key endpoints (minimum contract)

Return JSON. Errors use `{ "error": { "code", "message" } }`. Lists use `{ "items": [...] }`.

```
identity-access
  POST /api/v1/auth/login            {email,password} → {token, role, expiresAt}
  POST /api/v1/auth/demo-token       {role}           → {token, role}   (only when DEMO_MODE=true)
  GET  /api/v1/auth/me                                → {id,email,role}

graph-engine
  GET  /api/v1/graph/network                          → {source, nodes, edges, label}   (same as frontend netInfo)
  POST /api/v1/routes/plan           {from:{lat,lng}|nodeId, to:{lat,lng}|nodeId, breaks:[edgeId], closures:[closureId]}
                                     → {path:[nodeId], coords:[[lng,lat]], km, minutes, steps:[{edge,name,km,min}]}
  POST /api/v1/routes/reach          {from, breaks}   → {edges:[{id, minutes}]}            (drives "Show reach")
  POST /api/v1/topology/upload       multipart GeoJSON → {assets, edges}                   (Controller only)
  GET  /api/v1/topology/blast-radius/{assetId}        → {failed:[...], degraded:[...], waves:[[...]]}
  POST /api/v1/topology/assets/{id}/fail              → triggers cascade, publishes events

iot-broker
  POST /api/v1/telemetry/ingest      {asset_id,type,value,timestamp}   (X-Service-Key)
  POST /api/v1/events                envelope          → 202   (frontend mirror of publish())
  GET  /api/v1/events?limit=200                        → recent envelopes (audit trail)
  WS   /ws/v1/live?token=JWT&client=<id>               → stream of envelopes

predictive-analytics
  POST /api/v1/hazard/rain-whatif    {mm}             → {segments:[{edge, passable, confidence}]}
  POST /api/v1/analytics/evaluation  {runs, sosLoad}  → same shape as frontend simulation.js output

traffic-control
  GET  /api/v1/roads/closures        → {items}         POST /api/v1/roads/closures/{id}/toggle
  POST /api/v1/roads/breaks          {edgeId, reason}  DELETE /api/v1/roads/breaks/{edgeId}

environmental-monitor
  GET  /api/v1/environment/{rain|rivers|flood-zones|landslides|zoning}
  POST /api/v1/environment/flood-stage {stage}

emergency-dispatch
  GET  /api/v1/hospitals             POST /api/v1/hospitals/rank {at:{lat,lng}, need} → same shape as rankOnNet()
  PATCH /api/v1/hospitals/{id}       {free?, divert?}
  GET  /api/v1/units                 PATCH /api/v1/units/{id} {status, task}
  GET  /api/v1/shelters              PATCH /api/v1/shelters/{id} {occ?, open?}
  GET  /api/v1/pharmacies?med=ors&lat=&lng=   PATCH /api/v1/pharmacies/{id}/stock {med, qty}
  POST /api/v1/dispatch/approve      {incidentId, unitId, hospitalId, route}

citizen-reporting
  POST /api/v1/sos                   {place, hazard, people, injured, lat, lng, via} → {id, status}
  GET  /api/v1/incidents             PATCH /api/v1/incidents/{id} {status}
  POST /api/v1/reports               {type, lat, lng, desc, media_id}

media-storage
  POST /api/v1/media                 multipart → {id, url}     GET /api/v1/media/{id}

notification-gateway
  POST /api/v1/alerts                {area, severity, text_en, text_hi} → CAP-style alert
  GET  /api/v1/alerts
```

Auth: JWT HS256 (`JWT_SECRET`, shared by all services through `shared/auth`), with the role claim `Controller | Responder | Citizen`. Writes need the right role. `POST /api/v1/sos` accepts any token. Telemetry ingest uses `X-Service-Key`. Rate-limit `/sos` and `/auth/login` per IP with Redis.

---

## 6. The Sūtra envelope (one schema, everywhere)

The frontend already builds this in `publish()` in `store.js`. The backend adopts it as-is and adds the fields marked NEW.

```json
{
  "id": "uuid",                       // NEW
  "entity": "sos",                    // sos | assignment | incident | road_segment | route | hospital | unit | shelter | pharmacy | flood | rainfall | river | public_alert | telecom | asset | cascade | scenario | race | telemetry
  "type": "request",                  // see docs/events.md for every entity.type pair
  "geo": [30.3245, 78.0418],          // [lat, lng] or null
  "status": "dispatched",
  "capacity": { "free": 12 },         // or null
  "confidence": 0.9,                  // 0..1
  "timestamp": "2026-10-05T09:30:00Z",
  "source": "sutradhara",             // producer name; "simulated" or a SRC key where applicable
  "origin": "web-7f3a",               // NEW: client/service id, used to drop echoes
  "payload": { }                      // NEW: full domain object (e.g. the incident or the route)
}
```

- RabbitMQ: one durable **topic** exchange `nexus.events`. Routing key = `<entity>.<type>` (e.g. `sos.request`, `road_segment.passability`). Each service has its own durable queue (`q.<service>`) bound to the keys it consumes in §5. Use manual ack, `prefetch=32` and a dead-letter exchange `nexus.dlx`.
- Put this in `services/shared/bus` with: `Publish(ctx, env)`, `Subscribe(ctx, queue, keys []string, handler)`, reconnect with backoff, and publisher confirms.
- The iot-broker WebSocket hub sends every envelope to connected clients, filtered by role: Citizens get only `public_alert.*`, their own `sos.*` and `assignment.*` envelopes. Responders get their unit's `assignment` and `unit` envelopes plus `road_segment` envelopes. Controllers get everything. It drops the envelope back to the client whose `origin` matches.
- iot-broker writes every envelope to Postgres `event_log`. This is the server-side audit trail.

---

## 7. Data models

### 7.1 Single source of truth for seed data

`frontend/src/data/index.js` already holds every dataset with its source keys: `HOSPITALS, PHARMACIES, MEDICINES, SHELTERS, UNITS, NODES, EDGES, CLOSURES, SCENARIOS, FLOOD, SLIDES, INFRA, RAIN, ZONING, RIVERWATCH, DISTRICTS, MODULES, PLACES, RIVERS, SRC, SPEED, WIND, CAPS, SIMCAP`.

Write `tools/seed/export-data.mjs`. It should import that module with Node 18+ ESM and write one JSON file per dataset to `tools/seed/out/`. Each service seeds its store from these JSON files on first boot when its tables are empty. `scripts/seed.sh` runs the exporter and copies the outputs into each service's seed volume. **Never hand-copy data into Go.** If `public/roads.json` (real OpenStreetMap roads, produced by `npm run roads`) exists, graph-engine loads it in preference to the schematic `NODES`/`EDGES`. This is the same order the frontend uses, described in its README.

Port the simulated seeding logic exactly so the backend and the offline frontend show the same numbers: `seedHosp()` (free beds from `SIMCAP`) and `seedStock()` (deterministic sine hash) from `store.js`.

### 7.2 Neo4j (graph-engine)

```
(:Junction {id, name, lat, lng})
(:Junction)-[:ROAD {id, name, class: 'h'|'a'|'m', km, speedKmh, closureId?, status: 'OPEN'|'SLOW'|'CLOSED', confidence}]->(:Junction)
    // create both directions; all roads are treated as two-way (frontend assumption)
(:Asset {id, name, type: 'POWER'|'WATER'|'TELECOM'|'HOSPITAL'|'SHELTER'|'PUMP'|'BRIDGE',
         lat, lng, status: 'ACTIVE'|'DEGRADED'|'FAILED', load_percentage, failure_threshold, source})
(:Asset)-[:NEAR]->(:Junction)
(:Asset)-[:DEPENDS_ON {criticality: 0..1, grace_period_ms}]->(:Asset)    // dependent → provider
(:Asset)-[:LOCATED_IN]->(:Region {id, name, boundary})
```

Create constraints `Junction.id` UNIQUE and `Asset.id` UNIQUE. Seed hospitals and shelters as Assets near their `node`. For the Prāṇadhārā cascade demo, seed a small set of simulated utility assets (`source: "simulated"`): 3 substations, 4 water pumps and 3 telecom towers, with dependencies such as `hospital DEPENDS_ON substation`, `pump DEPENDS_ON substation` and `telecom DEPENDS_ON substation`.

**Cascade algorithm (fixes a bug in the old plan).** The old Cypher used `r.criticality` on a variable-length path, where `r` is a list, and its arrow direction was inverted. Implement the cascade in Go instead:

1. An asset fails when `load_percentage > failure_threshold` for longer than `grace_period_ms`, or when someone calls the fail endpoint directly.
2. BFS outwards over the reverse of `DEPENDS_ON`: `MATCH (d:Asset)-[r:DEPENDS_ON]->(f:Asset {id:$id}) RETURN d, r`.
3. For each dependent: `load += 100 * r.criticality`. If `load > threshold`, the dependent becomes FAILED and joins the queue. Otherwise it becomes DEGRADED.
4. Record each BFS level as a wave. Persist the statuses and publish `asset.status` for each changed asset, then one `cascade.computed` with `{root, waves, failed, degraded}`.
5. A HOSPITAL that goes FAILED or DEGRADED also publishes `hospital.capacity` with `divert: true`. emergency-dispatch then excludes it from ranking, and the frontend reroutes.

**Routing.** On startup and on every `road_segment.*` event, load the road graph into memory and run A* (haversine heuristic) on travel time. Port `frontend/src/engine/roadnet.js` exactly (`shortest`, `nearestNode`, `nearestEdge`, `distFrom`, break handling, `TRUE_SLOW`, `SPEED`, `WIND`) so backend and offline routes are identical. Write a table test that compares 10 fixed A→B pairs against the JS output.

### 7.3 Postgres (one database per service, migrations in `migrations/*.sql`, run on boot)

- `identity`: `users(id uuid pk, email unique, password_hash, role, created_at)`. Seed three demo users from env: `controller@nexus.local`, `responder@nexus.local`, `citizen@nexus.local`.
- `reports`: `incidents(id text pk, kind, title, place, sev, lat, lng, node, status, unit_id, sim bool, scenario, created_at)`, `incident_log(incident_id, at, text)`, `sos(id text pk, place, hazard, people int, injured bool, lat, lng, via, raised_at, status)`, `reports(id uuid pk, user_id, type, lat, lng, description, status, media_id, created_at)`.
- `dispatch`: `hospitals(id pk, name, own, lvl, lat, lng, loc, phone, beds, node, cap, free, inbound, divert, source)`, `units(id pk, type, node, status, task, lat, lng)`, `shelters(id pk, name, node, lat, lng, cap, occ, open)`, `pharmacies(...)`, `stock(pharmacy_id, med_id, qty, pk(pharmacy_id, med_id))`, `assignments(id, incident_id, unit_id, hospital_id, route jsonb, approved_at)`.
- `traffic`: `closures(id pk, name, kind, note, source, lat, lng, active bool)`, `breaks(edge_id pk, reason, created_at, created_by)`.
- `environment`: `rain_obs(station, mm, at)`, `river_levels(river, gauge, level, danger, at)`, `flood_zones(...)`, `landslides(...)`, `flood_state(stage, at)`.
- `events`: `event_log(id uuid pk, routing_key, envelope jsonb, received_at)` with an index on `received_at desc`. This table replaces the old `telemetry_logs`.
- `alerts`: `alerts(id, area, severity, text_en, text_hi, issued_at, issued_by)`.

---

## 8. Frontend integration (the only frontend changes allowed)

1. **`frontend/src/api/config.js`** reads `import.meta.env.VITE_API_URL` (default `/api/v1`), `VITE_WS_URL` (default `/ws/v1/live`) and `VITE_LIVE` (`auto` | `on` | `off`, default `auto`).
2. **`vite.config.js`**: keep the existing config. Add `server.proxy` mapping `/api` → `http://localhost:8000` and `/ws` → `ws://localhost:8000` (with `ws: true`), so there is no CORS in development.
3. **`src/api/client.js`** provides a `fetch` wrapper with a 2.5 s timeout and bearer token. Its `live` flag starts false and turns true after `GET /api/v1/auth/me` or `/healthz` succeeds. When the role changes it gets a token from `/auth/demo-token` (role switching stays instant, so the frontend gets no login screen).
4. **`src/api/live.js`** opens the WebSocket with backoff and dispatches incoming envelopes into the store through one new internal function, `applyRemote(env)`. That function maps `entity.type` to the existing `set()` patches: `hospital.capacity` → `hosp`, `unit.*` → `units`, `shelter.*` → `shelters`, `incident.*` / `assignment.dispatched` → `incidents`, `road_segment.passability` → `closures` / `breaks`, `public_alert.*` → `alerts`, `cascade.computed` → toast plus the Infrastructure view, `pharmacy.stock` → `stock`. It ignores envelopes whose `origin` is this client.
5. **In `store.js`:**
   - `publish()` also fires `client.post('/events', env)` in the background when live. Never `await` it in UI paths.
   - `actions.deliverSos`: when live, `POST /sos`. The backend then runs allocation and returns an `assignment.dispatched` envelope. When the backend is unreachable, run the current local logic unchanged.
   - Route planning (`plan`/`replan`) and `rankOnNet`: try the backend with the existing return shapes, and fall back to the local engine on timeout or error.
   - `sendSos` offline queueing, `flushQueue` and the service worker behaviour stay exactly as they are.
6. **Topbar** gets a small status chip: **Live** (bus connected) or **Local** (offline engine), styled with the existing CSS classes and tokens. This is the only new visible element.
7. Add `frontend/.env.example`, and fix the README: it lists `engine/routing.js` and `engine/allocation.js`, which don't exist, and repeats the pharmacies line. Add a "Running with the backend" section.
8. `tests/click-all.spec.js` must still pass in both modes (backend up and backend down).

---

## 9. Phases (do them in order and commit after each)

### Phase 0: Recon (no code changes)
- Read the whole repo: every service's boilerplate, compose files, Dockerfiles, `.env.example`, `gateway/`, `scripts/`, `tools/`, and the current `frontend/` scaffold.
- Read the partner frontend, which is already copied into the repo at **`nexus-frontend/`** (untracked). Ignore `nexus-frontend/Claude outputs/` (a duplicate of this file) and the empty folder `nexus-frontend/src/{components,views}`.
- Report: the current state, gaps against §4 and §5, port conflicts, and anything in the boilerplate that contradicts this file. Then continue straight to Phase 1 unless something blocks you.

### Phase 1: Move the frontend in
Repo facts to handle first:
- `frontend/` is **not** normal tracked code. It is a nested git repo (its own `.git`, "Initial commit from Create Next App") committed as a gitlink (mode 160000) with no `.gitmodules`. On GitHub it shows as an empty, unclickable folder. Remove it as a gitlink, not with a plain `git rm -r`.
- `start-all.sh` and `stop-all.sh` are deleted in the working tree but tracked in git. Restore them, then move them into `scripts/`.

```bash
git restore start-all.sh stop-all.sh
mkdir -p scripts && git mv start-all.sh scripts/ && git mv stop-all.sh scripts/
git add CLAUDE.md && git commit -m "chore: add master spec, move scripts"

git rm --cached frontend                      # drop the gitlink
mv frontend /tmp/nexus-next-scaffold-backup   # keep a local backup of the Next.js scaffold
mkdir frontend
rsync -a --exclude node_modules --exclude dist --exclude .git --exclude .DS_Store \
  --exclude "Claude outputs" --exclude "src/{components,views}" \
  nexus-frontend/ frontend/
cd frontend && npm install && npm run build && cd ..
```
- After the build passes and the app runs, delete `nexus-frontend/`; the copy now lives in `frontend/`.
- Merge the frontend's `.gitignore` entries into the root `.gitignore`: `node_modules`, `dist`, `*.local`, `.env`, `.next`.
- Update `scripts/start-all.sh` to start the frontend with `npm run dev`, or print the command to do so.
- Acceptance: `git ls-files frontend | head` lists real files (for example `frontend/src/App.jsx`), `npm run dev` shows the app at http://localhost:5173 looking exactly as before, and `npm run build` succeeds.
- Commit: `feat(frontend): replace Next.js scaffold with the team's Vite console`.

### Phase 2: Shared Go module and infra
- `services/shared`: `envelope`, `bus` (RabbitMQ), `httpx` (router helpers, JSON, errors, CORS, request id, recover, slog middleware), `auth` (JWT verify + role middleware), `config` (env loading with defaults), `health`, `pg` (pgx pool + migration runner), `seed` (JSON loader).
- `services/go.work` covering all 11 modules.
- `gateway/docker-compose.yml`: Traefik v3 (entrypoint `:80` mapped to host `8000`, dashboard on host `8081`, Docker provider with `exposedByDefault=false`), RabbitMQ 3-management (host `5672`, `15672`), MinIO (host `9000`, `9001`). It creates the external network `nexus-net`.
- Every service compose file joins `nexus-net`, keeps its own DB on a private network, and sets Traefik labels (`PathPrefix(...)` rules from §5).
- Memory limits suitable for a laptop: Neo4j heap 512m and pagecache 256m; Postgres containers use `postgres:16-alpine`.
- `scripts/start-all.sh` order: gateway → databases healthy → services → seed → frontend. It waits on health checks, not `sleep`. `stop-all.sh` reverses this. Add `--build` and `--clean` flags.
- Acceptance: `./scripts/start-all.sh` succeeds and `scripts/smoke.sh` shows every `/healthz` returning 200 through `localhost:8000`.

### Phase 3: Vertical slice (highest priority). One SOS end to end
identity-access (demo token) → citizen-reporting `POST /sos` → `sos.request` → emergency-dispatch ranks hospitals (calls graph-engine `/routes/plan` for drive times, applies capacity, specialty and inbound load, mirroring `rankOnNet`) → picks the nearest available ambulance → publishes `assignment.dispatched` + `hospital.capacity` + `unit.status` → iot-broker fans out over WS → frontend updates the incident list, hospital inbound count, unit status and Architecture trace.
- Acceptance: with the backend up, the Topbar shows **Live**. Sending an SOS from the Citizen role appears in the Controller's Incidents within 1 s, and `GET /api/v1/events` shows the chain. With the backend stopped, the same flow still works locally.

### Phase 4: Routing and road state
- graph-engine (A* port + table test), traffic-control (closures, breaks, repair), and scenario loading (`SCENARIOS` replay closures).
- Breaking a road in the Routes screen publishes `road_segment.passability` → graph-engine updates → reroute matches the local engine.

### Phase 5: Remaining services
- environmental-monitor, predictive-analytics (port `hazard.js` rain what-if and `simulation.js` evaluation), notification-gateway (alerts with Hindi text from `i18n.js` where available), media-storage (MinIO), Prāṇadhārā assets and the cascade endpoints in graph-engine.

### Phase 6: Frontend adapter
- Everything in §8. Run the Playwright suite with the backend both up and down.

### Phase 7: Python simulator (`tools/simulator/main.py`)
- Uses `requests` + `pika` with a CLI built on `argparse`. Modes:
  - `telemetry`: utility load, rainfall per station and river levels → `POST /api/v1/telemetry/ingest` every second with jitter.
  - `sos-burst --n 10`: realistic SOS at random `FLOOD` localities (read from `tools/seed/out/FLOOD.json`).
  - `chaos --asset <id>`: ramps a substation above 110% for longer than its grace period, which triggers the cascade.
  - `monsoon`: ramps rainfall, flood stage and river levels together for a 3-minute scripted storm.
- Everything it produces is tagged `source: "simulated"`.

### Phase 8: Docs and polish
- `README.md`: a one-paragraph pitch, the architecture diagram (mermaid), quick start, the demo script (§11), real vs simulated data, and the team.
- `docs/architecture.md`, `docs/api.md`, `docs/events.md`.
- Run `go vet ./...` and `go test ./...` in every service. `scripts/smoke.sh` must be fully green.

---

## 10. Priority if time runs short

| Must (demo breaks without it) | Should | Could |
|---|---|---|
| Phase 1 frontend in repo, unchanged | graph-engine A* + traffic-control breaks | GeoJSON topology upload |
| Gateway + start-all + healthz for all 10 | environmental-monitor + predictive-analytics | Redis rate limiting |
| Phase 3 SOS vertical slice, live | Prāṇadhārā cascade demo + simulator `chaos` | MinIO media for reports |
| Live/Local chip + automatic fallback | notification-gateway alerts over WS | Abhayasūchī/Sambharaṇa beyond stubs |
| README quick start | Playwright passes in both modes | Grafana or other dashboards (skip) |

If a service can't be fully implemented in time, ship it with real `/healthz`, `/readyz`, seeded `GET` endpoints and bus wiring, plus a `TODO.md` in its folder. Every service must respond.

---

## 11. Demo script (what must work tomorrow)

1. `./scripts/start-all.sh`, then open http://localhost:5173. The Topbar shows **Live**.
2. **Architecture** → "Trace one SOS".
3. Switch the role to **Citizen** and send an SOS (Trapped, 4 people, injured). Switch back to **Controller**: the incident appears with an ambulance assigned and the best hospital chosen.
4. **Routes** → plan a route → **Break** a step. It reroutes and states the difference. Then **Start the race**.
5. Run `python tools/simulator/main.py chaos --asset sub-01`. **Infrastructure** shows the cascade: the hospital loses power, goes into divert, and new SOS requests route elsewhere.
6. **Early warning** → rainfall what-if, or `simulator monsoon`. Roads close and routes change.
7. **Towers down** → the SOS is held on the device → towers up → it is delivered.
8. Stop the backend (`./scripts/stop-all.sh`). The chip flips to **Local** and everything still works.
9. **Audit trail** + `GET /api/v1/events` show both sides of the record. Open **Situation report** → print it.

---

## 12. Working agreements

- Use small, conventional commits per phase: `feat(dispatch): …`, `fix(graph): …`, `chore(infra): …`.
- Before a large refactor of existing boilerplate, state what you'll change and why in one paragraph, then do it.
- Don't invent real-world facts. New data that isn't in `src/data/index.js` is `source: "simulated"`.
- When something is ambiguous, choose the option that keeps the demo path working, note it in `docs/decisions.md`, and continue.
- At the end of the session, write `STATUS.md` with what is done, what is partial and how to run it.

**Start now with Phase 0.**
