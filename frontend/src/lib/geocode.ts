/**
 * Address lookup via Nominatim, OpenStreetMap's own free geocoding
 * service — no API key, same data behind the map tiles.
 *
 * Nominatim's usage policy asks for no heavy automated traffic; both
 * calls here are user-initiated and the search path is debounced by its
 * caller, which keeps this well inside acceptable use.
 */

const NOMINATIM = "https://nominatim.openstreetmap.org";

export type PlaceResult = {
  label: string;
  lat: number;
  lng: number;
};

type NominatimPlace = {
  display_name?: string;
  lat?: string;
  lon?: string;
};

/** Forward search: free-text query → ranked candidate places. */
export async function searchPlaces(query: string): Promise<PlaceResult[]> {
  const res = await fetch(
    `${NOMINATIM}/search?format=json&limit=5&q=${encodeURIComponent(query)}`,
  );
  if (!res.ok) return [];
  const data = (await res.json()) as NominatimPlace[];
  return data.flatMap((p) => {
    if (!p.display_name || !p.lat || !p.lon) return [];
    return [{ label: p.display_name, lat: parseFloat(p.lat), lng: parseFloat(p.lon) }];
  });
}

/** Reverse lookup: coordinates → a street address, or null if the point
 * has nothing mapped near it (open country, a lake, mid-ocean). */
export async function reverseGeocode(lat: number, lng: number): Promise<string | null> {
  const res = await fetch(
    `${NOMINATIM}/reverse?format=json&lat=${lat}&lon=${lng}`,
  );
  if (!res.ok) return null;
  const data = (await res.json()) as NominatimPlace;
  return data.display_name ?? null;
}
