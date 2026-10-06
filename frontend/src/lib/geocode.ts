/** Submitted address queries through the authenticated, throttled server proxy. */
import { api } from "./api";

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
  const data = await api<NominatimPlace[]>(`/geocode/search?q=${encodeURIComponent(query)}`, { auth: true });
  return data.flatMap((p) => {
    if (!p.display_name || !p.lat || !p.lon) return [];
    return [{ label: p.display_name, lat: parseFloat(p.lat), lng: parseFloat(p.lon) }];
  });
}

/** Reverse lookup: coordinates → a street address, or null if the point
 * has nothing mapped near it (open country, a lake, mid-ocean). */
export async function reverseGeocode(lat: number, lng: number): Promise<string | null> {
  const data = await api<NominatimPlace>(`/geocode/reverse?lat=${lat}&lon=${lng}`, { auth: true });
  return data.display_name ?? null;
}
