import { Pill, LayoutDashboard, TriangleAlert, Map, Route, Hospital, Tent, Users, CloudRain, Siren, Construction, FlaskConical, Boxes, ScrollText, BookOpen, Network, FileText, Settings, PanelLeftClose, PanelLeftOpen, ClipboardList } from "lucide-react";
import Logo from "./Logo.jsx";
import { useStore, actions, canSee } from "../state/store.js";
import { useThumb } from "./ui.jsx";
import { useT } from "../i18n.js";

export const NAV = [
  [["assignment", "My assignment", ClipboardList], ["overview", "Overview", LayoutDashboard], ["incidents", "Incidents", TriangleAlert], ["map", "Map", Map]],
  [["routes", "Routes", Route, "मार्ग"], ["hospitals", "Hospitals", Hospital, "आरोग्य"], ["pharmacies", "Pharmacies", Pill, "औषधि"], ["sos", "Citizen SOS", Siren, "आह्वान"]],
  [["shelters", "Shelters", Tent, "आश्रय"], ["responders", "Responders", Users, "रक्षक"], ["warning", "Early warning", CloudRain, "पूर्वसूचना"]],
  [["infrastructure", "Infrastructure", Construction], ["evaluation", "Evaluation", FlaskConical], ["modules", "Modules", Boxes], ["architecture", "Architecture", Network]],
  [["sitrep", "Situation report", FileText], ["audit", "Audit trail", ScrollText], ["sources", "Sources", BookOpen], ["settings", "Settings", Settings]]
];

export default function Sidebar() {
  const view = useStore((s) => s.view);
  const open = useStore((s) => s.navOpen);
  const rail = useStore((s) => s.rail);
  const role = useStore((s) => s.role);
  const openCount = useStore((s) => s.incidents.filter((i) => i.status !== "Closed").length);
  const t = useT();
  const lang = useStore((s) => s.lang);
  const [box, thumb] = useThumb(view + role + rail + lang);
  const groups = NAV.map((g) => g.filter(([id]) => canSee(role, id) && (id !== "assignment" || role === "Responder"))).filter((g) => g.length);
  return (
    <aside className={"side" + (open ? " open" : "")}>
      <button className="brand" onClick={() => actions.go("overview")} aria-label="Nexus, go to overview">
        <Logo />
        <span>
          <b>Nexus</b>
          <small>{t("Dehradun control room")}</small>
        </span>
      </button>
      <nav className="nav" aria-label="Sections" ref={box}>
        <i className="thumb" ref={thumb} aria-hidden="true" />
        {groups.map((group, gi) => (
          <div key={gi}>
            {gi > 0 && <hr />}
            {group.map(([id, label, Icon, dv]) => (
              <button key={id} title={label} className={view === id ? "on" : ""} aria-current={view === id ? "page" : undefined} onClick={() => actions.go(id)}>
                <Icon />
                <span className="lbl">{t(label)}</span>
                {id === "incidents" && openCount > 0 ? <span className="count">{openCount}</span> : dv && lang !== "hi" ? <i lang="sa">{dv}</i> : null}
              </button>
            ))}
          </div>
        ))}
      </nav>
      <button className="rail-btn" onClick={actions.toggleRail} aria-label={rail ? "Expand the sidebar" : "Collapse the sidebar"}>
        {rail ? <PanelLeftOpen /> : <PanelLeftClose />}<span>{t("Collapse")}</span>
      </button>
      <div className="side-foot">
        <b>Uttarakhand state control room</b>
        Pilot district: Dehradun
      </div>
    </aside>
  );
}
