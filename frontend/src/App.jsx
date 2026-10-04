import { useEffect } from "react";
import Sidebar from "./components/Sidebar.jsx";
import Topbar from "./components/Topbar.jsx";
import Drawer from "./components/Drawer.jsx";
import CommandPalette, { Shortcuts } from "./components/CommandPalette.jsx";
import Demo from "./components/Demo.jsx";
import { Grain, RefractDefs, Boot, useSheen } from "./components/Fx.jsx";
import { useStore, actions, getState } from "./state/store.js";
import Overview from "./views/Overview.jsx";
import Incidents from "./views/Incidents.jsx";
import MapPage from "./views/MapPage.jsx";
import RoutesView from "./views/RoutesView.jsx";
import Hospitals from "./views/Hospitals.jsx";
import Pharmacies from "./views/Pharmacies.jsx";
import Shelters from "./views/Shelters.jsx";
import Responders from "./views/Responders.jsx";
import Warning from "./views/Warning.jsx";
import Sos from "./views/Sos.jsx";
import Infrastructure from "./views/Infrastructure.jsx";
import Evaluation from "./views/Evaluation.jsx";
import Modules from "./views/Modules.jsx";
import Architecture from "./views/Architecture.jsx";
import Sitrep from "./views/Sitrep.jsx";
import Audit from "./views/Audit.jsx";
import Sources from "./views/Sources.jsx";
import Settings from "./views/Settings.jsx";
import Assignment from "./views/Assignment.jsx";

const VIEWS = { overview: Overview, incidents: Incidents, map: MapPage, routes: RoutesView, hospitals: Hospitals, pharmacies: Pharmacies, shelters: Shelters, responders: Responders, warning: Warning, sos: Sos, infrastructure: Infrastructure, evaluation: Evaluation, modules: Modules, architecture: Architecture, sitrep: Sitrep, audit: Audit, sources: Sources, settings: Settings, assignment: Assignment };
const GOTO = { o: "overview", i: "incidents", m: "map", r: "routes", h: "hospitals", p: "pharmacies", s: "sos", a: "audit" };
const CHROMIUM = typeof navigator !== "undefined" && /Chrome\//.test(navigator.userAgent);

function useShortcuts() {
  useEffect(() => {
    let g = 0;
    const key = (e) => {
      const s = getState();
      const typing = /INPUT|TEXTAREA|SELECT/.test(e.target.tagName);
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "k") { e.preventDefault(); if (s.role === "Controller") actions.setPalette(!s.palette); return; }
      if (e.key === "Escape") {
        if (s.pick) actions.setPick(null); else if (s.palette) actions.setPalette(false); else if (s.sheet) actions.setSheet(false); else if (s.selected) actions.closeDrawer(); else if (s.demo) actions.setDemo(null);
        return;
      }
      if (typing || e.ctrlKey || e.metaKey || e.altKey || s.palette || s.role !== "Controller") return;
      const k = e.key.toLowerCase();
      if (Date.now() - g < 900 && GOTO[k]) { g = 0; actions.go(GOTO[k]); return; }
      if (k === "g") { g = Date.now(); return; }
      if (e.key === "/") { e.preventDefault(); actions.setPalette(true); }
      else if (e.key === "?") actions.setSheet(!s.sheet);
      else if (k === "t") actions.setTheme(s.theme === "dark" ? "light" : "dark");
      else if (k === "n") actions.setNet(!s.net);
      else if (e.key === " " && s.clock.max) { e.preventDefault(); actions.play(!s.clock.playing); }
    };
    window.addEventListener("keydown", key);
    return () => window.removeEventListener("keydown", key);
  }, []);
}

export default function App() {
  const view = useStore((s) => s.view);
  const theme = useStore((s) => s.theme);
  const toast = useStore((s) => s.toast);
  const navOpen = useStore((s) => s.navOpen);
  const rail = useStore((s) => s.rail);
  const settings = useStore((s) => s.settings);
  const demo = useStore((s) => s.demo);
  const glass = settings.glass === "liquid" && !CHROMIUM ? "frosted" : settings.glass;
  useEffect(() => {
    const d = document.documentElement.dataset;
    d.theme = theme; d.glass = glass; d.motion = settings.motion; d.density = settings.density;
  }, [theme, glass, settings.motion, settings.density]);
  useSheen();
  useShortcuts();
  const View = VIEWS[view] || Overview;
  return (
    <div className={"app" + (rail ? " rail" : "")}>
      <Sidebar />
      {navOpen && <div className="scrim" onClick={actions.toggleNav} />}
      <div className="main">
        <Topbar />
        <main className="page" key={view}><View /></main>
      </div>
      <Drawer />
      <CommandPalette />
      <Shortcuts />
      <Demo />
      {toast && !demo && <div className="toast" role="status">{toast}</div>}
      <RefractDefs animate={settings.motion === "full"} />
      <Grain />
      <Boot motion={settings.motion} />
    </div>
  );
}
