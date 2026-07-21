import type { EventItem } from "./types";

const SITE_URL = process.env.NEXT_PUBLIC_SITE_URL ?? "http://localhost:3000";

/** Builds a schema.org Event JSON-LD object for search engine rich results. */
export function buildEventJsonLd(event: EventItem, locale: string) {
  const url = `${SITE_URL}/${locale}/events/${event.id}`;

  const location = event.isOnline
    ? { "@type": "VirtualLocation", url }
    : {
        "@type": "Place",
        name: event.locationName ?? event.citySlug ?? event.title,
        address: {
          "@type": "PostalAddress",
          ...(event.address ? { streetAddress: event.address } : {}),
          ...(event.citySlug ? { addressLocality: event.citySlug } : {}),
          addressCountry: "UZ",
        },
      };

  return {
    "@context": "https://schema.org",
    "@type": "Event",
    name: event.title,
    startDate: event.startsAt,
    ...(event.endsAt ? { endDate: event.endsAt } : {}),
    eventStatus: `https://schema.org/${
      event.status === "canceled" ? "EventCancelled" : "EventScheduled"
    }`,
    eventAttendanceMode: `https://schema.org/${
      event.isOnline ? "OnlineEventAttendanceMode" : "OfflineEventAttendanceMode"
    }`,
    location,
    ...(event.description ? { description: event.description } : {}),
    ...(event.coverUrl ? { image: [event.coverUrl] } : {}),
    organizer: { "@type": "Organization", name: event.organizerName },
    // Every event on the platform is free in v1 — see docs/roadmap.md.
    offers: {
      "@type": "Offer",
      price: "0",
      priceCurrency: "UZS",
      availability: `https://schema.org/${
        event.capacity !== null && event.goingCount >= event.capacity
          ? "SoldOut"
          : "InStock"
      }`,
      url,
    },
    url,
  };
}

/**
 * Serializes JSON-LD for embedding in a `<script>` tag. Event titles and
 * descriptions are organizer-controlled free text — plain JSON.stringify
 * doesn't escape `<`, so a description containing `</script>` could break
 * out of the tag and inject arbitrary markup. Escaping `<` as `<`
 * (valid inside a JSON string, inert as HTML) closes that off.
 */
export function stringifyJsonLd(data: unknown): string {
  return JSON.stringify(data).replace(/</g, "\\u003c");
}
