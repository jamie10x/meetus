"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { api, ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
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

const card =
  "rounded-xl border border-zinc-200 p-4 text-center dark:border-zinc-800";
const btn =
  "rounded-lg border px-2.5 py-1 text-xs font-medium transition-colors";

function StatCard({ label, value }: { label: string; value: number | string }) {
  return (
    <div className={card}>
      <p className="text-2xl font-bold">{value}</p>
      <p className="text-xs text-zinc-500">{label}</p>
    </div>
  );
}

export default function AdminPage() {
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
    const t = setTimeout(() => {
      api<AdminUser[]>(
        `/admin/users?q=${encodeURIComponent(userQuery)}`,
        { auth: true },
      )
        .then(setUsers)
        .catch(() => setUsers([]));
    }, 300);
    return () => clearTimeout(t);
  }, [user, userQuery]);

  if (loading || !user?.isAdmin) {
    return <main className="p-8 text-center text-zinc-500">Loading…</main>;
  }

  const act = async (path: string, refresh: () => void) => {
    setError(null);
    try {
      await api(path, { method: "POST", auth: true });
      refresh();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Action failed.");
    }
  };

  return (
    <main className="mx-auto max-w-4xl px-4 py-8">
      <h1 className="mb-6 text-2xl font-bold">Admin</h1>
      {error ? <p className="mb-4 text-sm text-red-600">{error}</p> : null}

      {stats ? (
        <div className="mb-8 grid grid-cols-2 gap-3 sm:grid-cols-4">
          <StatCard label="Users" value={stats.users} />
          <StatCard label="Organizers" value={stats.organizers} />
          <StatCard label="Upcoming events" value={stats.upcomingEvents} />
          <StatCard label="RSVPs (7d)" value={stats.rsvps7d} />
          <StatCard label="RSVPs (30d)" value={stats.rsvps30d} />
          <StatCard label="Check-ins (30d)" value={stats.checkins30d} />
          <StatCard
            label="Published events"
            value={stats.eventsByStatus["published"] ?? 0}
          />
          <StatCard
            label="Draft events"
            value={stats.eventsByStatus["draft"] ?? 0}
          />
        </div>
      ) : null}

      <section className="mb-10">
        <div className="mb-3 flex items-center justify-between">
          <h2 className="text-lg font-semibold">Events</h2>
          <select
            value={statusFilter}
            onChange={(e) => {
              setStatusFilter(e.target.value);
              loadEvents(e.target.value);
            }}
            className="rounded-lg border border-zinc-300 px-2 py-1 text-sm dark:border-zinc-700 dark:bg-zinc-900"
          >
            <option value="">All statuses</option>
            <option value="published">Published</option>
            <option value="draft">Draft</option>
            <option value="canceled">Canceled</option>
            <option value="finished">Finished</option>
          </select>
        </div>
        <ul className="divide-y divide-zinc-200 rounded-xl border border-zinc-200 dark:divide-zinc-800 dark:border-zinc-800">
          {events.map((e) => (
            <li key={e.id} className="flex items-center gap-3 p-3">
              <div className="min-w-0 flex-1">
                <Link
                  href={`/events/${e.id}`}
                  className="truncate font-medium hover:text-sky-500"
                >
                  {e.title}
                </Link>
                <p className="text-xs text-zinc-500">
                  {e.organizerName} · {new Date(e.startsAt).toLocaleString()} ·{" "}
                  {e.goingCount} going
                </p>
              </div>
              <span className="rounded-full bg-zinc-100 px-2.5 py-0.5 text-xs dark:bg-zinc-800">
                {e.status}
              </span>
              {e.status === "published" ? (
                <>
                  <button
                    onClick={() =>
                      act(`/admin/events/${e.id}/unpublish`, () =>
                        loadEvents(statusFilter),
                      )
                    }
                    className={`${btn} border-zinc-400 text-zinc-600 hover:bg-zinc-50 dark:hover:bg-zinc-900`}
                  >
                    Unpublish
                  </button>
                  <button
                    onClick={() =>
                      act(`/admin/events/${e.id}/cancel`, () =>
                        loadEvents(statusFilter),
                      )
                    }
                    className={`${btn} border-red-500 text-red-600 hover:bg-red-50 dark:hover:bg-red-950`}
                  >
                    Cancel
                  </button>
                </>
              ) : null}
            </li>
          ))}
          {events.length === 0 ? (
            <li className="p-6 text-center text-sm text-zinc-500">
              No events.
            </li>
          ) : null}
        </ul>
      </section>

      <section>
        <div className="mb-3 flex items-center justify-between">
          <h2 className="text-lg font-semibold">Users</h2>
          <input
            value={userQuery}
            onChange={(e) => setUserQuery(e.target.value)}
            placeholder="Search name or username…"
            className="rounded-lg border border-zinc-300 px-3 py-1 text-sm dark:border-zinc-700 dark:bg-zinc-900"
          />
        </div>
        <ul className="divide-y divide-zinc-200 rounded-xl border border-zinc-200 dark:divide-zinc-800 dark:border-zinc-800">
          {users.map((u) => (
            <li key={u.id} className="flex items-center gap-3 p-3">
              <div className="min-w-0 flex-1">
                <p className="truncate font-medium">
                  {u.name}
                  {u.isAdmin ? (
                    <span className="ml-2 rounded-full bg-sky-100 px-2 py-0.5 text-xs text-sky-700 dark:bg-sky-900 dark:text-sky-300">
                      admin
                    </span>
                  ) : null}
                </p>
                <p className="text-xs text-zinc-500">
                  {u.username ? `@${u.username} · ` : ""}
                  joined {new Date(u.createdAt).toLocaleDateString()}
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
                  className={`${btn} border-green-500 text-green-600 hover:bg-green-50 dark:hover:bg-green-950`}
                >
                  Unban
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
                  className={`${btn} border-red-500 text-red-600 hover:bg-red-50 dark:hover:bg-red-950`}
                >
                  Ban
                </button>
              ) : null}
            </li>
          ))}
          {users.length === 0 ? (
            <li className="p-6 text-center text-sm text-zinc-500">
              No users found.
            </li>
          ) : null}
        </ul>
      </section>
    </main>
  );
}
