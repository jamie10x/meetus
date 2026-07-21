import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { getTranslations } from "next-intl/server";
import { fetchEvent, fetchRelatedEvents, fetchSeriesEvents } from "@/lib/server-api";
import EventCard, { formatEventDate } from "@/components/EventCard";
import { categoryCoverStyle } from "@/lib/categoryStyle";
import RsvpSection from "@/components/RsvpSection";
import VerifiedBadge from "@/components/VerifiedBadge";
import AddToCalendar from "@/components/AddToCalendar";
import { buildEventJsonLd, stringifyJsonLd } from "@/lib/eventSchema";

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
  const tCommon = await getTranslations({ locale, namespace: "common" });
  const event = await fetchEvent(id);
  if (!event) notFound();
  const related = await fetchRelatedEvents(id);
  const seriesEvents = event.seriesId ? await fetchSeriesEvents(id) : [];

  const spotsLeft =
    event.capacity !== null ? event.capacity - event.goingCount : null;

  const jsonLd = buildEventJsonLd(event, locale);

  return (
    <main className="mx-auto max-w-3xl px-5 py-10">
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: stringifyJsonLd(jsonLd) }}
      />
      <div
        className="mb-7 h-56 w-full overflow-hidden rounded-card border border-line sm:h-72"
        style={
          event.coverUrl
            ? undefined
            : categoryCoverStyle(event.categorySlug)
        }
      >
        {event.coverUrl ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={event.coverUrl}
            alt=""
            className="h-full w-full object-cover"
          />
        ) : null}
      </div>

      {event.status !== "published" ? (
        <p className="mb-5 rounded-card border border-pomegranate/35 bg-pomegranate/[0.12] p-3.5 text-sm font-medium text-pomegranate">
          {t("statusNotice", { status: event.status })}
        </p>
      ) : null}

      <p className="font-mono text-sm font-medium text-registan-strong">
        {formatEventDate(event.startsAt, locale)}
        {event.endsAt ? ` – ${formatEventDate(event.endsAt, locale)}` : ""}
      </p>
      <h1 className="mt-2 font-display text-3xl font-black text-bone sm:text-4xl">
        {event.title}
      </h1>
      <p className="mt-2.5 text-dust">
        {t("hostedBy", { name: event.organizerName })}
        {event.organizerVerified ? (
          <VerifiedBadge label={tCommon("verifiedOrganizer")} className="ml-1.5" />
        ) : null}
      </p>

      <div className="mt-5 flex flex-wrap gap-2 text-sm">
        <span className="rounded-full border border-line bg-ink-raised px-3.5 py-1.5 font-mono text-xs uppercase tracking-wide text-registan-strong">
          {event.categorySlug}
        </span>
        <span className="rounded-full border border-line bg-ink-raised px-3.5 py-1.5 text-dust">
          {event.isOnline ? t("online") : (event.citySlug ?? t("inPerson"))}
        </span>
        <span className="rounded-full border border-line bg-ink-raised px-3.5 py-1.5 text-dust">
          {t("goingCount", { count: event.goingCount })}
          {spotsLeft !== null ? ` ${t("spotsLeft", { count: spotsLeft })}` : ""}
        </span>
      </div>

      {event.status === "published" ? (
        <>
          <RsvpSection
            eventId={event.id}
            spotsLeft={spotsLeft}
            isPast={new Date(event.startsAt) <= new Date()}
          />
          <AddToCalendar
            className="mt-4"
            path={`/${locale}/events/${event.id}`}
            event={{
              title: event.title,
              description: event.description,
              startsAt: event.startsAt,
              endsAt: event.endsAt,
              isOnline: event.isOnline,
              locationName: event.locationName,
              address: event.address,
              citySlug: event.citySlug,
            }}
          />
        </>
      ) : null}

      {!event.isOnline && (event.locationName || event.address) ? (
        <div className="mt-8 rounded-card border border-line bg-ink-raised p-5">
          <h2 className="font-display font-bold text-bone">{t("location")}</h2>
          <p className="mt-1.5 text-dust">
            {[event.locationName, event.address, event.district]
              .filter(Boolean)
              .join(" · ")}
          </p>
        </div>
      ) : null}

      {event.description ? (
        <div className="mt-8">
          <h2 className="font-display font-bold text-bone">{t("about")}</h2>
          <p className="mt-2 whitespace-pre-wrap leading-relaxed text-dust">
            {event.description}
          </p>
        </div>
      ) : null}

      {seriesEvents.length > 0 ? (
        <div className="mt-10">
          <h2 className="font-display font-bold text-bone">{t("seriesHeading")}</h2>
          <div className="mt-3 grid grid-cols-1 gap-4 sm:grid-cols-2">
            {seriesEvents.map((se) => (
              <EventCard key={se.id} event={se} />
            ))}
          </div>
        </div>
      ) : null}

      {related.length > 0 ? (
        <div className="mt-10">
          <h2 className="font-display font-bold text-bone">{t("relatedHeading")}</h2>
          <div className="mt-3 grid grid-cols-1 gap-4 sm:grid-cols-2">
            {related.map((re) => (
              <EventCard key={re.id} event={re} />
            ))}
          </div>
        </div>
      ) : null}
    </main>
  );
}
