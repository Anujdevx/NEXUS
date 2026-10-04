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
- 24 pharmacies: name, address, opening hours
- 34 flood-prone and waterlogging localities
- 11 landslide and subsidence points
- 15 infrastructure records, including the 21 buildings under Nagar Nigam notice
- replay incidents and road closures for 20 Jul 2026, 16 Sep 2025 and 20 Aug 2022
- rainfall, river status, flood zoning and seismic status

Simulated, and labelled "Simulated" on screen:

- free beds, inbound load, hospital capability (assumed from facility level)
- pharmacy stock of the 12 listed medicines (no pharmacy publishes live inventory)
- pharmacy stock for 12 response medicines
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
    routing.js          schematic graph used only by the evaluation sweep
    allocation.js       Ārogya: multi-constraint hospital assignment and the baseline
    hazard.js           Pūrvasūchanā: rainfall what-if to segment passability
    simulation.js       connectivity sweep on the road network
  points.js             every place a route can start or end
  i18n.js               English and Hindi strings
  state/store.js        one store; planning, hospital and pharmacy ranking; every action publishes a bus envelope and an audit entry
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

Open the Citizen SOS screen once, then switch off Wi-Fi. The page still opens, an SOS is held on the device (and survives a reload), and it is delivered when Wi-Fi returns. Settings and the SOS screen have an Install button; Chrome and Edge also offer "Install app" in the address bar. There is no server yet, so delivery means handing the request to the in-browser bus.

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
npm i -D @playwright/test && npx playwright install chromium   # once
npm run build && npm run preview                                # terminal 1
npx playwright test                                             # terminal 2
```

## Connecting the backend later

`src/state/store.js` is the only place that changes state. Each action already builds the common envelope (`entity, type, geo, status, capacity, confidence, timestamp, source`) in `publish()`. To go live:

1. In `publish()`, also emit the envelope on your Socket.IO connection.
2. Subscribe to the socket and call `set()` with incoming hospital, shelter, unit and incident updates.
3. Swap `planRoute` and `rankHospitals` calls for requests to your routing and allocation services, keeping the same return shapes.

## Look

Classically minimal: a matte, fine-grained leather surface, hairline edges, serif headings, one brass accent. Floating layers (top bar, map controls, drawers, command palette, the SOS phone, the KPI tiles) are liquid glass: a blurred, refracted backdrop with a highlight that follows the pointer. The refraction needs Chrome or Edge; Safari and Firefox get frosted glass. Settings can switch glass and motion off for a slow machine. The mark in `src/components/Logo.jsx` is a serif N in a hairline ring. Tokens are at the top of `src/styles/leather.css`.
