// Private data lives in an explicitly account-scoped application snapshot.
// Never cache API responses, RSC requests, or arbitrary same-origin GETs.
const SHELL_CACHE = "meetus-shell-v2";
self.addEventListener("install", () => self.skipWaiting());
self.addEventListener("activate", (event) => {
  event.waitUntil(caches.keys().then((keys) => Promise.all(
    keys.filter((key) => key.startsWith("meetus-") && key !== SHELL_CACHE).map((key) => caches.delete(key)),
  )).then(() => self.clients.claim()));
});
self.addEventListener("fetch", (event) => {
  const request = event.request;
  const url = new URL(request.url);
  if (request.method !== "GET" || url.origin !== self.location.origin || request.headers.has("Authorization")) return;
  if (url.pathname.startsWith("/api/") || request.headers.get("RSC") === "1") return;
  const navigation = request.mode === "navigate";
  if (!navigation && !url.pathname.startsWith("/_next/static/")) return;
  event.respondWith((async () => {
    const cache = await caches.open(SHELL_CACHE);
    if (!navigation) {
      const cached = await cache.match(request);
      if (cached) return cached;
    }
    try {
      const response = await fetch(request);
      // Only the public ticket page shell is needed for offline relaunch.
      if (response.ok && (!navigation || /^\/(uz|ru|en)\/tickets\/?$/.test(url.pathname))) {
        await cache.put(request, response.clone());
      }
      return response;
    } catch (error) {
      const cached = await cache.match(request);
      if (cached) return cached;
      throw error;
    }
  })());
});
