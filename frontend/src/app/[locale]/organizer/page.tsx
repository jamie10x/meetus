"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { Link, useRouter } from "@/i18n/navigation";
import { api, ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import type { EventItem, Organizer } from "@/lib/types";

type OrganizerStats = {
  totalEvents: number;
  upcomingPublished: number;
  totalRsvps: number;
  totalCheckins: number;
};

export default function OrganizerPage() {
  const t = useTranslations("organizer");
  const tEventCard = useTranslations("eventCard");
  const { user, loading } = useAuth();
  const router = useRouter();

  const [organizer, setOrganizer] = useState<Organizer | null>(null);
  const [events, setEvents] = useState<EventItem[]>([]);
  const [stats, setStats] = useState<OrganizerStats | null>(null);
  const [checked, setChecked] = useState(false);

  const [displayName, setDisplayName] = useState("");
  const [bio, setBio] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const statusStyle: Record<string, string> = {
    draft: "bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300",
    published: "bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300",
    canceled: "bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300",
    finished: "bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300",
  };
  const statusLabel: Record<string, string> = {
    draft: t("statusDraft"),
    published: t("statusPublished"),
    canceled: t("statusCanceled"),
    finished: t("statusFinished"),
  };

  useEffect(() => {
    if (!loading && !user) router.replace("/login");
  }, [loading, user, router]);

  useEffect(() => {
    if (!user) return;
    api<Organizer>("/organizers/me", { auth: true })
      .then(async (o) => {
        setOrganizer(o);
        setEvents(await api<EventItem[]>("/events/mine", { auth: true }));
        api<OrganizerStats>("/organizers/me/stats", { auth: true })
          .then(setStats)
          .catch(() => setStats(null));
      })
      .catch(() => setOrganizer(null))
      .finally(() => setChecked(true));
  }, [user]);

  if (loading || !user || !checked) {
    return <main className="p-8 text-center text-zinc-500">{t("loading")}</main>;
  }

  if (!organizer) {
    const become = async (e: React.FormEvent) => {
      e.preventDefault();
      setSubmitting(true);
      setError(null);
      try {
        const o = await api<Organizer>("/organizers", {
          method: "POST",
          auth: true,
          body: { displayName, bio: bio || undefined },
        });
        setOrganizer(o);
      } catch (err) {
        setError(err instanceof ApiError ? err.message : t("createFailed"));
      } finally {
        setSubmitting(false);
      }
    };

    return (
      <main className="mx-auto max-w-lg px-4 py-10">
        <h1 className="mb-2 text-2xl font-bold">{t("becomeTitle")}</h1>
        <p className="mb-6 text-zinc-500">{t("becomeSubtitle")}</p>
        <form onSubmit={become} className="flex flex-col gap-4">
          <label className="flex flex-col gap-1 text-sm font-medium">
            {t("organizerNameLabel")}
            <input
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              required
              maxLength={100}
              placeholder={t("organizerNamePlaceholder")}
              className="rounded-lg border border-zinc-300 px-3 py-2 dark:border-zinc-700 dark:bg-zinc-900"
            />
          </label>
          <label className="flex flex-col gap-1 text-sm font-medium">
            {t("aboutLabel")}
            <textarea
              value={bio}
              onChange={(e) => setBio(e.target.value)}
              rows={3}
              maxLength={1000}
              className="rounded-lg border border-zinc-300 px-3 py-2 dark:border-zinc-700 dark:bg-zinc-900"
            />
          </label>
          <button
            type="submit"
            disabled={submitting}
            className="rounded-lg bg-sky-500 px-4 py-2 font-medium text-white hover:bg-sky-600 disabled:opacity-50"
          >
            {submitting ? t("creating") : t("create")}
          </button>
          {error ? <p className="text-sm text-red-600">{error}</p> : null}
        </form>
      </main>
    );
  }

  return (
    <main className="mx-auto max-w-3xl px-4 py-10">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">{organizer.displayName}</h1>
          <p className="text-sm text-zinc-500">{t("yourEvents")}</p>
        </div>
        <Link
          href="/organizer/events/new"
          className="rounded-lg bg-sky-500 px-4 py-2 text-sm font-medium text-white hover:bg-sky-600"
        >
          {t("newEvent")}
        </Link>
      </div>

      {stats ? (
        <div className="mb-6 grid grid-cols-2 gap-3 sm:grid-cols-4">
          {[
            { label: t("statsEvents"), value: stats.totalEvents },
            { label: t("statsUpcoming"), value: stats.upcomingPublished },
            { label: t("statsTotalRsvps"), value: stats.totalRsvps },
            {
              label: t("statsCheckinRate"),
              value:
                stats.totalRsvps > 0
                  ? `${Math.round((stats.totalCheckins / stats.totalRsvps) * 100)}%`
                  : "—",
            },
          ].map((s) => (
            <div
              key={s.label}
              className="rounded-xl border border-zinc-200 p-4 text-center dark:border-zinc-800"
            >
              <p className="text-2xl font-bold">{s.value}</p>
              <p className="text-xs text-zinc-500">{s.label}</p>
            </div>
          ))}
        </div>
      ) : null}

      {events.length === 0 ? (
        <p className="rounded-lg border border-dashed border-zinc-300 p-10 text-center text-zinc-500 dark:border-zinc-700">
          {t("noEventsYet")}
        </p>
      ) : (
        <ul className="flex flex-col gap-3">
          {events.map((e) => (
            <li
              key={e.id}
              className="flex items-center justify-between rounded-lg border border-zinc-200 p-4 dark:border-zinc-800"
            >
              <div>
                <Link
                  href={`/organizer/events/${e.id}/edit`}
                  className="font-medium hover:text-sky-500"
                >
                  {e.title}
                </Link>
                <p className="text-sm text-zinc-500">
                  {new Date(e.startsAt).toLocaleString()} ·{" "}
                  {tEventCard("going", { count: e.goingCount })}
                  {e.capacity ? ` / ${e.capacity}` : ""}
                </p>
              </div>
              <span
                className={`rounded-full px-3 py-1 text-xs font-medium ${statusStyle[e.status]}`}
              >
                {statusLabel[e.status]}
              </span>
            </li>
          ))}
        </ul>
      )}
    </main>
  );
}
