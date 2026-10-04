import React from "react";
import { createRoot } from "react-dom/client";
import "@fontsource/spectral/300.css";
import "@fontsource/spectral/400.css";
import "@fontsource/spectral/500.css";
import "@fontsource/ibm-plex-sans/400.css";
import "@fontsource/ibm-plex-sans/500.css";
import "@fontsource/tiro-devanagari-sanskrit/400.css";
import "./styles/app.css";
import "./styles/glass.css";
import "./styles/leather.css";
import App from "./App.jsx";

createRoot(document.getElementById("root")).render(<React.StrictMode><App /></React.StrictMode>);

/* Offline shell: only in a production build, so it never interferes with the dev server. */
if (import.meta.env.PROD && "serviceWorker" in navigator) {
  window.addEventListener("load", () => {
    navigator.serviceWorker.register(import.meta.env.BASE_URL + "sw.js").then(() => navigator.serviceWorker.ready).then((reg) => {
      const urls = performance.getEntriesByType("resource").map((r) => r.name).filter((u) => u.startsWith(location.origin));
      reg.active && reg.active.postMessage({ type: "precache", urls });
    }).catch(() => { /* no service worker: the app still runs, just not offline */ });
  });
}
