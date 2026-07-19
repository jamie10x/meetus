import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { getTranslations } from "next-intl/server";
import { fetchEvent } from "@/lib/server-api";
import { formatEventDate } from "@/components/EventCard";
import RsvpSection from "@/components/RsvpSection";

type Props = { params: Promise<{ id: string; locale: string }> };

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { id, locale } = await params;
  const t = await getTranslations({ locale, namespace: "eventDetail" });
  const event = await fetchEvent(id);
  if (!event) return { title: t("notFoundTitle") };

  const description =
    event.description.slice(0, 160) ||
    `${formatEventDate(event.startsAt, locale)} · ${event.organizerName}`;

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
  const { id, locale } = await params;
  const t = await getTranslations({ locale, namespace: "eventDetail" });
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
          {t("statusNotice", { status: event.status })}
        </p>
      ) : null}

      <p className="text-sm font-medium text-sky-600 dark:text-sky-400">
        {formatEventDate(event.startsAt, locale)}
        {event.endsAt ? ` – ${formatEventDate(event.endsAt, locale)}` : ""}
      </p>
      <h1 className="mt-1 text-3xl font-bold">{event.title}</h1>
      <p className="mt-2 text-zinc-500">
        {t("hostedBy", { name: event.organizerName })}
      </p>

      <div className="mt-4 flex flex-wrap gap-2 text-sm">
        <span className="rounded-full bg-zinc-100 px-3 py-1 dark:bg-zinc-800">
          {event.categorySlug}
        </span>
        <span className="rounded-full bg-zinc-100 px-3 py-1 dark:bg-zinc-800">
          {event.isOnline ? t("online") : (event.citySlug ?? t("inPerson"))}
        </span>
        <span className="rounded-full bg-zinc-100 px-3 py-1 dark:bg-zinc-800">
          {t("goingCount", { count: event.goingCount })}
          {spotsLeft !== null ? ` ${t("spotsLeft", { count: spotsLeft })}` : ""}
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
          <h2 className="font-semibold">{t("location")}</h2>
          <p className="text-zinc-600 dark:text-zinc-300">
            {[event.locationName, event.address, event.district]
              .filter(Boolean)
              .join(" · ")}
          </p>
        </div>
      ) : null}

      {event.description ? (
        <div className="mt-6">
          <h2 className="mb-2 font-semibold">{t("about")}</h2>
          <p className="whitespace-pre-wrap text-zinc-600 dark:text-zinc-300">
            {event.description}
          </p>
        </div>
      ) : null}
    </main>
  );
}
