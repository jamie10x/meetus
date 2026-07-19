"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import EventCard from "./EventCard";
import { api } from "@/lib/api";
import type { TrendingEventItem } from "@/lib/types";

type Props = {
  /** Restrict to one city (slug), e.g. mirroring the Explore page's filter. */
  city?: string;
  limit?: number;
};

/**
 * Ranked by RSVP velocity (joins in the last 7 days), not lifetime total —
 * a quiet event with a sudden burst of interest outranks a stale one with
 * a bigger running count. Renders nothing if there's no signal yet, so it
 * never shows an awkward empty state on the home page.
 */
export default function TrendingSection({ city, limit = 6 }: Props) {
  const t = useTranslations("trending");
  const [items, setItems] = useState<TrendingEventItem[] | null>(null);

  useEffect(() => {
    const params = new URLSearchParams({ limit: String(limit) });
    if (city) params.set("city", city);
    let stale = false;
    api<TrendingEventItem[]>(`/explore/trending?${params}`)
      .then((data) => {
        if (!stale) setItems(data);
      })
      .catch(() => {
        if (!stale) setItems([]);
      });
    return () => {
      stale = true;
    };
  }, [city, limit]);

  if (!items || items.length === 0) return null;

  return (
    <section className="mb-12">
      <div className="mb-2 flex items-center gap-2 font-mono text-xs font-medium uppercase tracking-[0.14em] text-registan-strong">
        <span className="h-1.5 w-1.5 rounded-full bg-registan-strong shadow-[0_0_0_3px_rgba(24,173,160,0.16)]" />
        {t("title")}
      </div>
      <div className="-mx-1 flex snap-x snap-proximity gap-4 overflow-x-auto px-1 pb-3 [scrollbar-width:thin]">
        {items.map((e) => (
          <div key={e.id} className="w-[268px] shrink-0 snap-start">
            <EventCard event={e} badge={t("badge", { count: e.recentGoing })} />
          </div>
        ))}
      </div>
    </section>
  );
}
