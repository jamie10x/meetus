import type { MapLibreMap } from "maplibre-gl";

/**
 * Shared MapLibre configuration for every map in the app (the Explore
 * map and the event-form location picker).
 *
 * Tiles come from OpenFreeMap: free OpenStreetMap-derived *vector* tiles
 * with no API key, no registration, and no request limits. Vector (not
 * raster) is the whole point — the browser renders geometry on the GPU,
 * so zooming stays smooth and labels stay crisp at any scale, instead of
 * the blur-and-snap you get from pre-rendered PNG tiles.
 *
 * MapLibre adds OSM/OpenFreeMap attribution automatically, so there's no
 * manual attribution string to maintain here.
 */
export const MAP_STYLE_URL = "https://tiles.openfreemap.org/styles/dark";

/** Fallback center (Tashkent) for a map with nothing to focus on yet. */
export const TASHKENT: [number, number] = [69.2401, 41.2995]; // [lng, lat]

/**
 * Brand-blue map pin as a detached DOM node, handed to MapLibre's
 * `Marker({ element })`. Built in code rather than as an image asset so
 * it inherits the palette and needs no file to keep in sync — same
 * reasoning as the old Leaflet divIcon, minus Leaflet's broken-relative-
 * icon-URL problem.
 *
 * `pulse` adds a slow halo, used for the single draggable pin in the
 * location picker so it reads as "grab me"; the many read-only pins on
 * the Explore map leave it off to stay quiet.
 */
export function createMarkerElement(pulse = false): HTMLElement {
  const el = document.createElement("div");
  el.className = `map-pin${pulse ? " map-pin-pulse" : ""}`;
  return el;
}

/** Reads a brand token off :root so map colors can't drift from the rest
 * of the design system (Tailwind v4's `@theme` emits real custom
 * properties, so these resolve at runtime). */
function token(name: string, fallback: string): string {
  if (typeof window === "undefined") return fallback;
  const value = getComputedStyle(document.documentElement)
    .getPropertyValue(name)
    .trim();
  return value || fallback;
}

/**
 * Repaints the basemap's ground and water in the app's own near-blacks.
 * OpenFreeMap's "dark" style is a *neutral* black (rgb(12,12,12)), which
 * reads as a foreign grey rectangle next to the blue-tinted `ink`
 * surfaces it sits on. Retinting is only possible because these are
 * vector tiles — the style is JSON the client owns, not baked pixels.
 *
 * Layer lookups are guarded: if OpenFreeMap ever renames a layer, the
 * map keeps its stock colors instead of throwing.
 */
export function applyBrandTint(map: MapLibreMap): void {
  const ink = token("--color-ink", "#070b16");
  const water = token("--color-ink-raised", "#101a30");

  if (map.getLayer("background")) {
    map.setPaintProperty("background", "background-color", ink);
  }
  if (map.getLayer("water")) {
    map.setPaintProperty("water", "fill-color", water);
  }
  if (map.getLayer("waterway")) {
    map.setPaintProperty("waterway", "line-color", water);
  }
}
