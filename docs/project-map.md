# Project map: what is where

A guided walk through the repo. Paths are from the repo root. Line counts are for non-test Go files, approximate.

## Suggested reading order (about 2 hours)

1. `README.md` (10 min): pitch, quick start, demo, real vs simulated.
2. `docs/architecture.md` (10 min): the diagrams. Keep it open while reading code.
3. Run it: `./scripts/start-all.sh`, click through the console, run the simulator (`tools/simulator/main.py`).
4. `docs/events.md` (15 min): every event, who sends it, who listens. This is the spine of the system.
5. Follow **one SOS** through the code (section "Trace one SOS" below). This teaches you the most.
6. `docs/presentation-guide.md` Part 3 and 4: the algorithms and schemas, then read the code files named there.
7. `docs/decisions.md` and `STATUS.md`: why things are the way they are, and what is not done.

## Top level

| Path | What it is |
|---|---|
| `CLAUDE.md` | The master spec the build followed. Wins over older plans. |
| `README.md`, `STATUS.md` | Overview and honest status and test results. |
| `docs/` | `architecture.md` (diagrams), `api.md` (every endpoint), `events.md` (every event), `decisions.md` (23+ choices and why), `presentation-guide.md` (checklist, demo script, formulas, schemas, Q&A), `project-map.md` (this file). |
| `frontend/` | The partner's console. |
| `services/` | The ten Go services plus the shared module. |
| `gateway/` | Traefik, RabbitMQ, MinIO, Redis (shared infrastructure). |
| `scripts/` | `start-all.sh`, `stop-all.sh`, `seed.sh`, `smoke.sh`, `lib.sh`. |
| `tools/` | Seed export, test fixture generators, the simulator, the compose generator. |
| `.env`, `.run/` | Generated secrets and runtime state (gitignored; never commit). |

## frontend/ (the console, React 18 + Vite + MapLibre)

| Path | What is in it |
|---|---|
| `src/data/index.js` | **Every dataset**, with source keys: hospitals, pharmacies, shelters, units, flood localities, landslides, road nodes and edges, closures, scenarios, modules. The single source of truth the backend seeds from. |
| `src/state/store.js` | **The one store.** All actions, `publish()` (event envelope), routing and ranking calls, `applyRemote()` (events from the backend), `hydrate()`, `remoteSos()`. Most of what the console does is here. |
| `src/engine/roadnet.js` | Road network, A*, breaks, nearest lookups. (The Go version is a port.) |
| `src/engine/hazard.js`, `simulation.js` | Rainfall inference and the Evaluation sweep. (Also ported to Go.) |
| `src/api/` | **What was added for the backend:** `config.js` (env), `client.js` (fetch, tokens, login session), `live.js` (WebSocket with backoff). |
| `src/views/` | One file per screen: Overview, Incidents, Map, Routes, Hospitals, Pharmacies, Sos, Shelters, Responders, Warning, Infrastructure, Evaluation, Modules, Architecture, Sitrep, Audit, Sources, Settings, Assignment. |
| `src/components/` | Topbar (has the Live/Local chip), Sidebar, MapView, Drawer, CommandPalette, Login (only in secure mode). |
| `public/roads.json` | The real OpenStreetMap roads (5.5 MB, 167,500 junctions). |
| `public/sw.js` | Service worker for the offline citizen shell. |
| `tests/click-all.spec.js` | Playwright: clicks every control; plus two behavioural tests. |
| `vite.config.js` | Proxies `/api`, `/svc`, `/ws` to the gateway. |

## services/ (Go; each has `cmd/main.go`, `internal/{http,store,domain,bus}`, `migrations/`, `Dockerfile`, `docker-compose.yml`, `.env.example`)

