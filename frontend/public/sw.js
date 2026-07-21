// Minimal hand-rolled service worker (no Workbox — this is the only PWA
// feature the app needs, not worth a build-time dependency). Three jobs:
//
// 1. Network-first HTML page navigations, falling back to the last
//    cached copy offline — pages stay fresh while online (event
//    listings and RSVP counts change constantly; a cache-first page
//    would go stale) and previously-visited pages, including /tickets,
//    still open with no network.
// 2. Cache-first Next's fingerprinted static assets (_next/static, the
//    generated PWA icons) — immutable by construction, safe to cache
//    aggressively.
// 3. Network-first the ticket list API response, falling back to the
//    last-known-good copy when offline — the actual QR code is rendered
//    entirely client-side from that response's `qr` string (see
//    src/app/[locale]/tickets/page.tsx), so once the ticket list is
//    cached, showing a ticket at the door needs no network at all.
//
// Bump these on any change to the caching logic so activate() clears the
// old caches instead of serving stale entries forever.
const SHELL_CACHE = "meetus-shell-v1";
const API_CACHE = "meetus-api-v1";

const TICKETS_PATH = "/api/me/tickets";

self.addEventListener("install", () => {
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) =>
        Promise.all(
          keys
            .filter((key) => key !== SHELL_CACHE && key !== API_CACHE)
            .map((key) => caches.delete(key)),
        ),
      )
      .then(() => self.clients.claim()),
  );
});

async function networkFirst(request, cacheName) {
  const cache = await caches.open(cacheName);
  try {
    const response = await fetch(request);
    if (response.ok) cache.put(request, response.clone());
    return response;
  } catch (err) {
    const cached = await cache.match(request);
    if (cached) return cached;
    throw err;
  }
}

async function cacheFirst(request, cacheName) {
  const cache = await caches.open(cacheName);
  const cached = await cache.match(request);
  if (cached) return cached;
  const response = await fetch(request);
  if (response.ok) cache.put(request, response.clone());
  return response;
}

self.addEventListener("fetch", (event) => {
  if (event.request.method !== "GET") return;

  const url = new URL(event.request.url);

  // The ticket list: cross-origin in dev (API on :8080, app on :3000),
  // same-origin in prod (Caddy proxies /api/* on one host) — matched by
  // path only so both topologies work identically.
  if (url.pathname === TICKETS_PATH) {
    event.respondWith(networkFirst(event.request, API_CACHE));
    return;
  }

  if (url.origin !== self.location.origin) return;

  // Full-page loads: network-first so browsing never shows stale event
  // data while online.
  if (event.request.mode === "navigate") {
    event.respondWith(networkFirst(event.request, SHELL_CACHE));
    return;
  }

  // Everything else same-origin (fingerprinted JS/CSS, generated icons):
  // content-hashed by Next's build, safe to cache-first.
  event.respondWith(cacheFirst(event.request, SHELL_CACHE));
});
