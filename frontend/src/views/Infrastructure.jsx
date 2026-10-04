import { useState } from "react";
import { actions } from "../state/store.js";
import { INFRA, SLIDES } from "../data/index.js";
import { Panel, Seg, Sourced, Srcs } from "../components/ui.jsx";

export default function Infrastructure() {
  const [kind, setKind] = useState("All");
  const list = INFRA.filter((i) => kind === "All" || i.kind === kind);
  return (
    <>
      <div className="page-head">
        <div><h1>Infrastructure</h1><p>Roads, crossings and buildings that have failed or been flagged. “Not verified” means no source confirms the present state, so check before relying on it.</p></div>
        <Seg label="Kind" options={["All", "Bridge", "Road", "Building"]} value={kind} onChange={setKind} />
      </div>
      <Panel title={`${list.length} records`} right={<Sourced />} flush>
        <div className="tscroll"><table>
          <thead><tr><th>Asset</th><th>Kind</th><th>When</th><th>What happened</th><th>Status</th></tr></thead>
          <tbody>{list.map((f) => (
            <tr key={f.id} className="click" onClick={() => actions.select("infra", f.id)}>
              <td style={{ minWidth: 190 }}><b style={{ fontWeight: 500 }}>{f.n}</b><div className="muted" style={{ maxWidth: 260 }}>{f.where}</div></td>
              <td className="dim">{f.kind}</td><td className="dim" style={{ whiteSpace: "nowrap" }}>{f.when}</td><td className="dim" style={{ maxWidth: 420 }}>{f.what}</td><td className={"lv-" + f.sv} style={{ minWidth: 150 }}>{f.status}</td>
            </tr>))}</tbody>
        </table></div>
      </Panel>
      <div style={{ height: 16 }} />
      <Panel title="Landslide and subsidence points" right={<Sourced />} flush>
        <div className="rows">{SLIDES.map((f) => (
          <button key={f.id} onClick={() => actions.select("slide", f.id)}><span className="grow"><b>{f.n}</b><small>{f.road ? f.road + ". " : ""}{f.note}</small></span></button>
        ))}</div>
      </Panel>
      <Srcs ids={["dh260907", "navjivan2022"]} />
    </>
  );
}
