import MapView from "../components/MapView.jsx";

export default function MapPage() {
  return (
    <>
      <div className="page-head"><div><h1>Map</h1><p>Every marker opens its record and source. Use a lens to cut the picture down to one hazard.</p></div></div>
      <div style={{ height: "calc(100vh - 230px)", minHeight: 520 }}><MapView /></div>
    </>
  );
}
