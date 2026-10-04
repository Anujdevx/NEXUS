# Decisions

Where the spec was ambiguous or reality differed, this is what was chosen and why. The rule: keep the demo path working.

1. **Gateway port.** The spec says host port 8000. On this machine another project's container held it, so `start-all.sh`
   keeps its own gateway port if one is already running, otherwise uses `GATEWAY_PORT` (default 8000) when free and falls back to
   8010 to 8013. The choice is written to `.run/ports.env`; the frontend started by the script and `scripts/smoke.sh` read it.
   Postgres and Redis are never published to the host, because 5432 and 6379 are commonly taken.
2. **Traefik v3.6, not an older v3.** Docker Engine 29 raised its minimum API version to 1.44; older Traefik clients cannot talk to it.
3. **Go 1.25 in the build image.** The current versions of the approved dependencies require it (`go 1.25.0` in `go.mod`).
   Spec said 1.22 or newer. The final image is `alpine:3.20` (not distroless) so Docker health checks can use `wget`.
4. **One Redis, shared.** It is in `gateway/`, not per service. It is optional everywhere: rate limiting and the evaluation
   cache fall back to process memory when it is down.
5. **One Postgres per service, but not for predictive-analytics or media-storage.** They keep no relational data (Redis and MinIO).
6. **Neo4j holds assets and the schematic road graph, not the OSM roads.** The real network has about 170,000 junctions;
   loading it into a 512 MB Neo4j on a laptop would cost minutes and gain nothing, since routing runs over an in-memory graph.
   The asset graph (`:Asset`, `DEPENDS_ON`, `NEAR`, `LOCATED_IN`) is seeded into Neo4j, loaded back on boot, and statuses are
   persisted after each cascade. If Neo4j is slow to start, routing and the cascade still work from memory.
7. **Neo4j GDS is not used.** Routing and the cascade run in Go (as the spec says).
8. **The console renders routes locally and the backend confirms them.** `computeRoute` is synchronous and drives the map,
   the race and every view, so it keeps using the local engine instantly. When the backend graph is the same network (same source, node and edge count),
   the console also asks the backend to plan the route with the identical inputs and marks the route confirmed; a difference
   is logged to the console and the local route stands. Hospital choice and ambulance choice for an **SOS** are made by the
   backend when live (the spec's vertical slice), and fall back to the local logic if no assignment returns within five seconds.
9. **The console takes shared state from the backend when it connects.** On every (re)connect it reads hospitals, units,
   shelters, stock, closures, breaks and SOS incidents, because the backend is the picture all consoles and services agree on.
   A role switch reconnects the WebSocket with a token for the new role, which also resyncs the state a Citizen stream cannot carry.
   `Settings, Reset all state` publishes `scenario.loaded`, and every service reloads its seed state.
10. **Role switching has no login screen.** The console asks `/auth/demo-token` for a token per role (spec §8.3). `DEMO_MODE=false`
    turns that off; `/auth/login` works with the seeded demo users whose passwords are in `.env`.
11. **Two small additions to the console beyond the chip.** The Infrastructure screen shows a "Lifeline cascade" panel, with a
    Restore utilities button, but only while a cascade exists (spec §8.4 asks for the cascade there; §8.6 asks for no other visible change).
    Offline, and live with no cascade, the screen is byte-for-byte what it was. `publish()` adds `id`, `origin` and `payload` only when live.
12. **Extra consumers beyond the spec's table.** The spec's table is the minimum. Added because the data flow needs them:
    graph-engine also consumes `scenario.*` and `telemetry.raw` (utility load), traffic-control consumes `road_segment.*` and
    `scenario.loaded`, emergency-dispatch consumes `hospital.capacity`, `unit.*`, `shelter.*`, `pharmacy.stock`,
    `assignment.dispatched` and `incident.*` from other producers (to mirror the console's changes), citizen-reporting consumes
    `incident.*`, notification-gateway consumes `public_alert.cap_alert` (to record console-issued alerts). emergency-dispatch does
    not subscribe to `road_segment.*`: graph-engine keeps the road state and dispatch asks it for drive times. predictive-analytics
    subscribes to `rainfall.*`, `river.*` and `scenario.loaded` instead of `telemetry.raw`, because environmental-monitor turns
    telemetry into those events.
13. **The Evaluation sweep runs in graph-engine.** It needs the road graph. predictive-analytics exposes
    `POST /analytics/evaluation` and proxies to `graph-engine /routes/evaluate`, caching the result.
14. **Rain-inferred closures never reopen documented ones.** predictive-analytics asks traffic-control which closures are already
    in force and only flips the ones it turned on itself.
15. **A what-if is never a public alert.** `hazard.risk` carries its basis; notification-gateway ignores `what-if`.
16. **Hospital dependencies in the utility graph.** Each hospital depends on the nearest substation and water pump only when
    within 3.5 km (a feeder zone); a hospital outside any feeder is assumed to have another supply. Without this, one substation
    would take out almost every hospital in the district and the demo would have nowhere to send anyone.
    Criticality: tertiary 0.9, secondary 0.6, CHC and PHC 0.4. All of it is simulated.
17. **Password hashing is PBKDF2-HMAC-SHA256 from the standard library**, not bcrypt, because `golang.org/x/crypto` is not on the approved list.
18. **Frontend dependency.** `@playwright/test` was added as a dev dependency (the README told users to install it by hand).
    `frontend/public/roads.json` (5.5 MB, real OpenStreetMap roads) is committed so a fresh clone routes on real roads offline and
    graph-engine has the same network as the console.
19. **`frontend/vite.config.js`** proxies `/api`, `/svc` and `/ws` to `NEXUS_GATEWAY` (default `http://localhost:8000`) for both
    `dev` and `preview`.
20. **Seed files are mounted, not copied.** `scripts/seed.sh` runs the exporter; every service mounts `tools/seed/out` read-only at `/seed`
    (the spec said "copies the outputs into each service's seed volume"). One directory, no drift. `seed.sh --reset` also publishes
    `scenario.loaded` so a running system reloads its seed state.
21. **The Playwright suite refuses `roads.json` and Overpass.** The suite clicks controls, not road data, and rebuilding the real
    170,000-junction graph on each of hundreds of page loads made one view exceed its 240 s budget. It runs on the built-in network, as it
    did before `roads.json` existed. Run it twice, with the backend reachable and with `NEXUS_GATEWAY` pointed at a dead port (frontend/README.md).
22. **`compose` needs the root `.env`.** The scripts export it (`COMPOSE_ENV_FILES`). Running `docker compose` by hand without it falls back to
    development defaults for secrets, which will not match the running stack. Use the scripts.
23. **The production service worker no longer caches `/api`, `/svc` or `/ws`.** It cached every same-origin GET, which would have served stale
    shared state to the live console (cache name bumped to v2 so old shells are dropped).
