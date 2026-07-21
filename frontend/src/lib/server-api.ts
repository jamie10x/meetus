import { cache } from "react";
import { API_URL } from "./api";
import type { EventItem, TrendingEventItem } from "./types";

/**
 * Server-side fetch of one event, deduplicated per request so the page
 * and generateMetadata share a single call.
 */
export const fetchEvent = cache(async (id: string): Promise<EventItem | null> => {
  const res = await fetch(`${API_URL}/api/explore/events/${id}`, {
    cache: "no-store",
  });
  if (!res.ok) return null;
  const body = await res.json();
  return body.data as EventItem;
});

/** Server-side fetch of events related to one event (same category/city). */
export const fetchRelatedEvents = cache(
  async (id: string): Promise<EventItem[]> => {
    const res = await fetch(`${API_URL}/api/explore/events/${id}/related`, {
      cache: "no-store",
    });
    if (!res.ok) return [];
    const body = await res.json();
    return body.data as EventItem[];
  },
);

/** Server-side fetch of the other published upcoming dates in the same
 *  weekly series as one event. Empty when the event isn't part of a series. */
export const fetchSeriesEvents = cache(
  async (id: string): Promise<EventItem[]> => {
    const res = await fetch(`${API_URL}/api/explore/events/${id}/series`, {
      cache: "no-store",
    });
    if (!res.ok) return [];
    const body = await res.json();
    return body.data as EventItem[];
  },
);

/** Server-side trending fetch, used for the home page hero's ticket card. */
export const fetchTrending = cache(
  async (limit: number): Promise<TrendingEventItem[]> => {
    const res = await fetch(`${API_URL}/api/explore/trending?limit=${limit}`, {
      cache: "no-store",
    });
    if (!res.ok) return [];
    const body = await res.json();
    return body.data as TrendingEventItem[];
  },
);