| Service | Where to look first | What it owns |
|---|---|---|
| `shared/` (1.1k) | `bus/bus.go`, `httpx/httpx.go`, `auth/auth.go`, `envelope/`, `pg/`, `seed/` | Message bus client, HTTP plumbing, JWT and roles, the event envelope, migrations, seed ports. Every service imports it. |
| `graph-engine/` (2.4k) | `internal/domain/roadnet.go` (A*), `cascade.go`, `assets.go` (utility graph), `state.go` (road state, flood), `evaluate.go` (Monte Carlo), `internal/store/neo4j.go` | **The brain:** routing, reach, drive-time matrices, cascade, evaluation. Biggest service. |
| `emergency-dispatch/` (1.3k) | `internal/domain/rank.go` (hospital ranking), `internal/dispatch/dispatch.go` (the orchestrator), `internal/store/store.go` | Hospitals, units, shelters, pharmacies, assignments. Decides the hospital and ambulance for an SOS. |
| `citizen-reporting/` (0.5k) | `internal/http/handlers.go` (`/sos`), `internal/domain/sos.go` | SOS, incidents, hazard reports. |
| `iot-broker/` (0.5k) | `internal/hub/hub.go` (role-filtered WebSocket), `internal/http/handlers.go` (`/events`, telemetry ingest) | Event log, live fan-out. |
| `predictive-analytics/` (0.5k) | `internal/domain/hazard.go`, `internal/http/handlers.go` | Rain to road closures, risk, evaluation endpoint. |
| `environmental-monitor/` (0.4k) | `internal/bus/consumer.go`, `internal/domain/river.go` | Rainfall, rivers, flood stage. |
| `traffic-control/` (0.5k) | `internal/http/handlers.go` | Closures and reported breaks. |
| `notification-gateway/` (0.4k) | `internal/domain/cap.go` | CAP-style alerts, English and Hindi, SMS stub. |
| `media-storage/` (0.2k) | `internal/http/handlers.go` | Attachments in MinIO. |
| `identity-access/` (0.3k) | `internal/http/handlers.go`, `internal/domain/password.go` | Login, demo tokens, roles. |
| `go.work` | | Lets all 11 modules build together locally. |

Each service's `TODO.md` says what is deliberately thin. Tests sit next to the code as `*_test.go`; the important ones are `graph-engine/internal/domain/roadnet_test.go` (parity with the JS router), `evaluate_test.go`, `cascade_test.go`, `emergency-dispatch/internal/domain/rank_test.go`.

## gateway/ and scripts/

| Path | What it does |
|---|---|
| `gateway/docker-compose.yml` | Traefik (front door), RabbitMQ (the bus), MinIO (files), Redis. Creates the Docker network `nexus-net`. |
| `gateway/traefik.yml` | Gateway configuration. Routes come from labels in each service's compose file. |
| `scripts/start-all.sh` | Gateway, then databases, then images, then services, then health waits, then frontend. Flags: `--build`, `--clean`, `--secure`, `--demo`, `--no-frontend`. |
| `scripts/smoke.sh` | Curl checks for everything; `--e2e` also does an SOS and writes. |
| `scripts/seed.sh` | Exports seed data; `--reset` resets the running system. |

## tools/

| Path | What it does |
|---|---|
| `tools/seed/export-data.mjs` | Turns `frontend/src/data/index.js` into JSON in `tools/seed/out/` (mounted into every service). |
| `tools/seed/gen-route-fixtures.mjs`, `gen-eval-fixture.mjs` | Run the JS engines and save their output, so the Go ports can be tested against it. |
| `tools/simulator/main.py` | `telemetry`, `sos-burst`, `chaos`, `monsoon`. |
| `tools/infra/gen-compose.py` | Regenerates every service's Dockerfile and compose file from one table. Edit this, not the generated files. |
| `tools/parity/rank-parity.mjs` | Compares the console's hospital ranking with the backend's. Not yet run. |

## Trace one SOS through the code

1. `frontend/src/views/Sos.jsx`: the SOS button calls `actions.sendSos`.
2. `frontend/src/state/store.js`: `sendSos`, then `deliverSos`, then `remoteSos` posts to `/api/v1/sos` (falls back to `deliverSosLocal`).
3. `services/citizen-reporting/internal/http/handlers.go`: `sos()` stores it and publishes `sos.request`.
4. `services/emergency-dispatch/internal/bus/consumer.go`, then `internal/dispatch/dispatch.go`: `HandleSOS` ranks hospitals (`domain/rank.go`), asks `graph-engine` for drive times (`/routes/times`), claims an ambulance, publishes `assignment.dispatched`, `hospital.capacity`, `unit.status`.
5. `services/graph-engine/internal/http/handlers.go`: `times()` runs one Dijkstra from `domain/roadnet.go`.
6. `services/citizen-reporting/internal/bus/consumer.go`: marks the incident "Unit assigned".
7. `services/iot-broker/cmd/main.go` and `internal/hub/hub.go`: log it, then send it to the right WebSockets.
8. `frontend/src/api/live.js`, then `store.js` `applyRemote()`: the console updates.

## Trace the power cascade

`tools/simulator/main.py` (`chaos`), then `iot-broker` `/telemetry/ingest`, then bus `telemetry.raw`, then `graph-engine/internal/bus/consumer.go`, then `domain/cascade.go` (`Telemetry` for the grace period, `Fail` for the walk), then `internal/http/handlers.go` (`announce`: asset.status, hospital.capacity, cascade.computed), then `notification-gateway` (auto alert), then `frontend/src/views/Infrastructure.jsx`.
