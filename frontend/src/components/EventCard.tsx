import Link from "next/link";
import type { EventItem } from "@/lib/types";

export function formatEventDate(iso: string): string {
  return new Date(iso).toLocaleString(undefined, {
    weekday: "short",
    day: "numeric",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export default function EventCard({ event }: { event: EventItem }) {
  return (
    <Link
      href={`/events/${event.id}`}
      className="flex gap-4 rounded-xl border border-zinc-200 p-4 transition-colors hover:border-sky-400 dark:border-zinc-800"
    >
      {event.coverUrl ? (
        // eslint-disable-next-line @next/next/no-img-element
        <img
          src={event.coverUrl}
          alt=""
          className="h-24 w-32 shrink-0 rounded-lg object-cover"
        />
      ) : (
        <div className="flex h-24 w-32 shrink-0 items-center justify-center rounded-lg bg-gradient-to-br from-sky-400 to-indigo-500 text-3xl">
          🎟️
        </div>
      )}
      <div className="min-w-0">
        <p className="text-sm font-medium text-sky-600 dark:text-sky-400">
          {formatEventDate(event.startsAt)}
        </p>
        <h3 className="truncate text-lg font-semibold">{event.title}</h3>
        <p className="truncate text-sm text-zinc-500">
          {event.isOnline
            ? "Online"
            : (event.locationName ?? event.citySlug ?? "")}
          {" · "}
          {event.organizerName}
        </p>
        <p className="mt-1 text-xs text-zinc-400">
          {event.goingCount} going
          {event.capacity ? ` · ${event.capacity - event.goingCount} spots left` : ""}
        </p>
      </div>
    </Link>
  );
}
