import type { MetadataRoute } from "next";

// Static (not localized) — a PWA manifest has one name/description, and
// most install surfaces (Android's install prompt, iOS "Add to Home
// Screen") don't read the visitor's locale anyway. uz is the platform's
// default/majority-audience language (see i18n/routing.ts).
export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "Meetus.uz — Tadbirlar va uchrashuvlar",
    short_name: "Meetus.uz",
    description: "O'zbekiston bo'ylab tadbirlarni toping va bir tegishda qo'shiling.",
    start_url: "/uz",
    display: "standalone",
    background_color: "#160f16",
    theme_color: "#160f16",
    icons: [
      { src: "/pwa-icon/192", sizes: "192x192", type: "image/png" },
      { src: "/pwa-icon/512", sizes: "512x512", type: "image/png" },
      { src: "/pwa-icon/512", sizes: "512x512", type: "image/png", purpose: "maskable" },
    ],
  };
}
