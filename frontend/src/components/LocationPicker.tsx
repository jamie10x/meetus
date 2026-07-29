"use client";

import { useMemo } from "react";
import { MapContainer, TileLayer, Marker, useMapEvents } from "react-leaflet";
import L, { type LeafletMouseEvent } from "leaflet";
import "leaflet/dist/leaflet.css";

// Same brand-blue dot as EventMap — no image assets, sidesteps the
// bundler issue with Leaflet's default marker icons.
const markerIcon = L.divIcon({
  className: "",
  html: '<span style="display:block;width:16px;height:16px;border-radius:9999px;background:#5b9dff;border:2px solid #070b16;box-shadow:0 0 0 3px rgba(91,157,255,0.28)"></span>',
  iconSize: [16, 16],
  iconAnchor: [8, 8],
});

const TASHKENT: [number, number] = [41.2995, 69.2401];

type Props = {
  lat: number | null;
  lng: number | null;
  onChange: (lat: number, lng: number) => void;
};

function ClickHandler({ onChange }: { onChange: (lat: number, lng: number) => void }) {
  useMapEvents({
    click(e: LeafletMouseEvent) {
      onChange(e.latlng.lat, e.latlng.lng);
    },
  });
  return null;
}

/** Click-to-drop-pin location picker, built on the same free OSM/CARTO
 * tiles as the Explore page's map — no API key, no Google Maps billing. */
export default function LocationPicker({ lat, lng, onChange }: Props) {
  // Only the initial center matters — recentering on every marker move
  // would fight the user while they're panning around.
  const center = useMemo<[number, number]>(
    () => (lat !== null && lng !== null ? [lat, lng] : TASHKENT),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [],
  );

  return (
    <div
      style={{ height: 260 }}
      className="w-full overflow-hidden rounded-xl border border-line"
    >
      <MapContainer
        center={center}
        zoom={12}
        style={{ height: "100%", width: "100%", background: "#070b16" }}
      >
        <TileLayer
          url="https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png"
          attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors &copy; <a href="https://carto.com/attributions">CARTO</a>'
        />
        <ClickHandler onChange={onChange} />
        {lat !== null && lng !== null ? (
          <Marker
            position={[lat, lng]}
            icon={markerIcon}
            draggable
            eventHandlers={{
              dragend: (e) => {
                const pos = (e.target as L.Marker).getLatLng();
                onChange(pos.lat, pos.lng);
              },
            }}
          />
        ) : null}
      </MapContainer>
    </div>
  );
}
