import { useStore, actions } from "../state/store.js";
import { Panel, Seg } from "../components/ui.jsx";
import { startDemo } from "../components/Demo.jsx";

const cap = (s) => s[0].toUpperCase() + s.slice(1);

export default function Settings() {
  const settings = useStore((s) => s.settings);
  const theme = useStore((s) => s.theme);
  const role = useStore((s) => s.role);
  const netInfo = useStore((s) => s.netInfo);
  const three = useStore((s) => s.three);
  const lang = useStore((s) => s.lang);
  const installable = useStore((s) => s.installable);
  const Row = ({ title, hint, children }) => <div className="setrow"><div><b style={{ fontWeight: 500 }}>{title}</b><small>{hint}</small></div>{children}</div>;
  return (
    <>
      <div className="page-head"><div><h1>Settings</h1><p>These stay on this device. They change how the console looks, not what it knows.</p></div></div>
      <div className="stack" style={{ maxWidth: 760 }}>
        <Panel title="Appearance" flush>
          <Row title="Language" hint="Hindi covers the navigation, the citizen SOS screen and public alerts. The control-room screens stay in English."><Seg label="Language" options={["English", "हिंदी"]} value={lang === "hi" ? "हिंदी" : "English"} onChange={(v) => actions.setLang(v === "हिंदी" ? "hi" : "en")} /></Row>
          <Row title="Theme" hint="Dark matte is the default."><Seg label="Theme" options={["Dark", "Light"]} value={cap(theme)} onChange={(v) => actions.setTheme(v.toLowerCase())} /></Row>
          <Row title="Glass" hint="Liquid bends what is behind the floating layers and needs Chrome or Edge. Frosted only blurs. Off uses solid panels."><Seg label="Glass" options={["Liquid", "Frosted", "Off"]} value={cap(settings.glass)} onChange={(v) => actions.setSetting("glass", v.toLowerCase())} /></Row>
          <Row title="Motion" hint="Reduced keeps changes instant. Off removes every animation."><Seg label="Motion" options={["Full", "Reduced", "Off"]} value={cap(settings.motion)} onChange={(v) => actions.setSetting("motion", v.toLowerCase())} /></Row>
          <Row title="Density" hint="Compact fits more rows on a projector."><Seg label="Density" options={["Comfortable", "Compact"]} value={cap(settings.density)} onChange={(v) => actions.setSetting("density", v.toLowerCase())} /></Row>
        </Panel>
        <Panel title="Road network" flush>
          <Row title={netInfo.label} hint={`${netInfo.nodes.toLocaleString("en-IN")} junctions and ${netInfo.edges.toLocaleString("en-IN")} road segments. Run "npm run roads" once to save real OpenStreetMap roads for offline use.`}><button className="btn" onClick={actions.reloadNet}>Reload road network</button></Row>
          <Row title="3D terrain" hint="Tilts the map and raises the hills from open elevation data."><Seg label="Terrain" options={["3D", "Flat"]} value={three ? "3D" : "Flat"} onChange={() => actions.toggle3d()} /></Row>
        </Panel>
        <Panel title="Session" flush>
          {role === "Controller" && <Row title="Guided demo" hint="Plays one SOS through relay, allocation, routing and approval, with captions."><button className="btn primary" onClick={startDemo}>Run the 3-minute demo</button></Row>}
          <Row title="Install as an app" hint="From a production build (npm run build, then npm run preview) the app installs and opens with no network, so an SOS can be raised offline.">{<button className="btn" onClick={actions.install}>{installable ? "Install Nexus" : "How to install"}</button>}</Row>
          <Row title="Keyboard shortcuts" hint="Press ? at any time."><button className="btn" onClick={() => actions.setSheet(true)}>Show shortcuts</button></Row>
          <Row title="Reset everything" hint="Reloads the first replay and clears the audit trail, alerts and settings."><button className="btn danger" onClick={actions.resetAll}>Reset all state</button></Row>
        </Panel>
      </div>
    </>
  );
}
