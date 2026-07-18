"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { api, ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import type { EventItem, Organizer } from "@/lib/types";

type OrganizerStats = {
  totalEvents: number;
  upcomingPublished: number;
  totalRsvps: number;
  totalCheckins: number;
};

const statusStyle: Record<string, string> = {
  draft: "bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300",
  published: "bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300",
  canceled: "bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300",
  finished: "bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300",
};

export default function OrganizerPage() {
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
    return <main className="p-8 text-center text-zinc-500">Loading…</main>;
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
        setError(err instanceof ApiError ? err.message : "Request failed.");
      } finally {
        setSubmitting(false);
      }
    };

    return (
      <main className="mx-auto max-w-lg px-4 py-10">
        <h1 className="mb-2 text-2xl font-bold">Become an organizer</h1>
        <p className="mb-6 text-zinc-500">
          Create an organizer profile to start hosting events.
        </p>
        <form onSubmit={become} className="flex flex-col gap-4">
          <label className="flex flex-col gap-1 text-sm font-medium">
            Organizer name
            <input
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              required
              maxLength={100}
              placeholder="e.g. Tashkent JS Community"
              className="rounded-lg border border-zinc-300 px-3 py-2 dark:border-zinc-700 dark:bg-zinc-900"
            />
          </label>
          <label className="flex flex-col gap-1 text-sm font-medium">
            About (optional)
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
            {submitting ? "Creating…" : "Create organizer profile"}
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
          <p className="text-sm text-zinc-500">Your events</p>
        </div>
        <Link
          href="/organizer/events/new"
          className="rounded-lg bg-sky-500 px-4 py-2 text-sm font-medium text-white hover:bg-sky-600"
        >
          + New event
        </Link>
      </div>

      {stats ? (
        <div className="mb-6 grid grid-cols-2 gap-3 sm:grid-cols-4">
          {[
            { label: "Events", value: stats.totalEvents },
            { label: "Upcoming", value: stats.upcomingPublished },
            { label: "Total RSVPs", value: stats.totalRsvps },
            {
              label: "Check-in rate",
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
          No events yet. Create your first one!
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
                  {e.goingCount} going
                  {e.capacity ? ` / ${e.capacity}` : ""}
                </p>
              </div>
              <span
                className={`rounded-full px-3 py-1 text-xs font-medium ${statusStyle[e.status]}`}
              >
                {e.status}
              </span>
            </li>
          ))}
        </ul>
      )}
    </main>
  );
}
