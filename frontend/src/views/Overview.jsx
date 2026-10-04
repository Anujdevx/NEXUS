import { useStore, actions } from "../state/store.js";
import { DISTRICTS, SCENARIOS, FLOOD, SLIDES, INFRA, HOSPITALS, ZONING, RIVERWATCH } from "../data/index.js";
import { Panel, Sourced, Simulated, Srcs } from "../components/ui.jsx";
import MapView from "../components/MapView.jsx";
import { CountUp } from "../components/Fx.jsx";
import { startDemo } from "../components/Demo.jsx";

export default function Overview() {
  const s = useStore();
  const sc = SCENARIOS[s.scenario];
  const open = s.incidents.filter((i) => i.status !== "Closed");
  const closed = Object.values(s.closures).filter(Boolean).length;
  const amb = s.units.filter((u) => u.type === "Ambulance");
  const places = s.shelters.filter((x) => x.open).reduce((a, x) => a + (x.cap - x.occ), 0);
  const note = s.district && DISTRICTS.find((d) => d[0] === s.district);
  const register = [
    ["Flood-prone localities", FLOOD.length, () => { actions.setLens("Flood"); actions.go("map"); }],
    ["Landslide points", SLIDES.length, () => { actions.setLens("Landslide"); actions.go("map"); }],
    ["Roads and crossings on the register", INFRA.filter((i) => i.kind !== "Building").length, () => actions.go("infrastructure")],
    ["Unsafe buildings under notice", 21, () => actions.select("infra", "i_bld")],
    ["Hospitals mapped", HOSPITALS.length, () => actions.go("hospitals")]
  ];
  return (
    <>
      <div className="page-head hero">
        <div>
          <h1>Dehradun</h1>
          <p>Replaying {sc.label}. {sc.sum}</p>
          <div className="rowx" style={{ marginTop: 12 }}><button className="btn primary" onClick={startDemo}>Run the 3-minute demo</button><button className="btn" onClick={() => actions.go("sitrep")}>Situation report</button></div>
        </div>
        <div className="districts" role="group" aria-label="Districts">
          {DISTRICTS.map(([name, haz]) => (
            <button key={name} className={(s.district || "Dehradun") === name ? "on" : ""} onClick={() => actions.pickDistrict(name)}>{name}<small>{haz}</small></button>
          ))}
        </div>
      </div>
      {note && (
        <div className="note" style={{ marginBottom: 16 }}>
          <b>{note[0]}</b> has no verified dataset loaded yet. Hazard profile: {note[1].toLowerCase()}. {note[2]} The map below stays on the pilot district.{" "}
          <button className="link" onClick={() => actions.pickDistrict("Dehradun")}>Back to Dehradun</button>
        </div>
      )}
      <div className="ov">
        <div className="stack ov-left">
          <Panel title="Hazard register" right={<Sourced />} flush>
            <div className="rows">
              {register.map(([label, n, fn]) => (
                <button key={label} onClick={fn}><span className="grow">{label}</span><span className="num">{n}</span></button>
              ))}
            </div>
          </Panel>
          <Panel title="Flood hazard zoning, city" right={<Sourced />}>
            <p className="dim">Share of the city’s area in each class. Almost 59% is high or very high.</p>
            <div className="zoning">{ZONING.rows.map(([l, v, c]) => <i key={l} className={"c-" + c} style={{ width: v + "%" }} title={`${l}: ${v}%`} />)}</div>
            <div className="legend">{ZONING.rows.map(([l, v, c]) => <span key={l}><i className={"c-" + c} />{l} {v}%</span>)}</div>
            <Srcs ids={[ZONING.s]} />
          </Panel>
        </div>
        <div style={{ minHeight: 540 }}><MapView panelOpen={false} /></div>
        <div className="stack">
          <Panel title="Active incidents" right={<button className="link" onClick={() => actions.go("incidents")}>View all {open.length}</button>} flush>
            {open.length ? (
              <div className="rows">
                {open.slice(0, 5).map((i) => (
                  <button key={i.id} onClick={() => actions.select("incident", i.id)}>
                    <span className="grow"><b>{i.n}</b><small>{i.place}</small></span>
                    <span className={"sev " + i.sev}>{i.sev}</span>
                  </button>
                ))}
              </div>
            ) : <div className="empty">No open incidents. Load a replay from the top bar, or raise a test SOS.</div>}
          </Panel>
          <Panel title="River watch" right={<button className="link" onClick={() => actions.go("warning")}>Early warning</button>} flush>
            <div className="rows">
              {RIVERWATCH.slice(0, 4).map((r, k) => (
                <div key={k}><span className="grow"><b>{r.r}, {r.at}</b><small className={"lv-" + r.lv}>{r.st}</small></span><span className="muted">{r.d}</span></div>
              ))}
            </div>
          </Panel>
        </div>
      </div>
      <div className="kpis">
        <button className="panel kpi" onClick={() => actions.go("incidents")}><b className="num"><CountUp value={open.length} /></b><span>Open incidents</span><small>{open.filter((i) => i.sev === "High").length} high severity</small></button>
        <button className="panel kpi" onClick={() => actions.go("routes")}><b className="num"><CountUp value={closed} /></b><span>Segments closed or slow</span><small>Documented closures in this replay</small></button>
        <button className="panel kpi" onClick={() => actions.go("responders")}><b className="num"><CountUp value={amb.filter((u) => u.status === "Available").length} /><span className="muted" style={{ display: "inline", font: "inherit" }}> / {amb.length}</span></b><span>Ambulances available</span><small><Simulated /></small></button>
        <button className="panel kpi" onClick={() => actions.go("hospitals")}><b className="num"><CountUp value={HOSPITALS.length} /></b><span>Hospitals in network</span><small>{HOSPITALS.filter((h) => s.hosp[h.id].divert || s.hosp[h.id].free === 0).length} not accepting</small></button>
        <button className="panel kpi" onClick={() => actions.go("shelters")}><b className="num"><CountUp value={places} /></b><span>Shelter places free</span><small><Simulated /></small></button>
      </div>
      <div className="ticker" aria-live="off"><b>Sūtra bus</b>{s.bus.length ? <span key={s.bus[0]._id}>{s.bus[0].source} published {s.bus[0].entity}.{s.bus[0].type}{s.bus[0].status ? `, ${s.bus[0].status}` : ""}. {s.bus.length} envelopes this session.</span> : <span>Nothing published yet. Dispatch a unit, close a road or raise an SOS and each envelope shows here.</span>}</div>
    </>
  );
}
