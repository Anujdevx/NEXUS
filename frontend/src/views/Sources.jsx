import { SRC, HOSPITALS, FLOOD, SLIDES, INFRA } from "../data/index.js";
import { Panel, Sourced, Simulated } from "../components/ui.jsx";

export default function Sources() {
  const list = Object.entries(SRC);
  return (
    <>
      <div className="page-head"><div><h1>Sources</h1><p>Where each record comes from, and what is simulated. Live EMRI, hospital and municipal feeds are not public, so the console says plainly which is which.</p></div></div>
      <div className="grid2">
        <Panel title="Real, with a named source" right={<Sourced />}>
          <ul style={{ margin: 0, paddingLeft: 18, color: "var(--tx2)", display: "grid", gap: 6 }}>
            <li>{HOSPITALS.length} hospitals: name, locality, phone, ownership.</li>
            <li>{FLOOD.length} flood-prone and waterlogging localities.</li>
            <li>{SLIDES.length} landslide and subsidence points.</li>
            <li>{INFRA.length} infrastructure records, including the 21 buildings under notice.</li>
            <li>Replay incidents and road closures for 20 Jul 2026, 16 Sep 2025 and 20 Aug 2022.</li>
            <li>Rainfall, river status, flood zoning and seismic status.</li>
          </ul>
        </Panel>
        <Panel title="Simulated, and labelled so" right={<Simulated />}>
          <ul style={{ margin: 0, paddingLeft: 18, color: "var(--tx2)", display: "grid", gap: 6 }}>
            <li>Free beds, inbound load and hospital capability (assumed from facility level).</li>
            <li>Ambulances and response teams, and where they stand.</li>
            <li>Shelter sites and occupancy.</li>
            <li>Citizen SOS requests raised in a session.</li>
            <li>The road graph: a schematic of main roads with approximate positions, travel speeds and lengths. The build plan replaces it with the OpenStreetMap graph.</li>
            <li>The connectivity sweep, which is a toy model.</li>
          </ul>
        </Panel>
      </div>
      <div style={{ height: 16 }} />
      <Panel title={`${list.length} references`} flush>
        <div className="tscroll"><table>
          <thead><tr><th>Publisher</th><th>Date</th><th>Title</th></tr></thead>
          <tbody>{list.map(([k, s]) => <tr key={k}><td style={{ whiteSpace: "nowrap" }}>{s.p}</td><td className="muted" style={{ whiteSpace: "nowrap" }}>{s.d}</td><td>{s.u ? <a className="link" style={{ fontSize: "inherit" }} href={s.u} target="_blank" rel="noreferrer">{s.t}</a> : <span className="dim">{s.t}</span>}</td></tr>)}</tbody>
        </table></div>
      </Panel>
    </>
  );
}
