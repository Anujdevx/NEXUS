# NEXUS: your briefing

Everything you need to present and defend the project: what to do before the demo, how to present it, every algorithm and
formula, every schema and relation, and the honest limits. Numbers and formulas below are taken from the code.

---

## Part 1. What to do, step by step

### The day before (tonight)

1. **Freeze the code.** Do not change anything tonight. Run `git tag demo-ready` so you can always get back to this state.
2. **Docker Desktop running, internet on.** Close other heavy apps (the development Mac was at load 30+ and it showed).
3. **First full start, on the demo laptop:** `./scripts/start-all.sh --build`. The first run builds ten images (several minutes). Then
   `./scripts/smoke.sh --e2e`. It must end "smoke test green".
4. **Install the simulator's two Python packages:** `pip install -r tools/simulator/requirements.txt`. Test `python3 tools/simulator/main.py sos-burst --n 3`.
5. **Rehearse the 8-step demo twice** (Part 2). Between rehearsals: `./scripts/seed.sh --reset` (puts the system back to the opening state).
6. **Decide the mode.** Default `./scripts/start-all.sh` gives the instant role switch (simplest on stage). `--secure` gives real sign-in
   and prints three accounts. For a phone, use secure mode and open the printed address on the same Wi-Fi.
7. **Network.** Use your phone's hotspot or a router you control. Venue Wi-Fi often blocks device-to-device traffic, which breaks the phone demo.
   The map tiles need internet; everything else works without it.
8. **Backup plans, all three:**
   - Record a screen video of a clean run (QuickTime, 5 minutes).
   - If Docker fails on stage: `cd frontend && npm install && npm run dev`. The chip reads **Local** and the whole console still works. Say so; it is a feature.
   - Keep `docs/architecture.md` and this file open in a tab.
9. **Fill in** the team names in `README.md` (Team section) and your guide and institution.
10. **Print or save the Situation report** (console, *Situation report*, print) as a PDF after a rehearsal, as a handout.

### One hour before

1. `./scripts/start-all.sh` (or `--secure`), wait for "NEXUS is up".
2. `./scripts/seed.sh --reset`, then `./scripts/smoke.sh`.
3. Open the console on the projector. Chip must read **Live**. Hard-refresh once.
4. Open a second terminal with the simulator commands typed but not run. Open the phone on the console address (secure mode) and sign in as the citizen.
5. Phone on silent, laptop charger in, notifications off.

### If something breaks on stage

| Symptom | Do this |
|---|---|
| Chip says **Local** | The backend is down. Carry on; say "the console falls back by design". Then `./scripts/start-all.sh` in a terminal. |
| An old incident or closed road you did not expect | Leftover rehearsal state. `./scripts/seed.sh --reset`, refresh. |
| Phone cannot open the page | Not the same Wi-Fi, or hotspot isolation. Use the laptop for the citizen role (role menu, Citizen). |
| Map is grey with "API KEY REQUIRED" | That is the tile provider's watermark, unchanged from the original. Roads and routes still draw. |
| Anything else | `docker ps` (all `nexus-*` should be Up), `./scripts/smoke.sh` shows which service is red. |

---

## Part 2. How to present it (about 10 minutes)

**Opening (1 min): the problem.** In a disaster, hospitals, roads, shelters, rescue teams and utilities each sit in a different agency's
system. When a road breaks or a substation fails, nobody sees the whole picture, and an ambulance gets sent into the break or to a
full hospital. Use the console's own real events: the 16 Sep 2025 Sahastradhara cloudburst (192 mm, bridges and roads washed away).

**The idea (1 min).** NEXUS is a coordination layer: a digital twin of the district's response infrastructure. Every agency publishes into
one event bus (Sūtra) and reads from it. One change cascades through the graph, and NEXUS reroutes, re-ranks and alerts.

**Architecture (1 min).** One slide (the diagram in `docs/architecture.md`): console, gateway, ten services, message bus, databases.
Say: "Ten domains talking to each other directly would need 45 links; one bus needs 10."

