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
    <section className="mb-8">
      <h2 className="mb-4 flex items-center gap-2 text-xl font-bold">
        🔥 {t("title")}
      </h2>
      <div className="flex flex-col gap-3">
        {items.map((e) => (
          <EventCard
            key={e.id}
            event={e}
            badge={t("badge", { count: e.recentGoing })}
          />
        ))}
      </div>
    </section>
  );
}
