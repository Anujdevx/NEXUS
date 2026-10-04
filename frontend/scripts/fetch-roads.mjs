// Saves the real road network of Dehradun district from OpenStreetMap to public/roads.json.
// Run once, with internet:   npm run roads
// After that the app plans every route on real roads, with no network needed at demo time.
import { writeFileSync, mkdirSync } from "node:fs";

const BBOX = [30.05, 77.7, 30.56, 78.36]; // south, west, north, east
const QUERY = `[out:json][timeout:120];way["highway"~"^(motorway|trunk|primary|secondary|tertiary|unclassified)(_link)?$"](${BBOX.join(",")});out geom;`;
const HOSTS = ["https://overpass-api.de/api/interpreter", "https://overpass.kumi.systems/api/interpreter", "https://maps.mail.ru/osm/tools/overpass/api/interpreter"];
const r5 = (x) => Math.round(x * 1e5) / 1e5;

for (const host of HOSTS) {
  try {
    console.log("Asking", host, "...");
    const res = await fetch(host, { method: "POST", body: "data=" + encodeURIComponent(QUERY), headers: { "Content-Type": "application/x-www-form-urlencoded", "User-Agent": "nexus-major-project/1.0" } });
    if (!res.ok) { console.log("  refused:", res.status); continue; }
    const json = await res.json();
    const ways = json.elements.filter((e) => e.type === "way" && e.geometry && e.geometry.length > 1).map((e) => ({
      name: e.tags?.name || e.tags?.ref || "", hw: (e.tags?.highway || "unclassified").replace("_link", ""), ids: e.nodes, pts: e.geometry.map((g) => [r5(g.lat), r5(g.lon)])
    }));
    if (ways.length < 50) { console.log("  too few roads returned:", ways.length); continue; }
    mkdirSync("public", { recursive: true });
    writeFileSync("public/roads.json", JSON.stringify({ source: "OpenStreetMap contributors, ODbL", fetched: new Date().toISOString(), bbox: BBOX, ways }));
    console.log(`Saved public/roads.json: ${ways.length} roads, ${ways.reduce((a, w) => a + w.pts.length, 0)} points.`);
    process.exit(0);
  } catch (e) { console.log("  failed:", e.message); }
}
console.log("Could not reach Overpass. The app will use its built-in network. Try again later.");
process.exit(1);