**Live demo (6 min).** Follow this order. What to say is in italics.

1. **Chip says Live.** *"The console works alone; with the backend up it is connected to the bus."*
2. **Architecture, Trace one SOS.** *"This is the path one request takes."*
3. **Citizen SOS** (role menu, Citizen; or the phone). Hold the button, Trapped, 4 people, injured Yes, send. *"The request is accepted immediately; the backend picks the hospital and ambulance."* Switch to **Controller**: Incidents shows it with an ambulance; Hospitals shows the choice and why (reach, capacity, specialty, inbound load).
4. **Routes.** Plan a route, press **Break** on a step. *"It reroutes and states the difference."* Press **Start the race**: Nexus against the baseline that picks the nearest hospital and ignores roads and capacity.
5. **The cascade.** In a terminal: `python3 tools/simulator/main.py chaos --asset sub-01`. After about six seconds the substation fails. Show **Infrastructure** (the lifeline cascade panel), the hospitals now in divert, the public alert toast. Send another SOS: it goes to a hospital that still has power. *"All utility data here is simulated and labelled."* Press **Restore utilities**.
6. **Early warning.** Drag the rainfall what-if, or run `python3 tools/simulator/main.py monsoon --duration 60`. Roads close at 80 mm and 150 mm; routes change; the flood spreads.
7. **Towers down.** Citizen screen, Cut the network, send an SOS (held on the device), Peer finds an uplink (delivered). *"Store, carry, forward."* (Simulated in the UI.)
8. **Stop the backend** (`./scripts/stop-all.sh`) or close the laptop lid on Docker. Chip flips to **Local**; everything above still works.
9. **Audit trail** in the console and `GET /api/v1/events` on the server: two sides of one record. Open **Situation report**.

**Evidence (1 min).** Open **Evaluation**: Nexus against the baseline as connectivity falls (time to care, unserved, delivery). *"These are simulation results, not field results; the road network is real."*

**Close (30 s): honest limits and next steps.** Prototype on simulated operational data; next: live feeds, citizen app with GPS and map, city setup from a file, security hardening.

---

## Part 3. The algorithms and formulas

None of this is machine learning. It is graph search, rule-based inference, Monte Carlo simulation and graph propagation. Say so.
That is a strength (explainable, testable), not a weakness.

### 3.1 Road network and travel time

- **Data:** real OpenStreetMap roads for the district (`frontend/public/roads.json`): **167,500 junctions, 168,323 road segments**.
  A schematic 68-junction fallback network exists if the file is missing.
- **Distance (haversine):** `d = 2R · asin( sqrt( sin²(Δφ/2) + cos φ1 · cos φ2 · sin²(Δλ/2) ) )`, R = 6371 km (the code uses 12742 = 2R).
- **Segment time:** `minutes = km / speed × 60`. Speed by road class: motorway 60, trunk 50, primary 38, secondary 32, tertiary 26,
  unclassified 20, residential 16 km/h (all assumed). The schematic network uses h 42, a 26, m 18 km/h, with a winding factor on length of 1.15, 1.25, 1.75.
- **All roads are treated as two-way.**

### 3.2 Routing: A\* search

`f(n) = g(n) + h(n)`, where `g` is the travel time so far and `h(n)` is the straight-line distance to the goal in km, read as minutes.
Since no road is faster than 60 km/h, `h` never overestimates, so A\* returns the true shortest path. Ties are resolved by a binary heap
that matches the console's JavaScript exactly, which is why the Go and JS routes are identical (tested on 11 fixed pairs). A route takes about 30 ms on 167,500 junctions.

### 3.3 Broken and slow roads: weights, not deletions

A reported break is not removed from the graph. Its cost is multiplied by how much you believe the report:

