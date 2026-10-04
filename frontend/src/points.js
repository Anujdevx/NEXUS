/* Every place a route can start or end, in one list. */
import { HOSPITALS, PHARMACIES, FLOOD, NODES } from "./data/index.js";

export function allPoints(state) {
  return [
    ...FLOOD.map((f) => ({ key: "flood:" + f.id, group: "Flood-prone localities", name: f.n, lat: f.lat, lng: f.lng })),
    ...HOSPITALS.map((h) => ({ key: "hosp:" + h.id, group: "Hospitals", name: h.n, lat: h.lat, lng: h.lng })),
    ...PHARMACIES.map((p) => ({ key: "pharm:" + p.id, group: "Pharmacies", name: p.n, lat: p.lat, lng: p.lng })),
    ...state.shelters.map((x) => ({ key: "shel:" + x.id, group: "Shelters", name: x.n, lat: x.lat, lng: x.lng })),
    ...state.incidents.map((i) => ({ key: "inc:" + i.id, group: "Incidents", name: i.n, lat: i.lat, lng: i.lng })),
    ...Object.entries(NODES).map(([k, n]) => ({ key: "node:" + k, group: "Junctions and towns", name: n[2], lat: n[0], lng: n[1] }))
  ];
}
