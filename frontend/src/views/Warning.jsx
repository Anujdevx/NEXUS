import { useState } from "react";
import { useStore, actions } from "../state/store.js";
import { RAIN, ZONING, RIVERWATCH, FLOOD, CLOSURES } from "../data/index.js";
import { THRESH } from "../engine/hazard.js";
import { ALERT_TEXT } from "../i18n.js";
import { Panel, Seg, Srcs, Sourced, Simulated, fmtTime } from "../components/ui.jsx";

const AREAS = ["Rispana and Bindal riverbanks", "Sahastradhara and Maldevta", "Tons bank, Prem Nagar to Tapkeshwar", "Mussoorie road", "Rishikesh ghats", "Whole district"];

export default function Warning() {
  const alerts = useStore((s) => s.alerts);
  const rain = useStore((s) => s.rain);
  const rainOn = useStore((s) => s.rainOn);
  const [area, setArea] = useState(AREAS[0]);
  const [severity, setSeverity] = useState("Severe");
  const appLang = useStore((s) => s.lang);
  const [msgLang, setMsgLang] = useState(appLang === "hi" ? "हिंदी" : "English");
  const [message, setMessage] = useState(ALERT_TEXT[appLang === "hi" ? "hi" : "en"]);
  const max = Math.max(...RAIN.rows.map((r) => r[1]));
  const cap = { identifier: "NEXUS-DDN-preview", sender: "sutradhara@nexus", msgType: "Alert", scope: "Public", info: { language: msgLang === "हिंदी" ? "hi-IN" : "en-IN", event: "Flood", severity, urgency: "Immediate", areaDesc: area, description: message } };
  return (
    <>
      <div className="page-head"><div><h1>Early warning<span lang="sa">पूर्वसूचना</span></h1><p>Reference readings from documented events. A live build would subscribe to IMD rainfall, CWC gauges and SACHET alerts on the same bus.</p></div></div>
      <div className="grid3">
        <Panel title={RAIN.label} right={<Sourced />}>
          {RAIN.rows.map(([n, v]) => <div className="hbar" key={n}><span className="dim">{n}</span><i style={{ width: (v / max) * 100 + "%" }} /><span>{v}</span></div>)}
          <Srcs ids={[RAIN.s]} />
        </Panel>
        <Panel title="River watch" right={<Sourced />} flush>
          <div className="rows">{RIVERWATCH.map((r, k) => <div key={k}><span className="grow"><b>{r.r}, {r.at}</b><small className={"lv-" + r.lv}>{r.st}</small></span><span className="muted">{r.d}</span></div>)}</div>
        </Panel>
        <div className="stack">
          <Panel title="Seismic status" right={<Sourced />}>
            <p>Dehradun is in <b>Zone IV</b> under IS 1893 (Part 1): 2016.</p>
            <p className="dim" style={{ marginTop: 6 }}>The 2025 revision would have placed it in a new Zone VI. BIS withdrew that revision by gazette notification on 3 March 2026.</p>
            <Srcs ids={["bis2026"]} />
          </Panel>
          <Panel title={ZONING.label} right={<Sourced />}>
            <div className="zoning">{ZONING.rows.map(([l, v, c]) => <i key={l} className={"c-" + c} style={{ width: v + "%" }} />)}</div>
            <div className="legend">{ZONING.rows.map(([l, v, c]) => <span key={l}><i className={"c-" + c} />{l} {v}%</span>)}</div>
            <p className="muted" style={{ marginTop: 8 }}>{FLOOD.length} flood-prone localities are on the map.</p>
            <Srcs ids={[ZONING.s]} />
          </Panel>
        </div>
      </div>
      <div style={{ height: 16 }} />
      <Panel title="Rainfall what-if" right={<Simulated label="Illustrative thresholds" />}>
        <div className="grid2">
          <div className="stack" style={{ gap: 10 }}>
            <p className="dim">Passability is inferred, not observed. Drag the cumulative rainfall and the planner treats documented weak segments as affected: waterlogging stretches from {THRESH.waterlog} mm, hill-road slide points from {THRESH.hill} mm. These thresholds are illustrative, not published Garhwal values.</p>
            <label className="field"><span>Cumulative rainfall: {rain} mm</span><input type="range" min="0" max="250" step="10" value={rain} aria-label="Cumulative rainfall in millimetres" onChange={(e) => actions.setRain(+e.target.value)} onPointerUp={actions.commitRain} onKeyUp={actions.commitRain} /></label>
            <div className="rowx"><button className="btn" onClick={() => actions.go("routes")}>See the effect on routes</button><button className="btn" disabled={!rain} onClick={() => { actions.setRain(0); actions.commitRain(); }}>Reset to 0 mm</button></div>
          </div>
          <div>
            <b style={{ fontWeight: 500 }}>{rainOn.length} segment{rainOn.length === 1 ? "" : "s"} inferred as affected</b>
            {rainOn.length ? <div className="rows" style={{ marginTop: 6 }}>{rainOn.map((c) => <div key={c}><span className="grow"><b>{CLOSURES[c].n}</b><small>{CLOSURES[c].kind === "closed" ? "Treated as closed" : "Treated as slow"}</small></span></div>)}</div> : <p className="muted" style={{ marginTop: 6 }}>None yet. Segments already closed in this replay are not counted again.</p>}
          </div>
        </div>
      </Panel>
      <div className="grid2" style={{ marginTop: 16 }}>
        <Panel title="Issue a public alert">
          <div className="stack" style={{ gap: 12 }}>
            <label className="field"><span>Area</span><select value={area} onChange={(e) => setArea(e.target.value)}>{AREAS.map((a) => <option key={a}>{a}</option>)}</select></label>
            <div className="field"><span>Severity</span><Seg label="Severity" options={["Moderate", "Severe", "Extreme"]} value={severity} onChange={setSeverity} /></div>
            <div className="field"><span>Message language</span><Seg label="Message language" options={["English", "हिंदी"]} value={msgLang} onChange={(v) => { setMsgLang(v); setMessage(ALERT_TEXT[v === "हिंदी" ? "hi" : "en"]); }} /></div>
            <label className="field"><span>Message</span><textarea value={message} onChange={(e) => setMessage(e.target.value)} /></label>
            <div className="rowx"><button className="btn primary" disabled={!message.trim()} onClick={() => actions.issueAlert({ area, severity, message })}>Issue alert</button><span className="muted">Stays inside this demo. Nothing is sent to the public.</span></div>
          </div>
        </Panel>
        <div className="stack">
          <Panel title="Alert envelope, CAP 1.2 shape"><pre className="json">{JSON.stringify(cap, null, 2)}</pre></Panel>
          <Panel title="Issued this session" flush>
            {alerts.length ? <div className="rows">{alerts.map((a) => <div key={a.id}><span className="grow"><b>{a.severity}: {a.area}</b><small>{a.message}</small></span><span className="muted">{fmtTime(a.at)}</span></div>)}</div> : <div className="empty">No alerts issued yet. Fill the form and press Issue alert.</div>}
          </Panel>
        </div>
      </div>
    </>
  );
}