| Situation | Cost used by the planner |
|---|---|
| Open road | `t` |
| Broken road, report confidence `c` | `t × (1 + c × 40)` (soft), or infinity when a hard route is required |
| Slow road (waterlogged) | `t × (1 + c × 1.2)` |
| Real driving time on a slow road | `t × 2.2` |

Default confidence is 0.9, adjustable on the Routes screen. A wrong report therefore degrades the route gracefully instead of cutting the network.
**Spatial index:** a 0.01° grid; nearest node and nearest edge search rings of cells outward. A break reported at a point blocks all segments within 30 m.

### 3.4 Flood that spreads (illustrative, not hydraulic)

For the i-th documented flood-prone locality: `onset_i = 4 + (37·i mod 56)`, then
`radius_i = clamp((stage − onset_i) / 40, 0, 1) × 0.6 km`.
Roads within `radius` are slowed; roads within `0.55 × radius` are broken. `stage` runs 0 to 100.

### 3.5 Hospital allocation (multi-constraint ranking)

For each hospital: `cost = reach + specialty + capacity + load`, lowest wins.

```
reach     = drive_minutes                      (or 999 + 240 if no open road leads there)
specialty = +45 if the hospital's level cannot serve the need
capacity  = +90 if free beds = 0 (a hospital in divert counts as 0 free)
            +8  if fewer than 3 beds free
load      = +5 per patient already inbound
```

Needs served: Tertiary: Trauma, Cardiac, Maternity, General. Secondary: Trauma, Maternity, General. CHC: Maternity, General. PHC: General.
The **baseline** it is compared against is "nearest hospital in a straight line".
**Ambulance:** the available ambulance with the smallest drive time to the patient, claimed atomically in the database so two SOS never take the same unit.
**Drive times** for all hospitals come from one Dijkstra run (`/routes/times`), not 23 separate searches.

### 3.6 Predictive analysis: rainfall to road passability

Rule-based inference (Pūrvasūchanā), with illustrative thresholds, not published Garhwal values:

| Segment type | Affected when rainfall ≥ |
|---|---|
| Low-lying, "slow" segment | 80 mm |
| Hill road ("closed" kind, class m) | 150 mm |

