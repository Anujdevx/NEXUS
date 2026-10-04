# Nexus frontend

The Sūtradhāra control-room console for NEXUS, the coordination layer for multi-agency disaster response. Pilot district: Dehradun.

Built with React 18, Vite and MapLibre GL. No backend is needed to run it: state lives in the browser.

## Run it

You need Node.js 18 or newer.

```bash
npm install
npm run roads     # once, with internet: saves real OpenStreetMap roads to public/roads.json
npm run dev
```

Vite opens http://localhost:5173. To make a production build, run `npm run build`, then `npm run preview`.

### The road network

Every route is planned on one road network, chosen in this order:

1. `public/roads.json`, written by `npm run roads`. Real OpenStreetMap roads for Dehradun district. Do this before a demo so nothing depends on the venue's internet.
2. If that file is missing, the app asks the Overpass API for the same roads when it starts, and keeps a copy in the browser.
3. If both fail, it uses a built-in schematic network of the main roads, so the planner always works.

The Routes screen and Settings both say which one is in use and how many junctions and segments it has.

### The 3D map

The map is MapLibre GL. Base tiles come from CARTO, OpenTopoMap or Esri; the 3D terrain comes from the AWS open elevation tiles. All need internet. Offline, markers, roads and routes still draw on a plain tilted surface.

## What is real and what is simulated

Real, each with a named source (see the Sources screen):

- 23 hospitals: name, locality, phone, ownership
- 24 pharmacies: name, address, opening hours
- 34 flood-prone and waterlogging localities
- 11 landslide and subsidence points
- 15 infrastructure records, including the 21 buildings under Nagar Nigam notice
- replay incidents and road closures for 20 Jul 2026, 16 Sep 2025 and 20 Aug 2022
- rainfall, river status, flood zoning and seismic status

Simulated, and labelled "Simulated" on screen:

- free beds, inbound load, hospital capability (assumed from facility level)
- pharmacy stock of the 12 listed medicines (no pharmacy publishes live inventory)
- road breaks you report in a session
- ambulances and response teams
- shelter sites and occupancy
- citizen SOS requests raised in a session
- the built-in road network in `src/data/index.js` (`NODES`, `EDGES`): a schematic of main roads, used only when real roads cannot be loaded. Road speeds per class are assumed, and all roads are treated as two-way.
- in the Evaluation sweep: the SOS load, hospital capacity and connectivity. The road network it runs on is real when OpenStreetMap roads are loaded. These are simulation results, not field results.
- the flood spread: circles that grow around documented flood-prone localities, each at an arbitrary stage

Marker positions are approximate. Treat the map as schematic.

## Folder map

```
src/
  main.jsx              entry point, fonts, global CSS
  App.jsx               shell, hash routes (#/overview, #/routes ...), keyboard shortcuts
  styles/app.css        design tokens and component styles (matte base)
  styles/glass.css      film grain, liquid glass, motion, newer components
  data/index.js         every dataset, with source keys
  engine/
    roadnet.js          Mārga: the road network, A* shortest path, breaks, nearest lookups
    hazard.js           Pūrvasūchanā: rainfall what-if to segment passability
    simulation.js       connectivity sweep on the road network
  points.js             every place a route can start or end
  i18n.js               English and Hindi strings
  state/store.js        one store; planning, hospital and pharmacy ranking (rankOnNet, rankPharmacies); every action publishes a bus envelope and an audit entry; applyRemote() applies envelopes from the backend
  api/                  config.js (env), client.js (fetch wrapper, token, live flag), live.js (WebSocket with backoff)
  components/           Logo, Sidebar, Topbar, CommandPalette, Demo, Fx, MapView, Drawer, ui
  views/                one file per screen
public/sw.js            service worker: offline shell for the citizen app
public/manifest.webmanifest  install metadata
scripts/fetch-roads.mjs saves real OpenStreetMap roads (npm run roads)
tests/click-all.spec.js Playwright: clicks every control on every screen
```

## Screens

Overview, Incidents, Map, Routes (Mārga), Hospitals (Ārogya), Pharmacies, Citizen SOS (Āhvāna), Shelters (Āśraya), Responders (Rakṣaka), Early warning (Pūrvasūchanā), Infrastructure, Evaluation, Modules, Architecture, Situation report, Audit trail, Sources, Settings, and My assignment (Responder role).

## Offline citizen app

The service worker runs only in a production build, so it never interferes with the dev server.

```bash
npm run build
npm run preview        # http://localhost:4173
```

Open the Citizen SOS screen once, then switch off Wi-Fi. The page still opens, an SOS is held on the device (and survives a reload), and it is delivered when Wi-Fi returns. Settings and the SOS screen have an Install button; Chrome and Edge also offer "Install app" in the address bar. With no backend, delivery means handing the request to the in-browser bus; with the backend running it is posted to `/api/v1/sos`.

## Things to show in a demo

