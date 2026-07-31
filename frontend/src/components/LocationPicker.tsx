"use client";

import { useEffect, useMemo, useState } from "react";
import { MapContainer, TileLayer, Marker, useMap, useMapEvents } from "react-leaflet";
import L, { type LeafletMouseEvent } from "leaflet";
import "leaflet/dist/leaflet.css";
import { useTranslations } from "next-intl";

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
  /** Fired when a search result is picked, so the caller can also fill a
   * separate address text field — the search box itself only owns the map. */
  onSelectAddress?: (address: string) => void;
};

function ClickHandler({ onChange }: { onChange: (lat: number, lng: number) => void }) {
  useMapEvents({
    click(e: LeafletMouseEvent) {
      onChange(e.latlng.lat, e.latlng.lng);
    },
  });
  return null;
}

/** Floating in-map "locate me" control (same idea as Google Maps' target
 * button) — flies the map to the browser's geolocation and drops the pin
 * there, instead of the coordinates jumping with no visual transition. */
function LocateControl({ onChange }: { onChange: (lat: number, lng: number) => void }) {
  const t = useTranslations("eventForm");
  const map = useMap();
  const [locating, setLocating] = useState(false);
  const [error, setError] = useState(false);

  const handleClick = () => {
    if (!navigator.geolocation) {
      setError(true);
      return;
    }
    setLocating(true);
    setError(false);
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        const { latitude, longitude } = pos.coords;
        onChange(latitude, longitude);
        map.flyTo([latitude, longitude], 15, { duration: 0.8 });
        setLocating(false);
      },
      () => {
        setError(true);
        setLocating(false);
      },
    );
  };

  return (
    <div className="absolute bottom-2.5 right-2.5 z-[400] flex flex-col items-end gap-1.5">
      {error ? (
        <span className="rounded-md bg-ink/90 px-2 py-1 text-[11px] font-medium text-pomegranate shadow-card">
          {t("locationUnavailable")}
        </span>
      ) : null}
      <button
        type="button"
        onClick={handleClick}
        disabled={locating}
        aria-label={t("useMyLocation")}
        title={t("useMyLocation")}
        className="flex h-9 w-9 items-center justify-center rounded-full border border-line bg-ink-raised text-bone shadow-card transition-colors hover:border-registan-dim hover:text-registan-strong disabled:opacity-50"
      >
        {locating ? (
          <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-t-transparent" />
        ) : (
          <svg width="17" height="17" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <circle cx="12" cy="12" r="3" stroke="currentColor" strokeWidth="2" />
            <path
              d="M12 2v3M12 19v3M2 12h3M19 12h3"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
            />
          </svg>
        )}
      </button>
    </div>
  );
}

type SearchResult = { display_name: string; lat: string; lon: string };

/** Address search box, top of the map — geocodes via Nominatim (the same
 * free OSM search service, no API key) and flies the map to the pick. */
function SearchBox({
  onChange,
  onSelectAddress,
}: {
  onChange: (lat: number, lng: number) => void;
  onSelectAddress?: (address: string) => void;
}) {
  const t = useTranslations("eventForm");
  const map = useMap();
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<SearchResult[]>([]);
  const [open, setOpen] = useState(false);
  const [searching, setSearching] = useState(false);

  useEffect(() => {
    if (query.trim().length < 3) {
      setResults([]);
      return;
    }
    let cancelled = false;
    const handle = setTimeout(async () => {
      setSearching(true);
      try {
        const res = await fetch(
          `https://nominatim.openstreetmap.org/search?format=json&limit=5&q=${encodeURIComponent(query)}`,
        );
        const data = (await res.json()) as SearchResult[];
        if (!cancelled) {
          setResults(data);
          setOpen(true);
        }
      } catch {
        if (!cancelled) setResults([]);
      } finally {
        if (!cancelled) setSearching(false);
      }
    }, 500);
    return () => {
      cancelled = true;
      clearTimeout(handle);
    };
  }, [query]);

  const pick = (r: SearchResult) => {
    const lat = parseFloat(r.lat);
    const lon = parseFloat(r.lon);
    onChange(lat, lon);
    onSelectAddress?.(r.display_name);
    map.flyTo([lat, lon], 15, { duration: 0.8 });
    setQuery(r.display_name);
    setOpen(false);
  };

  return (
    <div className="absolute left-[46px] right-2.5 top-2.5 z-[400]">
      <input
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        onFocus={() => results.length > 0 && setOpen(true)}
        placeholder={t("searchLocationPlaceholder")}
        className="w-full rounded-lg border border-line bg-ink/90 px-3 py-2 text-sm text-bone placeholder:text-dust-dim shadow-card backdrop-blur transition-colors focus:border-registan-dim focus:outline-none"
      />
      {open && query.trim().length >= 3 ? (
        <ul className="mt-1 max-h-48 overflow-y-auto rounded-lg border border-line bg-ink-overlay shadow-pop">
          {searching ? (
            <li className="px-3 py-2 text-xs text-dust-dim">{t("searching")}</li>
          ) : results.length > 0 ? (
            results.map((r, i) => (
              <li key={i}>
                <button
                  type="button"
                  onClick={() => pick(r)}
                  className="block w-full truncate px-3 py-2 text-left text-xs text-dust transition-colors hover:bg-ink-raised hover:text-bone"
                >
                  {r.display_name}
                </button>
              </li>
            ))
          ) : (
            <li className="px-3 py-2 text-xs text-dust-dim">{t("searchNoResults")}</li>
          )}
        </ul>
      ) : null}
    </div>
  );
}

/** Click-to-drop-pin location picker, built on the same free OSM/CARTO
 * tiles as the Explore page's map — no API key, no Google Maps billing. */
export default function LocationPicker({ lat, lng, onChange, onSelectAddress }: Props) {
  const t = useTranslations("eventForm");
  // Only the initial center matters — recentering on every marker move
  // would fight the user while they're panning around.
  const center = useMemo<[number, number]>(
    () => (lat !== null && lng !== null ? [lat, lng] : TASHKENT),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [],
  );
  const hasPin = lat !== null && lng !== null;

  return (
    <div
      style={{ height: 260 }}
      className="relative w-full overflow-hidden rounded-xl border border-line"
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
        <LocateControl onChange={onChange} />
        <SearchBox onChange={onChange} onSelectAddress={onSelectAddress} />
        {!hasPin ? (
          <div className="pointer-events-none absolute inset-x-0 bottom-2.5 z-[400] flex justify-center">
            <span className="rounded-full bg-ink/85 px-3 py-1 text-[11px] font-medium text-dust shadow-card backdrop-blur">
              {t("mapHint")}
            </span>
          </div>
        ) : null}
        {hasPin ? (
          <Marker
            position={[lat!, lng!]}
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