- **Confidence:** `conf = 0.5 + 0.45 × min(1, |mm − threshold| / threshold)`, so 0.5 at the threshold and up to 0.95 well past it.
- **Rain used:** the **maximum** over all stations (so stations do not flap the decision).
- **Risk band:** below 40 mm low; 40 to 80 moderate; 80 to 150 high; 150 and above severe. High and severe raise a public alert.
- **It never reopens a documented closure:** it only flips closures it switched on itself.
- **River status:** per gauge, `below` under the warning level, `warning` from it, `danger` from the danger mark (the simulated gauges' marks are invented and labelled).

### 3.7 The cascade (the "hidden relations")

An asset graph (Neo4j): `(dependent)-[:DEPENDS_ON {criticality, grace_period_ms}]->(provider)`.

```
fail(root): root = FAILED.  queue = [root]
repeat per level (this is one wave):
  for every failed f, for every dependent d of f:
      d.load += 100 × criticality(d → f)
      if d.load > d.failure_threshold:  d = FAILED  → joins the next level
      else:                             d = DEGRADED
```

- A **hospital that is FAILED or DEGRADED** is put into divert, so dispatch ranks it out.
- **Automatic trigger:** an asset fails by itself when its load stays above its threshold for longer than its **grace period** (substation 5 s). The simulator's `chaos` mode does exactly that: ramps load to 110%+ and holds it.
- **How the relations are built** (the part to be honest about): we do not *discover* them with learning. They are **modelled** from rules, then **propagated**:
  - pump and tower depend on the nearest substation (criticality 0.8 and 1.0);
  - a hospital depends on the nearest substation and the nearest pump only if within a **3.5 km feeder zone** (criticality: tertiary 0.9, secondary 0.6, CHC and PHC 0.4; pump 0.4);
  - a shelter depends on the nearest tower and substation (0.3 each).
  What looks "hidden" is the **second-order effect**: a substation fails, its pump fails (wave 1), and the hospital that only depended on that pump, or whose load was already pushed up, now fails or degrades (wave 2). Nobody drew that arrow from the substation to the hospital. The graph walk finds it.
- Measured: failing `sub-01` gave **11 failed, 5 degraded, 2 waves**. All utility assets and dependencies are **simulated**.

### 3.8 Evaluation: Monte Carlo simulation

For each connectivity level `p ∈ {1, 0.8, 0.6, 0.4, 0.2}` and each policy (Nexus vs baseline), 30 runs × 40 SOS, same random seeds for both:

- Random generator: **mulberry32** (seeded; identical in JS and Go).
- Each SOS: random flood-prone origin, random need, online with probability `p`.
- **Delay if offline:** exponential with mean 12 min (Nexus peer relay) or 25 min (baseline retry). Delivered if delay ≤ 30 min.
- **Nexus** knows the road picture and hospital capacity with probability `p`; the **baseline** never does: it drives into a break (loses 3 min and replans) and is turned away from a full or wrong hospital (loses 6 min, up to 4 hops).
- A casualty is **unserved** if not treated within 150 minutes.
- Reported with a 95% interval: `CI = 1.96 × sd / sqrt(n)`.

Say: *"simulation results, not field results."*

### 3.9 Seeded simulated state (so everything agrees)

- **Free beds:** `cap × (0.18 + ((37·i) mod 50)/100)` rounded, minimum 1, where `cap` = 60 tertiary, 28 secondary, 10 CHC, 3 PHC.
- **Pharmacy stock:** a deterministic sine hash `h = frac(|sin((i+1)·12.9898 + (j+1)·78.233) · 43758.5453|)`; zero stock if `h < 0.2`, else `round(base × (0.15 + 1.2h))`.
- The Go versions are tested to reproduce the console's numbers exactly.

### 3.10 Security and limits

- Login: **PBKDF2-HMAC-SHA256**, 120,000 iterations. Tokens: **JWT HS256**, 12 h. Rate limits: `/login` 10 per minute and `/sos` 30 per minute, per IP (Redis, with in-memory fallback).
- **Roles:** Citizen (SOS only), Responder (assignment, map), Controller (everything). Citizens get 403 on operational data.

---

## Part 4. Schemas and relations

### 4.1 Where things live

| Service | Store | Tables / data |
|---|---|---|
| identity-access | Postgres `identity` | `users` |
| citizen-reporting | Postgres `reports` | `sos`, `incidents`, `incident_log`, `reports` |
| emergency-dispatch | Postgres `dispatch` | `hospitals`, `units`, `shelters`, `pharmacies`, `stock`, `assignments` |
| traffic-control | Postgres `traffic` | `closures`, `breaks` |
| environmental-monitor | Postgres `environment` | `rain_obs`, `river_levels`, `flood_zones`, `landslides`, `zoning`, `flood_state` |
| iot-broker | Postgres `events` | `event_log` (the server-side audit trail) |
| notification-gateway | Postgres `alerts` | `alerts` |
| graph-engine | Neo4j + memory | `Junction`, `Asset`, `Region`; the 167,500-junction road graph in memory |
| predictive-analytics | Redis | cache (evaluation results, last rainfall) |
| media-storage | MinIO | bucket `nexus-media` |

### 4.2 Postgres tables (columns that matter)

```
users(id uuid PK, email UNIQUE, password_hash, role CHECK Controller|Responder|Citizen, created_at)

sos(id PK 'SOS-12345', place, hazard, people, injured bool, lat, lng, via, raised_at, status, client)
incidents(id PK, kind, title, place, sev, lat, lng, node, status, unit_id, sim bool, scenario, detail, created_at)
incident_log(id PK, incident_id → incidents.id ON DELETE CASCADE, at, text)
reports(id uuid PK, user_id, type, lat, lng, description, status, media_id, created_at)

hospitals(id PK, name, own, lvl, lat, lng, loc, phone, beds, node, cap, free, inbound, divert bool, source)
units(id PK 'AMB-01', type, node, status, task, lat, lng, step)
shelters(id PK, name, node, lat, lng, cap, occ, open bool)
pharmacies(id PK, name, lat, lng, loc, hrs)
stock(pharmacy_id → pharmacies.id, med_id, qty, PK(pharmacy_id, med_id))
assignments(id PK, incident_id, unit_id, hospital_id, route jsonb, approved_at)

closures(id PK 'c_kolhu', name, kind closed|slow, note, source, lat, lng, active bool)
breaks(edge_id PK, reason, created_at, created_by, lat, lng, name)

rain_obs(id, station, mm, at, label, source)      river_levels(id, river, gauge, level, danger, status, lv, at, observed, source)
flood_zones(id PK, name, river, lat, lng, note, node, sources[], source)   landslides(id PK, name, road, lat, lng, note, sources[], source)
zoning(class PK, share, tone, label, source)      flood_state(id, stage, at, source)

event_log(id uuid PK, routing_key, envelope jsonb, received_at)     alerts(id PK, area, severity, text_en, text_hi, issued_at, issued_by, source, cap jsonb)
```

### 4.3 Relations

Each service owns its own database, so there are **no foreign keys across services**. The links are by shared ids, carried on the bus:

```
SOS-12345:  sos.id = incidents.id = assignments.incident_id
assignments.unit_id → units.id        assignments.hospital_id → hospitals.id
incidents.unit_id → units.id          incidents.node / hospitals.node / shelters.node → a road junction id
reports.media_id → an object key in MinIO        users.id → the JWT 'sub'
closures.id (c_kolhu) → the road segments near its point, resolved inside graph-engine
```

### 4.4 Neo4j graph

```
(:Junction {id, name, lat, lng})-[:ROAD {id, name, class, km, speedKmh, status, confidence}]->(:Junction)   (schematic network only; both directions)
(:Asset {id, name, type POWER|WATER|TELECOM|HOSPITAL|SHELTER|PUMP|BRIDGE, lat, lng, status ACTIVE|DEGRADED|FAILED,
         load_percentage, failure_threshold, grace_period_ms, base_load, source, ref})
(:Asset)-[:DEPENDS_ON {criticality 0..1, grace_period_ms}]->(:Asset)       dependent → provider
(:Asset)-[:NEAR]->(:Junction)        (:Asset)-[:LOCATED_IN]->(:Region {id:'dehradun'})
constraints: Junction.id UNIQUE, Asset.id UNIQUE, Region.id UNIQUE
```
Seeded assets: 23 hospitals, 7 shelters, 3 substations, 4 water pumps, 3 telecom towers (40 total, 90 dependencies).

### 4.5 The message bus

- One durable topic exchange **`nexus.events`**; routing key `entity.type` (for example `sos.request`, `road_segment.passability`, `cascade.computed`).
- Each service has a durable queue `q.<service>`, manual acks, prefetch 32, and a dead-letter exchange `nexus.dlx` for failed messages.
- **Envelope** (one JSON shape everywhere): `id, entity, type, geo, status, capacity, confidence, timestamp, source, origin, payload`.
- Every key, producer, consumer and payload is in **`docs/events.md`**. Every endpoint is in **`docs/api.md`**.

### 4.6 The data flow of one SOS, in words

Citizen device → `POST /sos` (citizen-reporting stores it and publishes `sos.request`) → emergency-dispatch asks graph-engine for drive times,
ranks hospitals, claims an ambulance, publishes `assignment.dispatched`, `hospital.capacity`, `unit.status` → citizen-reporting marks the
incident "Unit assigned" → iot-broker writes everything to `event_log` and pushes it to the right WebSockets by role → the console updates.

---

## Part 5. Real versus simulated (say this out loud)

| Real, with a named source | Simulated and labelled |
|---|---|
| Hospital names, places, phones; pharmacy names and hours; flood-prone localities; landslide points; infrastructure register; replay incidents and closures (20 Jul 2026, 16 Sep 2025, 20 Aug 2022); documented rainfall, river status, zoning; the OpenStreetMap road network | Free beds, inbound load; pharmacy stock; ambulances and teams; shelters and occupancy; SOS requests; **all power, water and telecom assets and their dependencies**; simulator telemetry; the flood spread; rainfall thresholds; road speeds |

---

## Part 6. Questions you will be asked, with honest answers

| Question | Answer |
|---|---|
| Is this machine learning? | No. Graph search, rule-based inference, Monte Carlo simulation and graph propagation. That is deliberate: every decision can be explained and tested. |
| How are the hidden dependencies found? | They are modelled by rules (nearest feeder within 3.5 km, criticality by facility level) and propagated by a graph walk; the second-order effects emerge from the walk. With real utility data they would come from the utilities' own network maps. |
| Is the data real? | Places and events are; operational state (beds, ambulances, utilities) is simulated and labelled everywhere. No agency exposes live feeds, which is why a coordination layer is needed. |
| How accurate is the prediction? | Thresholds are illustrative, not calibrated. The framework is what is demonstrated; calibration needs historical rainfall and road-damage records. |
| What if the backend is down? | The console falls back to its local engine automatically (chip reads Local). The demo never depends on the backend. |
| How fast is an SOS handled? | Accepted at once; assignment in about 0.1 to 0.6 s in tests. Routing is about 30 ms on 167,500 junctions. Not load-tested. |
| Can citizens see everything? | No. Citizen tokens get 403 on operational data and the console shows only the SOS screen. Known gaps: see the list below. |
| Is it production ready? | No. It is a working prototype. Missing: HTTPS, real accounts per user, live data feeds, load testing, backups, per-city data scoping, a lean citizen app with GPS and map. |
| What is the novelty? | One event bus plus one shared road and infrastructure graph that every agency reads, with cascade propagation and capacity-aware dispatch, working offline with automatic fallback. |

### Known limitations to state yourself before anyone asks

- A Responder token can publish arbitrary events (can fake status or reset the demo state). The stream filters trust a client-supplied id.
- The assignment sent to a citizen's device includes hospital ranking details a citizen does not need.
- The citizen app is the full console: no GPS (the location is a dropdown), no map, no per-person accounts.
- One district only; adding a city means editing datasets, not loading a file.
- Not load-tested; single copy of every service; no backups, no HTTPS.
- Playwright click-through was not run on the Overview, Map and Routes screens (the 3D map hangs under software rendering on the development machine, and the original console hangs the same way).

---

## Part 7. Cheat sheet

```bash
./scripts/start-all.sh               # everything, demo mode          --secure for real sign-in   --build to rebuild images
./scripts/smoke.sh --e2e             # must be green
./scripts/seed.sh --reset            # back to the opening scenario between rehearsals
./scripts/stop-all.sh                # stop (add --clean to wipe data)

python3 tools/simulator/main.py chaos --asset sub-01        # power cascade
python3 tools/simulator/main.py monsoon --duration 60       # storm
python3 tools/simulator/main.py sos-burst --n 10            # ten SOS
python3 tools/simulator/main.py telemetry                   # background readings

http://localhost:5173   console        http://localhost:8000   gateway (the script prints the real port)
http://localhost:8081   Traefik        http://localhost:15672  RabbitMQ (user nexus, password in .env)
```

Files to open if asked: `README.md`, `docs/architecture.md`, `docs/events.md`, `docs/api.md`, `docs/decisions.md`, `STATUS.md`.
Code to point at: routing `services/graph-engine/internal/domain/roadnet.go`, cascade `.../cascade.go`, ranking `services/emergency-dispatch/internal/domain/rank.go`,
inference `services/predictive-analytics/internal/domain/hazard.go`, simulation `services/graph-engine/internal/domain/evaluate.go`.
