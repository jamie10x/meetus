"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import {
  AttributionControl,
  LngLatBounds,
  MapLibreMap,
  Marker,
  NavigationControl,
  Popup,
} from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import { useLocale, useTranslations } from "next-intl";
import { useRouter } from "@/i18n/navigation";
import type { EventItem } from "@/lib/types";
import { formatEventDate } from "@/components/EventCard";
import {
  MAP_STYLE_URL,
  TASHKENT,
  applyBrandTint,
  createMarkerElement,
} from "@/lib/mapStyle";

type Props = {
  events: EventItem[];
};

type LocatedEvent = EventItem & { lat: number; lng: number };

export default function EventMap({ events }: Props) {
  const locale = useLocale();
  const t = useTranslations("explore");
  const router = useRouter();

  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<MapLibreMap | null>(null);
  const markersRef = useRef<Marker[]>([]);
  const [ready, setReady] = useState(false);

  const located = useMemo(
    () =>
      events.filter(
        (e): e is LocatedEvent => e.lat !== null && e.lng !== null,
      ),
    [events],
  );

  // Navigation goes through a ref so the popup click handlers registered
  // below always reach the current router instance.
  const routerRef = useRef(router);
  useEffect(() => {
    routerRef.current = router;
  });

  useEffect(() => {
    if (!containerRef.current || mapRef.current) return;

    const map = new MapLibreMap({
      container: containerRef.current,
      style: MAP_STYLE_URL,
      center: TASHKENT,
      zoom: 10,
      attributionControl: false,
    });
    map.addControl(new NavigationControl({ showCompass: false }), "top-right");
    map.addControl(new AttributionControl({ compact: true }), "bottom-right");
    map.on("load", () => {
      applyBrandTint(map);
      setReady(true);
    });

    // MapLibre only auto-handles window resizes; observe the element so a
    // container that reflows on its own can't leave a stale canvas size.
    const observer = new ResizeObserver(() => map.resize());
    observer.observe(containerRef.current);

    mapRef.current = map;
    return () => {
      observer.disconnect();
      map.remove();
      mapRef.current = null;
      markersRef.current = [];
    };
  }, []);

  useEffect(() => {
    const map = mapRef.current;
    if (!map || !ready) return;

    markersRef.current.forEach((m) => m.remove());
    markersRef.current = [];
    if (located.length === 0) return;

    for (const e of located) {
      // Popup content is built with DOM APIs rather than an HTML string:
      // titles and venue names are organizer-supplied text, and
      // textContent can't be escaped out of the way innerHTML could.
      const node = document.createElement("div");
      node.className = "flex flex-col gap-0.5";

      const title = document.createElement("button");
      title.type = "button";
      title.textContent = e.title;
      title.className = "text-left text-sm font-semibold text-bone hover:underline";
      title.onclick = () => routerRef.current.push(`/events/${e.id}`);

      const when = document.createElement("div");
      when.textContent = formatEventDate(e.startsAt, locale);
      when.className = "font-mono text-[11px] text-registan-strong";

      const where = document.createElement("div");
      where.textContent = e.locationName ?? e.citySlug ?? "";
      where.className = "text-[11px] text-dust";

      node.append(title, when, where);

      const marker = new Marker({ element: createMarkerElement() })
        .setLngLat([e.lng, e.lat])
        .setPopup(new Popup({ offset: 14, closeButton: false }).setDOMContent(node))
        .addTo(map);
      markersRef.current.push(marker);
    }

    // Frame every pin, but don't zoom past street level for a single one.
    const bounds = new LngLatBounds();
    located.forEach((e) => bounds.extend([e.lng, e.lat]));
    map.fitBounds(bounds, { padding: 64, maxZoom: 14, duration: 0 });
  }, [located, ready, locale]);

  return (
    <div
      style={{ height: 520 }}
      className="relative w-full overflow-hidden rounded-card border border-line"
    >
      <div ref={containerRef} className="h-full w-full" />
      {located.length === 0 ? (
        <div className="absolute inset-0 z-10 flex items-center justify-center bg-ink/70 text-sm text-dust backdrop-blur-sm">
          {t("noMappableEvents")}
        </div>
      ) : null}
    </div>
  );
}