- **The race** (Routes): after planning a route, press Start the race. Two vehicles leave together; the baseline drives into the broken road or the full hospital, turns back, and the clock shows what that cost.
- **Flood that spreads** (bar at the bottom right of any map): press play or drag the stage. Water rises around the documented flood-prone localities, roads in it break, and the active route reroutes. This is illustrative, not a hydraulic model.
- **Routes**: choose a start (A) and a destination (B), from the lists or by clicking the map. The shortest route appears. Press **Break** beside any step, or the break tool on the map, and the planner finds the next shortest way round. The old way stays dashed and the difference is stated. **Repair** restores it.
- **Destination types**: a place, the best hospital for a clinical need, the nearest pharmacy that has a medicine in stock, or the nearest open shelter.
- **Pharmacies**: pick a medicine, see stock across 24 real shops, and find the nearest one that has it by drive time.
- **3D terrain**: the mountain button on the map. Terrain, Satellite and Matte base maps.
- **Run the 3-minute demo** (Overview or Settings): one SOS through relay, allocation, routing and approval, with captions.
- **Ctrl or Cmd + K**: jump to any screen, hospital, pharmacy, locality or incident, or run an action such as "Cut the network" or "Break a road".
- **Replay bar** (top): pick a documented event, press play, and incidents appear in report order. The minute marks are illustrative.
- **Towers up / down** (top): cut the network, send an SOS (hold the button for one second), then let a peer find an uplink or use the SMS fallback.
- **Rainfall what-if** (Early warning): drag the slider and watch Routes change.
- **Show reach** (any hospital drawer): roads coloured by drive time under the current breaks.
- **Role switch** (top right): Controller, Responder (assignment view), Citizen (SOS only).
- **Situation report**: print or save as PDF.
- **Hindi** (the हि button, top right): navigation, the citizen SOS screen and public alerts.
- **Evaluation**: the connectivity sweep now runs on the loaded road network.
- Press **?** for all shortcuts.

## Tests

```bash
npm install && npx playwright install chromium                   # once (@playwright/test is a devDependency)
npm run build && npm run preview                                # terminal 1
npx playwright test                                             # terminal 2   (NEXUS_URL=http://localhost:4173/ by default)

# the same suite against the live backend (start-all.sh first) and with it down:
NEXUS_GATEWAY=http://localhost:8000 npm run preview             # live
NEXUS_GATEWAY=http://localhost:9    npm run preview             # backend down
```

## Running with the backend

The console always runs on its own: the road network, hazard inference, the simulation and the hospital ranking all live in the
browser. When the NEXUS services are up it also talks to them, and falls back to its local engine, silently, the moment they are not.
The Topbar chip says which: **Live** (connected to the Sūtra bus) or **Local**.

```bash
../scripts/start-all.sh       # from the repo root: gateway, databases, ten services, then this app
```

or, with the backend already running, `npm run dev`. The dev and preview servers proxy `/api`, `/svc` and `/ws` to the gateway
(`NEXUS_GATEWAY`, default `http://localhost:8000`; `start-all.sh` sets it when it had to pick another port), so there is no CORS.

What goes through the backend when live:

- **SOS**: `POST /api/v1/sos`. The backend ranks hospitals, assigns the ambulance and publishes `assignment.dispatched`; the console applies it from the WebSocket.
  If no assignment arrives within five seconds, the local logic runs unchanged.
- **Every action** that publishes an envelope (breaking a road, hospital divert, bed counts, shelter changes, unit status, alerts) is also sent to `POST /api/v1/events` in the background.
- **Incoming envelopes** (`applyRemote` in `store.js`) update hospitals, units, shelters, stock, incidents, closures, breaks, the flood stage, alerts and the lifeline cascade.
- **Routes** are drawn by the local engine at once; when the backend graph is the same network it plans the route too and confirms it.
- **On connect**, shared state (hospitals, units, shelters, stock, closures, breaks, SOS incidents) is read from the backend. A role switch reconnects with a token for the new role.
- **Settings, Reset all state** resets the backend too.

Environment (all optional, see `.env.example`): `VITE_API_URL` (default `/api/v1`), `VITE_WS_URL` (default `/ws/v1/live`),
`VITE_LIVE` (`auto` | `on` | `off`; `off` never calls the backend).

To rehearse the backend-down case with the stack running, start the preview with a dead gateway:
`NEXUS_GATEWAY=http://localhost:9 npm run preview`.

## Look

Classically minimal: a matte, fine-grained leather surface, hairline edges, serif headings, one brass accent. Floating layers (top bar, map controls, drawers, command palette, the SOS phone, the KPI tiles) are liquid glass: a blurred, refracted backdrop with a highlight that follows the pointer. The refraction needs Chrome or Edge; Safari and Firefox get frosted glass. Settings can switch glass and motion off for a slow machine. The mark in `src/components/Logo.jsx` is a serif N in a hairline ring. Tokens are at the top of `src/styles/leather.css`.
