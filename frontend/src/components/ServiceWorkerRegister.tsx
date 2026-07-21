"use client";

import { useEffect } from "react";

/** Registers the offline/PWA service worker (public/sw.js) once mounted. */
export default function ServiceWorkerRegister() {
  useEffect(() => {
    if (!("serviceWorker" in navigator)) return;
    navigator.serviceWorker.register("/sw.js").catch(() => {
      // Offline support degrading silently is fine — the site works
      // identically without it, just without the offline fallback.
    });
  }, []);

  return null;
}
