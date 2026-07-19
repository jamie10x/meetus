import { useLocale, useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
import type { EventItem } from "@/lib/types";

export function formatEventDate(iso: string, locale: string): string {
  return new Date(iso).toLocaleString(locale, {
    weekday: "short",
    day: "numeric",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
  });
}

type Props = {
  event: EventItem;
  /** Optional corner badge, e.g. a trending indicator. */
  badge?: string;
};

export default function EventCard({ event, badge }: Props) {
  const t = useTranslations("eventCard");
  const tExplore = useTranslations("explore");
  const locale = useLocale();

  return (
    <Link
      href={`/events/${event.id}`}
      className="relative flex gap-4 rounded-xl border border-zinc-200 p-4 transition-colors hover:border-sky-400 dark:border-zinc-800"
    >
      {badge ? (
        <span className="absolute -top-2 right-3 rounded-full bg-orange-500 px-2.5 py-0.5 text-xs font-medium text-white shadow-sm">
          {badge}
        </span>
      ) : null}
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
          {formatEventDate(event.startsAt, locale)}
        </p>
        <h3 className="truncate text-lg font-semibold">{event.title}</h3>
        <p className="truncate text-sm text-zinc-500">
          {event.isOnline
            ? tExplore("online")
            : (event.locationName ?? event.citySlug ?? "")}
          {" · "}
          {event.organizerName}
        </p>
        <p className="mt-1 text-xs text-zinc-400">
          {t("going", { count: event.goingCount })}
          {event.capacity
            ? ` · ${t("spotsLeft", { count: event.capacity - event.goingCount })}`
            : ""}
        </p>
      </div>
    </Link>
  );
}
