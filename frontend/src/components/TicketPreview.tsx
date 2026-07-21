import { getLocale, getTranslations } from "next-intl/server";
import { Link } from "@/i18n/navigation";
import { formatEventDate } from "./EventCard";
import { categoryCoverStyle } from "@/lib/categoryStyle";
import type { TrendingEventItem } from "@/lib/types";

/**
 * Decorative ticket-stub card for the home hero — the same visual device
 * as an actual QR ticket, applied to whatever event is trending right now.
 * Not interactive; the whole card links to the event page.
 */
export default async function TicketPreview({
  event,
}: {
  event: TrendingEventItem;
}) {
  const t = await getTranslations("home");
  const tEventCard = await getTranslations("eventCard");
  const tExplore = await getTranslations("explore");
  const locale = await getLocale();

  return (
    <Link
      href={`/events/${event.id}`}
      className="relative block overflow-hidden rounded-[20px] border border-line bg-ink-raised shadow-pop transition-transform duration-300 hover:-translate-y-1"
      style={{
        clipPath:
          "polygon(0 0, 100% 0, 100% 100%, 28px 100%, 0 calc(100% - 28px))",
      }}
    >
      <div className="relative h-36" style={categoryCoverStyle(event.categorySlug)}>
        <span className="absolute left-4 top-3.5 rounded-full border border-bone/25 bg-ink/55 px-2.5 py-1.5 font-mono text-[0.68rem] uppercase tracking-wider text-bone backdrop-blur-sm">
          {t("ticketPreviewLabel")}
        </span>
      </div>

      <div className="px-6 pt-5">
        <h3 className="text-xl font-bold text-bone">{event.title}</h3>
        <ul className="mt-2.5 flex flex-col gap-1.5">
          <li className="flex items-baseline gap-2.5 text-sm text-dust">
            <b className="font-mono text-[0.82rem] font-semibold text-bone">
              {formatEventDate(event.startsAt, locale)}
            </b>
          </li>
          <li className="flex items-baseline gap-2.5 text-sm text-dust">
            {event.isOnline
              ? tExplore("online")
              : (event.locationName ?? event.citySlug ?? "")}
          </li>
        </ul>
      </div>

      <div className="relative my-5 h-0 border-t-2 border-dashed border-line">
        <span className="absolute -left-[31px] -top-[9px] h-[18px] w-[18px] rounded-full bg-ink" />
        <span className="absolute -right-[31px] -top-[9px] h-[18px] w-[18px] rounded-full bg-ink" />
      </div>

      <div className="flex items-center justify-between px-6 pb-6">
        <span className="font-mono text-xs text-dust">
          {tEventCard("going", { count: event.goingCount })}
        </span>
        <span className="btn btn-primary btn-sm">
          {t("ticketPreviewCta")}
        </span>
      </div>
    </Link>
  );
}
