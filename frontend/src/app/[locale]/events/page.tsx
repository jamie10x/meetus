"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import EventCard from "@/components/EventCard";
import { api } from "@/lib/api";
import type { EventItem, MetaItem } from "@/lib/types";

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
  "rounded-lg border border-zinc-300 px-3 py-2 text-sm dark:border-zinc-700 dark:bg-zinc-900";

export default function ExplorePage() {
  const t = useTranslations("explore");
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
    <main className="mx-auto max-w-3xl px-4 py-8">
      <h1 className="mb-6 text-3xl font-bold">{t("title")}</h1>

      <div className="mb-6 flex flex-wrap gap-2">
        <input
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder={t("searchPlaceholder")}
          className={`${selectCls} w-full sm:w-64`}
        />
        <select value={city} onChange={(e) => setCity(e.target.value)} className={selectCls}>
          <option value="">{t("allCities")}</option>
          {cities.map((c) => (
            <option key={c.id} value={c.slug}>
              {c.nameEn}
            </option>
          ))}
        </select>
        <select
          value={category}
          onChange={(e) => setCategory(e.target.value)}
          className={selectCls}
        >
          <option value="">{t("allCategories")}</option>
          {categories.map((c) => (
            <option key={c.id} value={c.slug}>
              {c.nameEn}
            </option>
          ))}
        </select>
        <select
          value={preset}
          onChange={(e) => setPreset(e.target.value as DatePreset)}
          className={selectCls}
        >
          <option value="all">{t("anyDate")}</option>
          <option value="today">{t("today")}</option>
          <option value="tomorrow">{t("tomorrow")}</option>
          <option value="week">{t("thisWeek")}</option>
        </select>
        <select
          value={online}
          onChange={(e) => setOnline(e.target.value)}
          className={selectCls}
        >
          <option value="">{t("onlineAndOffline")}</option>
          <option value="false">{t("inPerson")}</option>
          <option value="true">{t("online")}</option>
        </select>
      </div>

      {loading ? (
        <p className="py-16 text-center text-zinc-500">{t("loading")}</p>
      ) : failed ? (
        <p className="py-16 text-center text-red-500">{t("loadFailed")}</p>
      ) : items.length === 0 ? (
        <p className="py-16 text-center text-zinc-500">{t("noResults")}</p>
      ) : (
        <>
          <div className="flex flex-col gap-3">
            {items.map((e) => (
              <EventCard key={e.id} event={e} />
            ))}
          </div>
          {nextCursor ? (
            <button
              onClick={loadMore}
              className="mx-auto mt-6 block rounded-lg border border-zinc-300 px-5 py-2 text-sm font-medium hover:border-sky-500 hover:text-sky-500 dark:border-zinc-700"
            >
              {t("loadMore")}
            </button>
          ) : null}
        </>
      )}
    </main>
  );
}
