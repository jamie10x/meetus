import type { MetadataRoute } from "next";
import { API_URL } from "@/lib/api";
import { routing } from "@/i18n/routing";
import type { EventItem } from "@/lib/types";

const SITE_URL = process.env.NEXT_PUBLIC_SITE_URL ?? "http://localhost:3000";

// Google's own per-sitemap cap is 50,000 URLs; this is a much smaller
// safety cap on how many published-event pages/API pages we're willing
// to walk in one build — comfortably above anything this platform is
// realistically going to list, without risking an unbounded fetch loop.
const MAX_EVENT_PAGES = 40;
const PAGE_SIZE = 50;

// The API isn't reachable during `next build` itself (CI builds the
// frontend in isolation, and the production Docker image builds each
// service separately with no live backend on the network) — only once
// the built app is actually running. A thrown fetch (ECONNREFUSED, DNS
// failure, etc.) must not fail the build; it just means the sitemap ships
// with the static entries only, until ISR revalidates it against a live
// API (see the fetch `revalidate` below) after deploy.
async function fetchAllPublishedEvents(): Promise<EventItem[]> {
  const events: EventItem[] = [];
  let cursor: string | undefined;

  try {
    for (let page = 0; page < MAX_EVENT_PAGES; page++) {
      const params = new URLSearchParams({ limit: String(PAGE_SIZE) });
      if (cursor) params.set("cursor", cursor);

      const res = await fetch(`${API_URL}/api/explore/events?${params}`, {
        next: { revalidate: 3600 },
      });
      if (!res.ok) break;

      const body = await res.json();
      const items: EventItem[] = body.data?.items ?? [];
      events.push(...items);

      cursor = body.data?.nextCursor ?? undefined;
      if (!cursor) break;
    }
  } catch {
    return [];
  }
  return events;
}

function localizedEntry(
  path: string,
  lastModified: string | Date,
): MetadataRoute.Sitemap[number] {
  const alternates = Object.fromEntries(
    routing.locales.map((l) => [l, `${SITE_URL}/${l}${path}`]),
  );
  return {
    url: `${SITE_URL}/${routing.defaultLocale}${path}`,
    lastModified,
    alternates: { languages: alternates },
  };
}

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const now = new Date();
  const staticEntries: MetadataRoute.Sitemap = [
    { ...localizedEntry("", now), changeFrequency: "daily", priority: 1 },
    { ...localizedEntry("/events", now), changeFrequency: "hourly", priority: 0.9 },
    { ...localizedEntry("/privacy", now), changeFrequency: "yearly", priority: 0.2 },
  ];

  const events = await fetchAllPublishedEvents();
  // createdAt, not startsAt: this is "when the page's content last
  // changed," not "when the event happens" — the DTO doesn't expose an
  // updatedAt, so createdAt is the closest available signal.
  const eventEntries: MetadataRoute.Sitemap = events.map((e) => ({
    ...localizedEntry(`/events/${e.id}`, e.createdAt),
    changeFrequency: "daily",
    priority: 0.7,
  }));

  return [...staticEntries, ...eventEntries];
}
