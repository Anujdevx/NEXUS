import { X, MapPin, Send, Check, CircleOff, Target, Undo2 } from "lucide-react";
import { useStore, actions } from "../state/store.js";
import { HOSPITALS, FLOOD, SLIDES, INFRA, CLOSURES, CAPS, NODES, MODULES, PHARMACIES, MEDICINES } from "../data/index.js";
import { Srcs, Sourced, Simulated, Bar } from "./ui.jsx";

function Shell({ kicker, title, children, footer }) {
  return (
    <>
      <div className="scrim" onClick={actions.closeDrawer} />
      <aside className="drawer" role="dialog" aria-label={title}>
        <header>
          <div><div className="muted" style={{ marginBottom: 4 }}>{kicker}</div><h2>{title}</h2></div>
          <button className="icon-btn" aria-label="Close" onClick={actions.closeDrawer}><X /></button>
        </header>
        <div className="body">{children}</div>
        {footer && <footer>{footer}</footer>}
      </aside>
    </>
  );
}
const MapBtn = ({ lat, lng, z }) => lat == null ? null : <button className="btn" onClick={() => { actions.closeDrawer(); actions.showOnMap(lat, lng, z); }}><MapPin />Show on map</button>;

export default function Drawer() {
  const sel = useStore((s) => s.selected);
  const st = useStore();
  if (!sel) return null;
  const { kind, id } = sel;

  if (kind === "hospital") {
    const h = HOSPITALS.find((x) => x.id === id), x = st.hosp[id];
    return (
      <Shell kicker={`${h.own} hospital, ${h.lvl.toLowerCase()}`} title={h.n} footer={<><MapBtn lat={h.lat} lng={h.lng} z={15} /><button className="btn" onClick={() => actions.setReach(id)}><Target />Show reach</button><button className={"btn" + (x.divert ? "" : " danger")} onClick={() => actions.toggleDivert(id)}>{x.divert ? <><Check />Accept patients again</> : <><CircleOff />Set to divert</>}</button></>}>
        <dl className="kv">
          <dt>Address</dt><dd>{h.loc}</dd>
          <dt>Phone</dt><dd>{h.ph || <span className="muted">Not listed</span>}</dd>
          <dt>Bed strength</dt><dd>{h.beds ? <>{h.beds.toLocaleString("en-IN")} <Sourced /><div className="muted">{h.bnote}</div></> : <span className="muted">Not verified</span>}</dd>
          <dt>Can take</dt><dd>{CAPS[h.lvl].join(", ")}<div className="muted">Assumed from facility level</div></dd>
        </dl>
        <Srcs ids={["maps", h.bsrc, ["doon", "cor", "aiims", "smi", "max", "geh"].includes(id) ? "nic" : null].filter(Boolean)} />
        <div className="panel"><div className="body">
          <div className="rowx" style={{ justifyContent: "space-between", marginBottom: 8 }}><b>Emergency beds free</b><Simulated /></div>
          <div className="rowx"><button className="btn sm" aria-label="One fewer free bed" disabled={x.divert} onClick={() => actions.adjustBeds(id, -1)}>−</button><span className="num">{x.divert ? 0 : x.free}</span><span className="muted">of {x.cap}</span><button className="btn sm" aria-label="One more free bed" disabled={x.divert} onClick={() => actions.adjustBeds(id, 1)}>+</button><div style={{ flex: 1 }}><Bar value={x.cap - (x.divert ? 0 : x.free)} max={x.cap} /></div></div>
          <p className="muted" style={{ marginTop: 8 }}>{x.inbound} inbound. {x.divert ? "Diverting: the allocation engine skips this hospital." : "Changes here alter the next assignment."}</p>
        </div></div>
      </Shell>
    );
  }
  if (kind === "flood" || kind === "slide") {
    const f = (kind === "flood" ? FLOOD : SLIDES).find((x) => x.id === id);
    return (
      <Shell kicker={kind === "flood" ? `Flood-prone locality, ${f.r}` : `Landslide point${f.road ? ", " + f.road : ""}`} title={f.n} footer={<><MapBtn lat={f.lat} lng={f.lng} />{kind === "flood" && <button className="btn primary" onClick={() => { actions.closeDrawer(); actions.go("routes"); actions.computeRoute({ from: f.node, mode: "hospital", need: "Trauma" }); }}><Send />Route to care from here</button>}</>}>
        <p>{f.note}</p><div><Sourced /></div><Srcs ids={f.s} />
      </Shell>
    );
  }
  if (kind === "infra") {
    const f = INFRA.find((x) => x.id === id);
    return (
      <Shell kicker={`${f.kind}, ${f.when}`} title={f.n} footer={<MapBtn lat={f.lat} lng={f.lng} />}>
        <dl className="kv"><dt>Where</dt><dd>{f.where}</dd><dt>What happened</dt><dd>{f.what}</dd><dt>Status</dt><dd className={"lv-" + f.sv}>{f.status}</dd></dl>
        <div><Sourced /></div><Srcs ids={f.s} />
      </Shell>
    );
  }
  if (kind === "closure") {
    const c = CLOSURES[id], on = st.closures[id];
    return (
      <Shell kicker={c.kind === "closed" ? "Road closure" : "Slow segment"} title={c.n} footer={<><MapBtn lat={c.at[0]} lng={c.at[1]} /><button className="btn" onClick={() => actions.toggleClosure(id)}>{on ? "Mark passable" : "Mark impassable"}</button></>}>
        <p>{c.note}</p>
        <p className="dim">{on ? "In force in this replay. The router avoids it in proportion to report confidence." : "Not in force in this replay."}</p>
        <div><Sourced /></div><Srcs ids={[c.s]} />
      </Shell>
    );
  }
  if (kind === "incident") {
    const i = st.incidents.find((x) => x.id === id);
    if (!i) return null;
    return (
      <Shell kicker={`${i.t}, ${i.when}`} title={i.n}
        footer={<><MapBtn lat={i.lat} lng={i.lng} />
          {i.status === "Open" && <button className="btn" onClick={() => actions.ackIncident(id)}><Check />Acknowledge</button>}
          {!i.unit && i.status !== "Closed" && <button className="btn primary" onClick={() => actions.dispatchIncident(id)}><Send />Dispatch nearest unit</button>}
          {i.unit && i.status !== "Closed" && <button className="btn" onClick={() => actions.reassign(id)}><Undo2 />Reassign</button>}
          {i.status !== "Closed" && <button className="btn danger" onClick={() => actions.closeIncident(id)}>Close incident</button>}</>}>
        <div className="rowx"><span className={"sev " + i.sev}>{i.sev}</span><span className="tag">{i.status}</span>{i.sim ? <Simulated /> : <Sourced />}</div>
        <dl className="kv"><dt>Place</dt><dd>{i.place}</dd><dt>Detail</dt><dd>{i.d}</dd>{i.unit && <><dt>Unit</dt><dd>{i.unit} <Simulated /></dd></>}</dl>
        <Srcs ids={i.s} />
        {i.log && i.log.length > 0 && <ol className="flow" style={{ marginTop: 4 }}>{i.log.map(([what, when], k) => <li key={k} className="done"><div><b>{what}</b><small>{when}</small></div></li>)}</ol>}
      </Shell>
    );
  }
  if (kind === "shelter") {
    const x = st.shelters.find((s) => s.id === id);
    return (
      <Shell kicker="Shelter site" title={x.n} footer={<><MapBtn lat={x.lat} lng={x.lng} /><button className="btn" onClick={() => actions.toggleShelter(id)}>{x.open ? "Close site" : "Open site"}</button></>}>
        <div className="rowx"><Simulated /><span className="tag">{!x.open ? "Closed" : x.occ >= x.cap ? "Full" : "Open"}</span></div>
        <div className="rowx"><button className="btn sm" onClick={() => actions.adjustShelter(id, -20)}>−20</button><span className="num">{x.occ}</span><span className="muted">of {x.cap} places</span><button className="btn sm" onClick={() => actions.adjustShelter(id, 20)}>+20</button></div>
        <Bar value={x.occ} max={x.cap} />
        {x.doc && <><p className="dim">{x.doc}</p><Srcs ids={[x.s]} /></>}
        <p className="muted">No verified public shelter list exists for the district, so sites and occupancy are simulated.</p>
      </Shell>
    );
  }
  if (kind === "unit") {
    const u = st.units.find((x) => x.id === id);
    return (
      <Shell kicker={u.type} title={u.id} footer={<button className="btn" onClick={() => actions.setUnit(id, u.status === "Available" ? "On mission" : "Available")}>{u.status === "Available" ? "Send on mission" : "Mark available"}</button>}>
        <div className="rowx"><Simulated /><span className="tag">{u.status}</span></div>
        <dl className="kv"><dt>Base</dt><dd>{NODES[u.node][2]}</dd><dt>Task</dt><dd>{u.task || <span className="muted">None</span>}</dd></dl>
        <p className="muted">Live fleet positions are not public. Units are simulated.</p>
      </Shell>
    );
  }
  if (kind === "pharmacy") {
    const p = PHARMACIES.find((x) => x.id === id), stock = st.stock[id];
    const pt = { key: "pharm:" + id, name: p.n, lat: p.lat, lng: p.lng };
    return (
      <Shell kicker="Pharmacy" title={p.n} footer={<><MapBtn lat={p.lat} lng={p.lng} z={16} /><button className="btn primary" onClick={() => { actions.closeDrawer(); actions.go("routes"); actions.setPlanner({ to: pt, mode: "place" }); }}><Send />Route here</button></>}>
        <dl className="kv"><dt>Address</dt><dd>{p.loc}</dd><dt>Hours</dt><dd>{p.hrs}</dd></dl>
        <Srcs ids={["maps"]} />
        <div className="rowx" style={{ justifyContent: "space-between" }}><b style={{ fontWeight: 500 }}>Stock</b><Simulated label="Stock simulated" /></div>
        <div className="rows" style={{ margin: "0 -6px" }}>
          {MEDICINES.map((m) => (
            <div key={m.id}><span className="grow"><b>{m.n}</b><small>{m.use}</small></span>
              <span className={stock[m.id] ? "" : "lv-bad"} style={{ minWidth: 86, textAlign: "right" }}>{stock[m.id] ? `${stock[m.id]} ${m.unit}` : "Out of stock"}</span>
              <button className="btn sm" aria-label={`Remove 5 ${m.n}`} onClick={() => actions.adjustStock(id, m.id, -5)}>−</button><button className="btn sm" aria-label={`Add 5 ${m.n}`} onClick={() => actions.adjustStock(id, m.id, 5)}>+</button>
            </div>
          ))}
        </div>
      </Shell>
    );
  }
  if (kind === "module") {
    const m = MODULES.find((x) => x.k === id);
    return (
      <Shell kicker={`${m.en}. Tier 3, schema-defined`} title={<>{m.n} <span lang="sa" style={{ font: "400 18px var(--deva)", color: "var(--tx3)" }}>{m.dv}</span></>}>
        <p>{m.d}</p>
        <p className="dim">Defined in the model with a stubbed record and labelled as future integration. A new domain costs a config entry, not a subsystem.</p>
        <pre className="json">{JSON.stringify({ ...m.stub, timestamp: new Date().toISOString() }, null, 2)}</pre>
        <Srcs ids={["synopsis"]} />
      </Shell>
    );
  }
  return null;
}
