import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { fetchEvent } from "@/lib/server-api";
import { formatEventDate } from "@/components/EventCard";
import RsvpSection from "@/components/RsvpSection";

type Props = { params: Promise<{ id: string }> };

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { id } = await params;
  const event = await fetchEvent(id);
  if (!event) return { title: "Event not found" };

  const description =
    event.description.slice(0, 160) ||
    `${formatEventDate(event.startsAt)} · ${event.organizerName}`;

  return {
    title: event.title,
    description,
    openGraph: {
      title: event.title,
      description,
      type: "article",
      ...(event.coverUrl ? { images: [{ url: event.coverUrl }] } : {}),
    },
  };
}

export default async function EventDetailPage({ params }: Props) {
  const { id } = await params;
  const event = await fetchEvent(id);
  if (!event) notFound();

  const spotsLeft =
    event.capacity !== null ? event.capacity - event.goingCount : null;

  return (
    <main className="mx-auto max-w-3xl px-4 py-8">
      {event.coverUrl ? (
        // eslint-disable-next-line @next/next/no-img-element
        <img
          src={event.coverUrl}
          alt=""
          className="mb-6 max-h-80 w-full rounded-2xl object-cover"
        />
      ) : null}

      {event.status !== "published" ? (
        <p className="mb-4 rounded-lg bg-red-50 p-3 text-sm font-medium text-red-700 dark:bg-red-950 dark:text-red-300">
          This event has been {event.status}.
        </p>
      ) : null}

      <p className="text-sm font-medium text-sky-600 dark:text-sky-400">
        {formatEventDate(event.startsAt)}
        {event.endsAt ? ` – ${formatEventDate(event.endsAt)}` : ""}
      </p>
      <h1 className="mt-1 text-3xl font-bold">{event.title}</h1>
      <p className="mt-2 text-zinc-500">
        Hosted by <span className="font-medium">{event.organizerName}</span>
      </p>

      <div className="mt-4 flex flex-wrap gap-2 text-sm">
        <span className="rounded-full bg-zinc-100 px-3 py-1 dark:bg-zinc-800">
          {event.categorySlug}
        </span>
        <span className="rounded-full bg-zinc-100 px-3 py-1 dark:bg-zinc-800">
          {event.isOnline ? "Online" : (event.citySlug ?? "In person")}
        </span>
        <span className="rounded-full bg-zinc-100 px-3 py-1 dark:bg-zinc-800">
          {event.goingCount} going
          {spotsLeft !== null ? ` · ${spotsLeft} spots left` : ""}
        </span>
      </div>

      {event.status === "published" ? (
        <RsvpSection
          eventId={event.id}
          spotsLeft={spotsLeft}
          isPast={new Date(event.startsAt) <= new Date()}
        />
      ) : null}

      {!event.isOnline && (event.locationName || event.address) ? (
        <div className="mt-6 rounded-xl border border-zinc-200 p-4 dark:border-zinc-800">
          <h2 className="font-semibold">Location</h2>
          <p className="text-zinc-600 dark:text-zinc-300">
            {[event.locationName, event.address, event.district]
              .filter(Boolean)
              .join(" · ")}
          </p>
        </div>
      ) : null}

      {event.description ? (
        <div className="mt-6">
          <h2 className="mb-2 font-semibold">About this event</h2>
          <p className="whitespace-pre-wrap text-zinc-600 dark:text-zinc-300">
            {event.description}
          </p>
        </div>
      ) : null}
    </main>
  );
}
