"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { Link, useRouter } from "@/i18n/navigation";
import { api, ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import MetaManager from "@/components/MetaManager";
import type { EventItem } from "@/lib/types";

type AdminStats = {
  users: number;
  organizers: number;
  eventsByStatus: Record<string, number>;
  upcomingEvents: number;
  rsvps7d: number;
  rsvps30d: number;
  checkins30d: number;
};

type AdminUser = {
  id: number;
  name: string;
  username: string | null;
  isBanned: boolean;
  isAdmin: boolean;
  createdAt: string;
};

const card = "rounded-card border border-line bg-ink-raised p-4 text-center";
const btn =
  "rounded-lg border px-2.5 py-1 text-xs font-medium transition-colors";

const eventStatusKey = {
  draft: "statusDraft",
  published: "statusPublished",
  canceled: "statusCanceled",
  finished: "statusFinished",
} as const;

const eventStatusStyle: Record<string, string> = {
  draft: "border border-atlas/35 bg-atlas/[0.12] text-atlas",
  published: "border border-registan-dim bg-registan/[0.12] text-registan-strong",
  canceled: "border border-pomegranate/35 bg-pomegranate/[0.12] text-pomegranate",
  finished: "border border-line bg-ink-raised text-dust",
};

function StatCard({ label, value }: { label: string; value: number | string }) {
  return (
    <div className={card}>
      <p className="text-2xl font-bold text-bone">{value}</p>
      <p className="text-xs text-dust">{label}</p>
    </div>
  );
}

export default function AdminPage() {
  const t = useTranslations("admin");
  const { user, loading } = useAuth();
  const router = useRouter();

  const [stats, setStats] = useState<AdminStats | null>(null);
  const [events, setEvents] = useState<EventItem[]>([]);
  const [statusFilter, setStatusFilter] = useState("");
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [userQuery, setUserQuery] = useState("");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!loading && (!user || !user.isAdmin)) router.replace("/");
  }, [loading, user, router]);

  const loadEvents = useCallback((status: string) => {
    const qs = status ? `?status=${status}` : "";
    api<EventItem[]>(`/admin/events${qs}`, { auth: true })
      .then(setEvents)
      .catch(() => setEvents([]));
  }, []);

  useEffect(() => {
    if (!user?.isAdmin) return;
    api<AdminStats>("/admin/stats", { auth: true })
      .then(setStats)
      .catch(() => setStats(null));
    loadEvents("");
  }, [user, loadEvents]);

  // Debounced user search.
  useEffect(() => {
    if (!user?.isAdmin) return;
    const timer = setTimeout(() => {
      api<AdminUser[]>(
        `/admin/users?q=${encodeURIComponent(userQuery)}`,
        { auth: true },
      )
        .then(setUsers)
        .catch(() => setUsers([]));
    }, 300);
    return () => clearTimeout(timer);
  }, [user, userQuery]);

  if (loading || !user?.isAdmin) {
    return <main className="p-8 text-center text-dust">{t("loading")}</main>;
  }

  const act = async (path: string, refresh: () => void) => {
    setError(null);
    try {
      await api(path, { method: "POST", auth: true });
      refresh();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("actionFailed"));
    }
  };

  return (
    <main className="mx-auto max-w-4xl px-4 py-8">
      <h1 className="mb-6 text-2xl font-bold text-bone">{t("title")}</h1>
      {error ? <p className="mb-4 text-sm text-pomegranate">{error}</p> : null}

      {stats ? (
        <div className="mb-8 grid grid-cols-2 gap-3 sm:grid-cols-4">
          <StatCard label={t("statUsers")} value={stats.users} />
          <StatCard label={t("statOrganizers")} value={stats.organizers} />
          <StatCard label={t("statUpcomingEvents")} value={stats.upcomingEvents} />
          <StatCard label={t("statRsvps7d")} value={stats.rsvps7d} />
          <StatCard label={t("statRsvps30d")} value={stats.rsvps30d} />
          <StatCard label={t("statCheckins30d")} value={stats.checkins30d} />
          <StatCard
            label={t("statPublishedEvents")}
            value={stats.eventsByStatus["published"] ?? 0}
          />
          <StatCard
            label={t("statDraftEvents")}
            value={stats.eventsByStatus["draft"] ?? 0}
          />
        </div>
      ) : null}

      <section className="mb-10">
        <div className="mb-3 flex items-center justify-between">
          <h2 className="text-lg font-semibold text-bone">{t("eventsHeading")}</h2>
          <select
            value={statusFilter}
            onChange={(e) => {
              setStatusFilter(e.target.value);
              loadEvents(e.target.value);
            }}
            className="rounded-lg border border-line bg-ink-raised px-2 py-1 text-sm text-bone transition-colors focus:border-registan-dim"
          >
            <option value="">{t("allStatuses")}</option>
            <option value="published">{t("statusPublished")}</option>
            <option value="draft">{t("statusDraft")}</option>
            <option value="canceled">{t("statusCanceled")}</option>
            <option value="finished">{t("statusFinished")}</option>
          </select>
        </div>
        <ul className="divide-y divide-line rounded-card border border-line bg-ink-raised">
          {events.map((e) => (
            <li key={e.id} className="flex items-center gap-3 p-3">
              <div className="min-w-0 flex-1">
                <Link
                  href={`/events/${e.id}`}
                  className="truncate font-medium text-bone hover:text-registan-strong"
                >
                  {e.title}
                </Link>
                <p className="text-xs text-dust">
                  {e.organizerName} · {new Date(e.startsAt).toLocaleString()} ·{" "}
                  {e.goingCount}
                </p>
              </div>
              <span
                className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${eventStatusStyle[e.status] ?? eventStatusStyle.finished}`}
              >
                {t(eventStatusKey[e.status as keyof typeof eventStatusKey])}
              </span>
              {e.status === "published" ? (
                <>
                  <button
                    onClick={() =>
                      act(`/admin/events/${e.id}/unpublish`, () =>
                        loadEvents(statusFilter),
                      )
                    }
                    className={`${btn} border-line text-dust hover:border-registan-strong hover:text-registan-strong`}
                  >
                    {t("unpublish")}
                  </button>
                  <button
                    onClick={() =>
                      act(`/admin/events/${e.id}/cancel`, () =>
                        loadEvents(statusFilter),
                      )
                    }
                    className={`${btn} border-pomegranate/35 text-pomegranate hover:bg-pomegranate/[0.12]`}
                  >
                    {t("cancel")}
                  </button>
                </>
              ) : null}
            </li>
          ))}
          {events.length === 0 ? (
            <li className="p-6 text-center text-sm text-dust-dim">
              {t("noEvents")}
            </li>
          ) : null}
        </ul>
      </section>

      <MetaManager resource="cities" heading={t("citiesHeading")} />
      <MetaManager resource="categories" heading={t("categoriesHeading")} />

      <section>
        <div className="mb-3 flex items-center justify-between">
          <h2 className="text-lg font-semibold text-bone">{t("usersHeading")}</h2>
          <input
            value={userQuery}
            onChange={(e) => setUserQuery(e.target.value)}
            placeholder={t("searchUsersPlaceholder")}
            className="rounded-lg border border-line bg-ink-raised px-3 py-1 text-sm text-bone placeholder:text-dust-dim transition-colors focus:border-registan-dim"
          />
        </div>
        <ul className="divide-y divide-line rounded-card border border-line bg-ink-raised">
          {users.map((u) => (
            <li key={u.id} className="flex items-center gap-3 p-3">
              <div className="min-w-0 flex-1">
                <p className="truncate font-medium text-bone">
                  {u.name}
                  {u.isAdmin ? (
                    <span className="ml-2 rounded-full border border-registan-dim bg-registan/[0.12] px-2 py-0.5 text-xs text-registan-strong">
                      {t("adminBadge")}
                    </span>
                  ) : null}
                </p>
                <p className="text-xs text-dust">
                  {u.username ? `@${u.username} · ` : ""}
                  {t("joined", {
                    date: new Date(u.createdAt).toLocaleDateString(),
                  })}
                </p>
              </div>
              {u.isBanned ? (
                <button
                  onClick={() =>
                    act(`/admin/users/${u.id}/unban`, () =>
                      setUsers((prev) =>
                        prev.map((x) =>
                          x.id === u.id ? { ...x, isBanned: false } : x,
                        ),
                      ),
                    )
                  }
                  className={`${btn} border-registan-dim text-registan-strong hover:bg-registan/[0.12]`}
                >
                  {t("unban")}
                </button>
              ) : !u.isAdmin ? (
                <button
                  onClick={() =>
                    act(`/admin/users/${u.id}/ban`, () =>
                      setUsers((prev) =>
                        prev.map((x) =>
                          x.id === u.id ? { ...x, isBanned: true } : x,
                        ),
                      ),
                    )
                  }
                  className={`${btn} border-pomegranate/35 text-pomegranate hover:bg-pomegranate/[0.12]`}
                >
                  {t("ban")}
                </button>
              ) : null}
            </li>
          ))}
          {users.length === 0 ? (
            <li className="p-6 text-center text-sm text-dust-dim">
              {t("noUsers")}
            </li>
          ) : null}
        </ul>
      </section>
    </main>
  );
}
