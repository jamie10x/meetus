"use client";

import { errorMessage } from "@/lib/errorMessage";

import { useEffect, useRef, useState } from "react";
import {
  AttributionControl,
  MapLibreMap,
  Marker,
  NavigationControl,
  type MapMouseEvent,
} from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import { useTranslations } from "next-intl";
import {
  MAP_STYLE_URL,
  TASHKENT,
  applyBrandTint,
  createMarkerElement,
} from "@/lib/mapStyle";
import { reverseGeocode, searchPlaces, type PlaceResult } from "@/lib/geocode";

type Props = {
  lat: number | null;
  lng: number | null;
  onChange: (lat: number, lng: number) => void;
  /** Called with a human-readable address whenever one is resolved.
   * `source` lets the caller treat the two cases differently: "search"
   * is an explicit pick and should win over whatever is typed, while
   * "pin" is inferred from a dropped marker and shouldn't clobber an
   * address the organizer wrote themselves. */
  onAddressResolved?: (address: string, source: "search" | "pin") => void;
};

export default function LocationPicker({ lat, lng, onChange, onAddressResolved }: Props) {
  const t = useTranslations("eventForm");
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<MapLibreMap | null>(null);
  const markerRef = useRef<Marker | null>(null);
  const pinRequest = useRef({ value: 0 }).current;
  const [ready, setReady] = useState(false);

  // Callbacks live in refs so the map's event handlers — registered once
  // on mount — always call the newest version instead of capturing the
  // first render's closure.
  const onChangeRef = useRef(onChange);
  const onAddressRef = useRef(onAddressResolved);
  useEffect(() => {
    onChangeRef.current = onChange;
    onAddressRef.current = onAddressResolved;
  });

  // Resolve a street address for a pin the user just placed, so the
  // Address field can fill itself in. Best-effort: a failed lookup just
  // leaves the field alone rather than surfacing an error.
  const fillAddress = (nextLat: number, nextLng: number) => {
    if (!onAddressRef.current) return;
    const request = ++pinRequest.value;
    reverseGeocode(nextLat, nextLng)
      .then((address) => { if (request === pinRequest.value && address) onAddressRef.current?.(address, "pin"); })
      .catch(() => {});
  };

  const place = (nextLat: number, nextLng: number) => {
    onChangeRef.current(nextLat, nextLng);
    fillAddress(nextLat, nextLng);
  };

  useEffect(() => {
    if (!containerRef.current || mapRef.current) return;

    const map = new MapLibreMap({
      container: containerRef.current,
      style: MAP_STYLE_URL,
      // MapLibre takes [lng, lat] — the reverse of Leaflet's [lat, lng].
      center: lat !== null && lng !== null ? [lng, lat] : TASHKENT,
      zoom: lat !== null && lng !== null ? 15 : 11,
      attributionControl: false,
    });
    map.addControl(new NavigationControl({ showCompass: false }), "top-right");
    map.addControl(new AttributionControl({ compact: true }), "bottom-right");
    map.on("click", (e: MapMouseEvent) => place(e.lngLat.lat, e.lngLat.lng));
    map.on("load", () => {
      applyBrandTint(map);
      setReady(true);
    });
    // The container is responsive (it sits in a grid column), and
    // MapLibre only auto-handles *window* resizes — a container that
    // reflows on its own would otherwise keep a stale canvas size. This
    // project has already shipped one map-sizing bug (see AGENTS.md), so
    // observe the element directly.
    const observer = new ResizeObserver(() => map.resize());
    observer.observe(containerRef.current);

    mapRef.current = map;
    return () => {
      pinRequest.value++;
      observer.disconnect();
      map.remove();
      mapRef.current = null;
      markerRef.current = null;
    };
    // Mount-only: the map instance manages its own view from here, and
    // recreating it on every lat/lng change would fight the user's panning.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Keep the marker in sync with the coordinates owned by the form.
  useEffect(() => {
    const map = mapRef.current;
    if (!map || !ready) return;

    if (lat === null || lng === null) {
      markerRef.current?.remove();
      markerRef.current = null;
      return;
    }
    if (markerRef.current) {
      markerRef.current.setLngLat([lng, lat]);
      return;
    }
    const marker = new Marker({
      element: createMarkerElement(true),
      draggable: true,
    })
      .setLngLat([lng, lat])
      .addTo(map);
    marker.on("dragend", () => {
      const pos = marker.getLngLat();
      place(pos.lat, pos.lng);
    });
    markerRef.current = marker;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [lat, lng, ready]);

  const flyTo = (nextLat: number, nextLng: number, zoom = 15) => {
    mapRef.current?.flyTo({ center: [nextLng, nextLat], zoom, duration: 900 });
  };

  return (
    <div
      style={{ height: 300 }}
      className="relative w-full overflow-hidden rounded-xl border border-line"
    >
      <div ref={containerRef} className="h-full w-full" />

      <SearchBox
        onPick={(r) => {
          pinRequest.value++;
          onChangeRef.current(r.lat, r.lng);
          onAddressRef.current?.(r.label, "search");
          flyTo(r.lat, r.lng);
        }}
      />

      <LocateButton
        onLocated={(nextLat, nextLng) => {
          place(nextLat, nextLng);
          flyTo(nextLat, nextLng, 16);
        }}
      />

      {lat === null || lng === null ? (
        <div className="pointer-events-none absolute inset-x-0 bottom-3 z-10 flex justify-center">
          <span className="rounded-full bg-ink/85 px-3 py-1.5 text-[11px] font-medium text-dust shadow-card backdrop-blur">
            {t("mapHint")}
          </span>
        </div>
      ) : null}
    </div>
  );
}

/** Floating "locate me" control, styled to the app rather than to
 * MapLibre's default control chrome. */
function LocateButton({
  onLocated,
}: {
  onLocated: (lat: number, lng: number) => void;
}) {
  const t = useTranslations("eventForm");
  const [locating, setLocating] = useState(false);
  const [failed, setFailed] = useState(false);

  const locate = () => {
    if (!navigator.geolocation) {
      setFailed(true);
      return;
    }
    setLocating(true);
    setFailed(false);
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        onLocated(pos.coords.latitude, pos.coords.longitude);
        setLocating(false);
      },
      () => {
        setFailed(true);
        setLocating(false);
      },
    );
  };

  return (
    <div className="absolute bottom-3 right-3 z-10 flex flex-col items-end gap-1.5">
      {failed ? (
        <span className="rounded-md bg-ink/90 px-2 py-1 text-[11px] font-medium text-pomegranate shadow-card">
          {t("locationUnavailable")}
        </span>
      ) : null}
      <button
        type="button"
        onClick={locate}
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

/** Address search over the map, backed by the same free OSM service the
 * tiles come from. */
function SearchBox({ onPick }: { onPick: (r: PlaceResult) => void }) {
  const t = useTranslations("eventForm");
  const tErrors = useTranslations("errors");
  const [failed, setFailed] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<PlaceResult[]>([]);
  const [open, setOpen] = useState(false);
  const [searching, setSearching] = useState(false);

  const search = async () => {
    if (query.trim().length < 3 || searching) return;
    setSearching(true);
    setFailed(null);
    setOpen(true);
    try { setResults(await searchPlaces(query)); }
    catch (error) { setResults([]); setFailed(errorMessage(error, tErrors, t("searchFailed"))); }
    finally { setSearching(false); }
  };

  return (
    <div className="absolute left-3 right-16 top-3 z-10">
      <input
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        onFocus={() => results.length > 0 && setOpen(true)}
        onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); void search(); } if (e.key === "Escape") setOpen(false); }}
        aria-label={t("searchLocationPlaceholder")}
        placeholder={t("searchLocationPlaceholder")}
        className="w-full rounded-lg border border-line bg-ink/90 px-3 py-2 text-sm text-bone placeholder:text-dust-dim shadow-card backdrop-blur transition-colors focus:border-registan-dim focus:outline-none"
      />
      <button type="button" disabled={searching || query.trim().length < 3} onClick={search} className="btn btn-secondary btn-sm mt-1">{t("searchLocation")}</button>
      <a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noreferrer" className="ml-2 text-[10px] text-dust">© OpenStreetMap</a>
      {open && query.trim().length >= 3 ? (
        <ul className="mt-1 max-h-44 overflow-y-auto rounded-lg border border-line bg-ink-overlay shadow-pop">
          {searching ? (
            <li className="px-3 py-2 text-xs text-dust-dim">{t("searching")}</li>
          ) : failed ? (
            <li role="alert" className="px-3 py-2 text-xs text-pomegranate">{failed}</li>
          ) : results.length > 0 ? (
            results.map((r, i) => (
              <li key={i}>
                <button
                  type="button"
                  onClick={() => {
                    onPick(r);
                    setQuery(r.label);
                    setOpen(false);
                  }}
                  className="block w-full truncate px-3 py-2 text-left text-xs text-dust transition-colors hover:bg-ink-raised hover:text-bone"
                >
                  {r.label}
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
