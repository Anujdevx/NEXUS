/* Nexus service worker: keeps the app shell on the device so the citizen screen opens
   and an SOS can be raised with no network at all. Map tiles and APIs are not cached. */
const CACHE = "nexus-shell-v2";
const SHELL = ["./", "./index.html", "./manifest.webmanifest", "./nexus-mark.svg", "./icon-192.png"];

self.addEventListener("install", (e) => { self.skipWaiting(); e.waitUntil(caches.open(CACHE).then((c) => c.addAll(SHELL))); });
self.addEventListener("activate", (e) => e.waitUntil(
  caches.keys().then((keys) => Promise.all(keys.filter((k) => k.startsWith("nexus-shell-") && k !== CACHE).map((k) => caches.delete(k)))).then(() => self.clients.claim())
));
/* The page tells us which files it loaded, so the first visit is enough to work offline. */
self.addEventListener("message", (e) => {
  if (e.data && e.data.type === "precache") e.waitUntil(caches.open(CACHE).then((c) => Promise.all(e.data.urls.map((u) => c.add(u).catch(() => null)))));
});
self.addEventListener("fetch", (e) => {
  const req = e.request;
  const url = new URL(req.url);
  if (req.method !== "GET" || url.origin !== self.location.origin) return;
  /* The backend is never cached: shared state must be fresh, and with no network the console falls back to its local engine. */
  if (/^\/(api|svc|ws)\//.test(url.pathname)) return;
  if (req.mode === "navigate") {
    e.respondWith(fetch(req).then((res) => { const copy = res.clone(); caches.open(CACHE).then((c) => c.put("./index.html", copy)); return res; }).catch(() => caches.match("./index.html")));
    return;
  }
  e.respondWith(caches.match(req).then((hit) => hit || fetch(req).then((res) => {
    if (res.ok) { const copy = res.clone(); caches.open(CACHE).then((c) => c.put(req, copy)); }
    return res;
  })));
});
