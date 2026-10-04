import { useRef, useState } from "react";
import { useStore, actions } from "../state/store.js";
import { MODULES } from "../data/index.js";
import { Panel, Srcs, Seg, fmtTime } from "../components/ui.jsx";

const TIERS = { 1: ["Tier 1", "Built in depth. These four carry the evaluation."], 2: ["Tier 2", "Live on the bus with a reduced state model."], 3: ["Tier 3", "Schema-defined, with stubbed records."] };

export default function Modules() {
  const live = useStore((s) => s.bus);
  const [paused, setPaused] = useState(false);
  const [src, setSrc] = useState("All");
  const frozen = useRef(live);
  if (!paused) frozen.current = live;
  const sources = ["All", ...new Set(live.map((e) => e.source.split("/")[0]))].slice(0, 8);
  const bus = frozen.current.filter((e) => src === "All" || e.source.startsWith(src));
  return (
    <>
      <div className="page-head"><div><h1>Modules</h1><p>Ten domains, each named for what it does. Every one publishes the same JSON envelope to the Sūtra bus, so adding an eleventh is a config entry.</p></div></div>
      {[1, 2, 3].map((t) => (
        <div key={t}>
          <h2 className="tier-h" style={t === 1 ? { marginTop: 0 } : null}>{TIERS[t][0]}<small>{TIERS[t][1]}</small></h2>
          <div className="gridm">
            {MODULES.filter((m) => m.tier === t).map((m) => (
              <button key={m.k} className="panel mod" onClick={() => (m.view ? actions.go(m.view) : actions.select("module", m.k))}>
                <h3>{m.n}<span lang="sa">{m.dv}</span></h3><em>{m.g}</em><p><b style={{ fontWeight: 500, color: "var(--tx)" }}>{m.en}.</b> {m.d}</p>
                <span className="link" style={{ display: "block", marginTop: 10 }}>{m.view ? "Open" : "View schema"}</span>
              </button>
            ))}
          </div>
        </div>
      ))}
      <div style={{ height: 22 }} />
      <Panel title="Sūtra bus, this session" right={<div className="rowx"><Seg label="Source domain" options={sources} value={src} onChange={setSrc} /><button className="btn sm" onClick={() => setPaused((v) => !v)}>{paused ? "Resume feed" : "Pause feed"}</button><span className="muted">{bus.length} envelopes</span></div>} flush>
        {bus.length ? (
          <div className="tscroll"><table>
            <thead><tr><th>Time</th><th>Entity</th><th>Type</th><th>Status</th><th>Source</th><th>Confidence</th></tr></thead>
            <tbody>{bus.slice(0, 14).map((e) => <tr key={e._id}><td className="muted">{fmtTime(e.timestamp)}</td><td>{e.entity}</td><td className="dim">{e.type}</td><td className="dim">{e.status || "—"}</td><td className="dim">{e.source}</td><td className="dim">{e.confidence}</td></tr>)}</tbody>
          </table></div>
        ) : <div className="empty">Nothing published yet. Dispatch a unit, toggle a closure or raise an SOS and the envelopes appear here.</div>}
      </Panel>
      <Srcs ids={["synopsis"]} />
    </>
  );
}
