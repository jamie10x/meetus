/**
 * Client-side ICS (.ics) generation for "Add to calendar" — no backend
 * endpoint needed, the browser already has everything it needs to build
 * one from an event's fields.
 */

export type IcsEvent = {
  title: string;
  description: string;
  startsAt: string; // ISO
  endsAt: string | null;
  isOnline: boolean;
  locationName: string | null;
  address: string | null;
  citySlug: string | null;
  /** Absolute URL to the event page; omit to leave the ICS URL/description link blank. */
  webUrl?: string;
};

// Meetups with no explicit end time default to a 2-hour block — long
// enough to be a sane calendar placeholder, short enough not to eat the
// rest of the day.
const DEFAULT_DURATION_MS = 2 * 60 * 60 * 1000;

function toIcsDate(iso: string): string {
  return new Date(iso).toISOString().replace(/[-:]/g, "").replace(/\.\d{3}Z$/, "Z");
}

// RFC 5545 text escaping: backslash, semicolon, comma, and newlines.
// Line folding at 75 octets is deliberately skipped — every mainstream
// calendar app tolerates long unfolded lines, and folding correctly
// requires UTF-8-aware octet counting that isn't worth the complexity here.
function escapeIcsText(s: string): string {
  return s
    .replace(/\\/g, "\\\\")
    .replace(/;/g, "\\;")
    .replace(/,/g, "\\,")
    .replace(/\n/g, "\\n");
}

function icsLocation(e: IcsEvent): string {
  if (e.isOnline) return "Online";
  return [e.locationName, e.address, e.citySlug].filter(Boolean).join(", ");
}

function icsRange(e: IcsEvent): { start: Date; end: Date } {
  const start = new Date(e.startsAt);
  const end = e.endsAt ? new Date(e.endsAt) : new Date(start.getTime() + DEFAULT_DURATION_MS);
  return { start, end };
}

/** Builds the raw .ics file content (CRLF line endings per RFC 5545). */
export function buildIcsContent(e: IcsEvent): string {
  const { start, end } = icsRange(e);
  const uid = `${start.getTime()}-${e.title.length}@meetus.uz`;

  const lines = [
    "BEGIN:VCALENDAR",
    "VERSION:2.0",
    "PRODID:-//Meetus.uz//Events//EN",
    "CALSCALE:GREGORIAN",
    "BEGIN:VEVENT",
    `UID:${uid}`,
    `DTSTAMP:${toIcsDate(new Date().toISOString())}`,
    `DTSTART:${toIcsDate(start.toISOString())}`,
    `DTEND:${toIcsDate(end.toISOString())}`,
    `SUMMARY:${escapeIcsText(e.title)}`,
    ...(e.description ? [`DESCRIPTION:${escapeIcsText(e.description)}`] : []),
    `LOCATION:${escapeIcsText(icsLocation(e))}`,
    ...(e.webUrl ? [`URL:${e.webUrl}`] : []),
    "END:VEVENT",
    "END:VCALENDAR",
  ];
  return lines.join("\r\n");
}

/**
 * Triggers a browser download of the .ics file via a Blob object URL.
 * Deliberately not a plain `<a href="data:...">` — some webviews
 * (including Telegram's in-app browser, which this site also runs
 * inside as a Mini App) navigate to `data:` URLs instead of downloading
 * them, even with the `download` attribute set. An object URL, clicked
 * through a detached anchor, downloads reliably everywhere and is
 * revoked immediately after.
 */
export function downloadIcs(e: IcsEvent, filename: string): void {
  const blob = new Blob([buildIcsContent(e)], { type: "text/calendar;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

/** Google Calendar's "quick add" template link — no API key needed. */
export function googleCalendarUrl(e: IcsEvent): string {
  const { start, end } = icsRange(e);
  const params = new URLSearchParams({
    action: "TEMPLATE",
    text: e.title,
    dates: `${toIcsDate(start.toISOString())}/${toIcsDate(end.toISOString())}`,
    details: e.webUrl ? `${e.description}\n\n${e.webUrl}` : e.description,
    location: icsLocation(e),
  });
  return `https://calendar.google.com/calendar/render?${params.toString()}`;
}

/** A filesystem-safe .ics filename derived from the event title. */
export function icsFilename(title: string): string {
  const slug = title
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
  return `${slug || "event"}.ics`;
}
