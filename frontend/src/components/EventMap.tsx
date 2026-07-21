"use client";

import { useMemo } from "react";
import { MapContainer, TileLayer, Marker, Popup } from "react-leaflet";
import L from "leaflet";
import "leaflet/dist/leaflet.css";
import { Link } from "@/i18n/navigation";
import type { EventItem } from "@/lib/types";
import { formatEventDate } from "@/components/EventCard";
import { useLocale, useTranslations } from "next-intl";

// A small brand-blue dot instead of Leaflet's default blue/gold pin —
// matches the dark theme, and sidesteps the well-known bundler issue
// where Leaflet's default marker icons resolve to broken relative image
// URLs (no image assets needed here at all).
const markerIcon = L.divIcon({
  className: "",
  html: '<span style="display:block;width:14px;height:14px;border-radius:9999px;background:#5b9dff;border:2px solid #070b16;box-shadow:0 0 0 3px rgba(91,157,255,0.28)"></span>',
  iconSize: [14, 14],
  iconAnchor: [7, 7],
});

type Props = {
  events: EventItem[];
};

export default function EventMap({ events }: Props) {
  const locale = useLocale();
  const t = useTranslations("explore");

  const located = useMemo(
    () => events.filter((e): e is EventItem & { lat: number; lng: number } =>
      e.lat !== null && e.lng !== null,
    ),
    [events],
  );

  if (located.length === 0) {
    return (
      <div
        style={{ height: 520 }}
        className="flex w-full items-center justify-center rounded-card border border-line bg-ink-raised text-sm text-dust"
      >
        {t("noMappableEvents")}
      </div>
    );
  }

  return (
    <div style={{ height: 520 }} className="w-full overflow-hidden rounded-card border border-line">
      <MapContainer
        center={[located[0].lat, located[0].lng]}
        zoom={12}
        style={{ height: "100%", width: "100%", background: "#070b16" }}
      >
        {/* CARTO's free dark basemap — matches the site's dark theme far
            better than stock OpenStreetMap tiles; still built on OSM data,
            so the OSM attribution stays alongside CARTO's own. */}
        <TileLayer
          url="https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png"
          attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors &copy; <a href="https://carto.com/attributions">CARTO</a>'
        />
        {located.map((e) => (
          <Marker key={e.id} position={[e.lat, e.lng]} icon={markerIcon}>
            <Popup>
              <Link
                href={`/events/${e.id}`}
                className="font-semibold text-ink hover:underline"
              >
                {e.title}
              </Link>
              <div className="mt-1 text-xs text-ink/70">
                {formatEventDate(e.startsAt, locale)}
              </div>
              <div className="text-xs text-ink/70">
                {e.locationName ?? e.citySlug ?? ""}
              </div>
            </Popup>
          </Marker>
        ))}
      </MapContainer>
    </div>
  );
}
