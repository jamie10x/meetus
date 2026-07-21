"use client";

import { useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import dynamic from "next/dynamic";
import EventCard from "@/components/EventCard";
import TrendingSection from "@/components/TrendingSection";
import { api } from "@/lib/api";
import { metaName, type EventItem, type MetaItem } from "@/lib/types";

// Leaflet touches `window` at import time, so the map view can only ever
// render client-side — ssr: false keeps it out of the server bundle
// entirely rather than erroring during SSR.
const EventMap = dynamic(() => import("@/components/EventMap"), {
  ssr: false,
  loading: () => (
    <div
      style={{ height: 520 }}
      className="flex w-full items-center justify-center rounded-card border border-line bg-ink-raised text-sm text-dust"
    />
  ),
});

type Page = { items: EventItem[]; nextCursor: string | null };

type DatePreset = "all" | "today" | "tomorrow" | "week";

function presetRange(preset: DatePreset): { from?: string; to?: string } {
  const now = new Date();
  const startOfDay = (d: Date) =>
    new Date(d.getFullYear(), d.getMonth(), d.getDate());
  const endOfDay = (d: Date) =>
    new Date(d.getFullYear(), d.getMonth(), d.getDate(), 23, 59, 59);

  switch (preset) {
    case "today":
      return { from: now.toISOString(), to: endOfDay(now).toISOString() };
    case "tomorrow": {
      const t = new Date(now);
      t.setDate(t.getDate() + 1);
      return {
        from: startOfDay(t).toISOString(),
        to: endOfDay(t).toISOString(),
      };
    }
    case "week": {
      const end = new Date(now);
      end.setDate(end.getDate() + 7);
      return { from: now.toISOString(), to: endOfDay(end).toISOString() };
    }
    default:
      return {};
  }
}

const selectCls =
  "rounded-full border border-line bg-ink-raised px-4 py-2 text-sm font-medium text-dust transition-all hover:text-bone focus:border-registan-dim focus:text-bone focus:outline-none focus:ring-2 focus:ring-registan/20";
const chipCls = (active: boolean) =>
  `shrink-0 whitespace-nowrap rounded-full border px-4 py-2 text-sm font-semibold transition-all ${
    active
      ? "border-registan bg-gradient-to-br from-registan to-registan-dim text-[#f8fbff] shadow-[0_6px_18px_-8px_rgba(47,111,235,0.7)]"
      : "border-line bg-ink-raised text-dust hover:border-registan-dim hover:text-bone"
  }`;

export default function ExplorePage() {
  const t = useTranslations("explore");
  const locale = useLocale();
  const [cities, setCities] = useState<MetaItem[]>([]);
  const [categories, setCategories] = useState<MetaItem[]>([]);

  const [city, setCity] = useState("");
  const [category, setCategory] = useState("");
  const [preset, setPreset] = useState<DatePreset>("all");
  const [online, setOnline] = useState("");
  const [q, setQ] = useState("");
  const [search, setSearch] = useState("");

  const [items, setItems] = useState<EventItem[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [failed, setFailed] = useState(false);
  const [view, setView] = useState<"list" | "map">("list");

  useEffect(() => {
    api<MetaItem[]>("/meta/cities").then(setCities).catch(() => {});
    api<MetaItem[]>("/meta/categories").then(setCategories).catch(() => {});
  }, []);

  // Debounce free-text search.
  useEffect(() => {
    const timer = setTimeout(() => setSearch(q), 350);
    return () => clearTimeout(timer);
  }, [q]);

  const buildQuery = useCallback(
    (cursor?: string) => {
      const params = new URLSearchParams();
      if (city) params.set("city", city);
      if (category) params.set("category", category);
      if (online) params.set("online", online);
      if (search) params.set("q", search);
      const { from, to } = presetRange(preset);
      if (from) params.set("from", from);
      if (to) params.set("to", to);
      if (cursor) params.set("cursor", cursor);
      return params.toString();
    },
    [city, category, online, search, preset],
  );

  useEffect(() => {
    let stale = false;
    setLoading(true);
    setFailed(false);
    api<Page>(`/explore/events?${buildQuery()}`)
      .then((page) => {
        if (stale) return;
        setItems(page.items);
        setNextCursor(page.nextCursor);
      })
      .catch(() => {
        if (!stale) setFailed(true);
      })
      .finally(() => {
        if (!stale) setLoading(false);
      });
    return () => {
      stale = true;
    };
  }, [buildQuery]);

  const loadMore = async () => {
    if (!nextCursor) return;
    const page = await api<Page>(
      `/explore/events?${buildQuery(nextCursor)}`,
    ).catch(() => null);
    if (page) {
      setItems((prev) => [...prev, ...page.items]);
      setNextCursor(page.nextCursor);
    }
  };

  return (
    <main>
      <div className="mx-auto max-w-6xl px-5 pb-4 pt-12">
        <h1 className="text-[clamp(2.1rem,3vw+1rem,3rem)] font-black text-bone">
          {t("title")}
        </h1>
      </div>

      <div className="sticky top-16 z-20 border-b border-bone/[0.09] bg-ink/95 py-4 backdrop-blur">
        <div className="mx-auto flex max-w-6xl flex-col gap-3 px-5">
          <div className="flex flex-wrap gap-3">
            <input
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder={t("searchPlaceholder")}
              className={`${selectCls} min-w-56 flex-1 rounded-2xl`}
            />
            <select value={city} onChange={(e) => setCity(e.target.value)} className={selectCls}>
              <option value="">{t("allCities")}</option>
              {cities.map((c) => (
                <option key={c.id} value={c.slug}>
                  {metaName(c, locale)}
                </option>
              ))}
            </select>
          </div>

          <div className="-mx-5 flex gap-2 overflow-x-auto px-5 pb-1 [scrollbar-width:none]">
            <button onClick={() => setCategory("")} className={chipCls(category === "")}>
              {t("allCategories")}
            </button>
            {categories.map((c) => (
              <button
                key={c.id}
                onClick={() => setCategory(c.slug)}
                className={chipCls(category === c.slug)}
              >
                {metaName(c, locale)}
              </button>
            ))}
          </div>

          <div className="-mx-5 flex gap-2 overflow-x-auto px-5 pb-1 [scrollbar-width:none]">
            {(
              [
                ["all", t("anyDate")],
                ["today", t("today")],
                ["tomorrow", t("tomorrow")],
                ["week", t("thisWeek")],
              ] as [DatePreset, string][]
            ).map(([value, label]) => (
              <button
                key={value}
                onClick={() => setPreset(value)}
                className={chipCls(preset === value)}
              >
                {label}
              </button>
            ))}
            <span className="mx-1 w-px shrink-0 self-stretch bg-line" />
            {[
              ["", t("onlineAndOffline")],
              ["false", t("inPerson")],
              ["true", t("online")],
            ].map(([value, label]) => (
              <button
                key={value}
                onClick={() => setOnline(value)}
                className={chipCls(online === value)}
              >
                {label}
              </button>
            ))}
          </div>
        </div>
      </div>

      <div className="mx-auto max-w-6xl px-5 py-10">
        <TrendingSection city={city} />

        {loading ? (
          <p className="py-16 text-center text-dust">{t("loading")}</p>
        ) : failed ? (
          <p className="py-16 text-center text-pomegranate">{t("loadFailed")}</p>
        ) : items.length === 0 ? (
          <p className="py-16 text-center text-dust">{t("noResults")}</p>
        ) : (
          <>
            <div className="mb-5 flex justify-end gap-2">
              <button
                onClick={() => setView("list")}
                className={chipCls(view === "list")}
              >
                {t("listView")}
              </button>
              <button
                onClick={() => setView("map")}
                className={chipCls(view === "map")}
              >
                {t("mapView")}
              </button>
            </div>

            {view === "map" ? (
              <EventMap events={items} />
            ) : (
              <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-3">
                {items.map((e) => (
                  <EventCard key={e.id} event={e} />
                ))}
              </div>
            )}

            {nextCursor ? (
              <button onClick={loadMore} className="btn btn-secondary mx-auto mt-8 block">
                {t("loadMore")}
              </button>
            ) : null}
          </>
        )}
      </div>
    </main>
  );
}
