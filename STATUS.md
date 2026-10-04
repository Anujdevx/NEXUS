# STATUS

Written at the end of the build session (Sun 4 Oct 2026). Demo: tomorrow.

## Done and verified

| Area | State |
|---|---|
| Frontend in `frontend/` | The partner's console, moved in unchanged (a recursive diff against the original was empty); builds; screenshots of Overview, Routes, Citizen SOS and Infrastructure match the original. The only visible additions: the **Live / Local** chip, and a "Lifeline cascade" panel on Infrastructure that appears only while a cascade exists |
| Shared Go module | envelope, bus (topic exchange, DLX, confirms, reconnect), httpx, auth (JWT roles), pg (+migrations), seed (+ports), ratelimit |
| Ten Go services | All build, pass `go vet`, `go test`, serve `/healthz` and `/readyz` through the gateway, and run in containers |
| Gateway and scripts | `scripts/start-all.sh` (health-check waits, free-port fallback), `stop-all.sh`, `seed.sh`, `smoke.sh` |
| SOS vertical slice | Citizen SOS to assignment in well under a second (measured 69 ms in a browser), incident in the Controller view, full event chain in `GET /api/v1/events` |
| Routing | The Go A\* is an exact port of `roadnet.js`: eleven fixed route pairs, lookups and a distance tree match the JS engine in a table test. Runs on the real OSM network (167,500 junctions), about 30 ms per route |
| Evaluation sweep | Go port of `simulation.js`, tested against the JS output |
| Prāṇadhārā cascade | 3 substations, 4 pumps, 3 towers (simulated) with hospitals and shelters depending on them; the cascade, grace-period trigger, blast radius, hospital divert and the Infrastructure panel work live |
| Simulator | `telemetry`, `sos-burst`, `chaos`, `monsoon`, all verified against the running stack |
| Fallback | With the backend down the console runs on its local engine; the chip reads **Local** |

Automated checks: see "Test results" below.

## Partial (works, deliberately thin; see each service's `TODO.md`)

- Abhayasūchī and Sambharaṇa are stubs (`/api/v1/dispatch/registry`, `/logistics`).
- SMS fallback is a stub. Nothing is sent to anyone.
- media-storage works (MinIO) but the console has no attachment UI.
- `topology/upload` reads GeoJSON Point features only.
- iot-broker keeps the latest telemetry in memory (Redis unused there).
- The console renders routes locally and the backend confirms them when the networks match (docs/decisions.md, item 8); SOS hospital and ambulance choice are made by the backend when live.

## Not done

- No CI. No load testing. No handler-level (HTTP) tests; the tests are on the domain logic, the ports and the smoke script.

## How to run it

```bash
./scripts/start-all.sh           # first run builds ten images: allow several minutes
./scripts/smoke.sh --e2e         # must be green
python3 tools/simulator/main.py chaos --asset sub-01
./scripts/stop-all.sh
```

**Do the first run before the demo, on the demo machine, with internet.** Docker Desktop must be running. If another program
holds port 8000 (another project's container did on the development machine), the script uses the next free port and prints it.
`scripts/seed.sh --reset` (or Settings, Reset all state) puts the whole system back to the opening scenario between rehearsals.

## Things to know before the demo

- The console takes shared state from the backend when it connects, so rehearsal leftovers (assigned ambulances, hospital divert,
  closures) reappear after a page reload until you reset. Reset between rehearsals.
- With the real road network the first page load builds a 170,000-junction graph (a few seconds); the Routes screen says which network is in use.
- The map's base tiles need internet (CARTO, AWS terrain). The "API KEY REQUIRED" watermark is the tile provider's, unchanged from the original.
- Real versus simulated is spelled out in README.md. Everything in the cascade is simulated and labelled so.
