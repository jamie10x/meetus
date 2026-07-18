import { cache } from "react";
import { API_URL } from "./api";
import type { EventItem } from "./types";

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
