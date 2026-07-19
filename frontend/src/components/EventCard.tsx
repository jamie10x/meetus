import { useLocale, useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
import type { EventItem } from "@/lib/types";
import { categoryCoverStyle, categoryLabelClass } from "@/lib/categoryStyle";

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
      className="group flex h-full flex-col overflow-hidden rounded-card border border-line bg-ink-raised shadow-card transition-[transform,border-color,box-shadow] duration-300 hover:-translate-y-1 hover:border-registan-dim hover:shadow-[0_26px_50px_-22px_rgba(24,173,160,0.35)]"
    >
      <div
        className="relative overflow-hidden transition-transform duration-500 group-hover:scale-[1.04]"
        style={{ height: 132, ...categoryCoverStyle(event.categorySlug) }}
      >
        <span
          className={`absolute left-2.5 top-2.5 rounded-full border border-bone/20 bg-ink/55 px-2.5 py-1 font-mono text-[0.64rem] uppercase tracking-wider backdrop-blur-sm ${categoryLabelClass(event.categorySlug)}`}
        >
          {event.categorySlug}
        </span>
        {badge ? (
          <span className="absolute bottom-2.5 right-2.5 rounded-full border border-registan-strong/35 bg-ink/60 px-2 py-1 font-mono text-[0.66rem] font-medium text-registan-strong">
            {badge}
          </span>
        ) : null}
      </div>
      <div className="flex flex-1 flex-col gap-2.5 p-4">
        <p className="text-xs font-medium text-registan-strong">
          {formatEventDate(event.startsAt, locale)}
        </p>
        <h3 className="line-clamp-2 font-sans text-base font-bold leading-snug text-bone">
          {event.title}
        </h3>
        <p className="truncate text-sm text-dust">
          {event.isOnline
            ? tExplore("online")
            : (event.locationName ?? event.citySlug ?? "")}
          <span className="text-dust-dim"> · </span>
          {event.organizerName}
        </p>
        <div className="mt-auto flex items-center justify-between border-t border-bone/[0.09] pt-2.5">
          <span className="font-mono text-xs text-dust">
            {t("going", { count: event.goingCount })}
            {event.capacity
              ? ` · ${t("spotsLeft", { count: event.capacity - event.goingCount })}`
              : ""}
          </span>
        </div>
      </div>
    </Link>
  );
}
