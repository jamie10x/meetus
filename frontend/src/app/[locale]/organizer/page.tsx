"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { Link, useRouter } from "@/i18n/navigation";
import { api, ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import type { Channel, EventItem, Organizer } from "@/lib/types";
import VerifiedBadge from "@/components/VerifiedBadge";

type OrganizerStats = {
  totalEvents: number;
  upcomingPublished: number;
  totalRsvps: number;
  totalCheckins: number;
};

const BOT_USERNAME = process.env.NEXT_PUBLIC_TELEGRAM_BOT_USERNAME ?? "";

export default function OrganizerPage() {
  const t = useTranslations("organizer");
  const tEventCard = useTranslations("eventCard");
  const tCommon = useTranslations("common");
  const { user, loading } = useAuth();
  const router = useRouter();

  const [organizer, setOrganizer] = useState<Organizer | null>(null);
  const [events, setEvents] = useState<EventItem[]>([]);
  const [stats, setStats] = useState<OrganizerStats | null>(null);
  const [channels, setChannels] = useState<Channel[]>([]);
  const [checked, setChecked] = useState(false);

  const [displayName, setDisplayName] = useState("");
  const [bio, setBio] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const statusStyle: Record<string, string> = {
    draft: "border border-atlas/35 bg-atlas/[0.12] text-atlas",
    published: "border border-registan-dim bg-registan/[0.12] text-registan-strong",
    canceled: "border border-pomegranate/35 bg-pomegranate/[0.12] text-pomegranate",
    finished: "border border-line bg-ink-raised text-dust",
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
        api<Channel[]>("/organizers/me/channels", { auth: true })
          .then(setChannels)
          .catch(() => setChannels([]));
      })
      .catch(() => setOrganizer(null))
      .finally(() => setChecked(true));
  }, [user]);

  if (loading || !user || !checked) {
    return <main className="p-8 text-center text-dust">{t("loading")}</main>;
  }

  const disconnectChannel = async (id: number) => {
    try {
      await api(`/organizers/me/channels/${id}`, { method: "DELETE", auth: true });
      setChannels((prev) => prev.filter((c) => c.id !== id));
    } catch {
      // Non-critical; the list will just show the stale entry until refresh.
    }
  };

  const setChannelLanguage = async (id: number, language: string) => {
    const value = language === "" ? null : language;
    const prev = channels;
    setChannels((cs) => cs.map((c) => (c.id === id ? { ...c, language: value } : c)));
    try {
      await api(`/organizers/me/channels/${id}`, {
        method: "PATCH",
        auth: true,
        body: { language: value },
      });
    } catch {
      setChannels(prev);
    }
  };

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
        <h1 className="mb-2 text-2xl font-bold text-bone">{t("becomeTitle")}</h1>
        <p className="mb-6 text-dust">{t("becomeSubtitle")}</p>
        <form onSubmit={become} className="flex flex-col gap-4">
          <label className="flex flex-col gap-1.5 text-sm font-medium text-dust">
            {t("organizerNameLabel")}
            <input
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              required
              maxLength={100}
              placeholder={t("organizerNamePlaceholder")}
              className="rounded-xl border border-line bg-ink-raised px-3.5 py-2.5 text-bone placeholder:text-dust-dim transition-all focus:border-registan-dim focus:outline-none focus:ring-2 focus:ring-registan/20"
            />
          </label>
          <label className="flex flex-col gap-1.5 text-sm font-medium text-dust">
            {t("aboutLabel")}
            <textarea
              value={bio}
              onChange={(e) => setBio(e.target.value)}
              rows={3}
              maxLength={1000}
              className="rounded-xl border border-line bg-ink-raised px-3.5 py-2.5 text-bone placeholder:text-dust-dim transition-all focus:border-registan-dim focus:outline-none focus:ring-2 focus:ring-registan/20"
            />
          </label>
          <button type="submit" disabled={submitting} className="btn btn-primary">
            {submitting ? t("creating") : t("create")}
          </button>
          {error ? <p className="text-sm text-pomegranate">{error}</p> : null}
        </form>
      </main>
    );
  }

  return (
    <main className="mx-auto max-w-3xl px-4 py-10">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-bone">
            {organizer.displayName}
            {organizer.isVerified ? (
              <VerifiedBadge label={tCommon("verifiedOrganizer")} className="ml-2" />
            ) : null}
          </h1>
          <p className="text-sm text-dust">{t("yourEvents")}</p>
        </div>
        <Link href="/organizer/events/new" className="btn btn-primary btn-sm">
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
              className="rounded-card border border-line bg-ink-raised p-4 text-center"
            >
              <p className="text-2xl font-bold text-bone">{s.value}</p>
              <p className="text-xs text-dust">{s.label}</p>
            </div>
          ))}
        </div>
      ) : null}

      {events.length === 0 ? (
        <p className="rounded-card border border-dashed border-line p-10 text-center text-dust">
          {t("noEventsYet")}
        </p>
      ) : (
        <ul className="flex flex-col gap-3">
          {events.map((e) => (
            <li
              key={e.id}
              className="flex items-center justify-between rounded-card border border-line bg-ink-raised p-4"
            >
              <div>
                <Link
                  href={`/organizer/events/${e.id}/edit`}
                  className="font-medium text-bone hover:text-registan-strong"
                >
                  {e.title}
                </Link>
                <p className="text-sm text-dust">
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

      <section className="mt-10">
        <h2 className="mb-2 text-lg font-semibold text-bone">{t("channelsHeading")}</h2>
        {BOT_USERNAME ? (
          <p className="mb-4 text-sm text-dust">
            {t("channelsHint", { botUsername: `@${BOT_USERNAME}` })}
          </p>
        ) : null}
        {channels.length === 0 ? (
          <p className="rounded-card border border-dashed border-line p-6 text-center text-sm text-dust">
            {t("noChannels")}
          </p>
        ) : (
          <ul className="flex flex-col gap-2">
            {channels.map((c) => (
              <li
                key={c.id}
                className="flex items-center justify-between rounded-card border border-line bg-ink-raised px-4 py-3"
              >
                <div>
                  <p className="font-medium text-bone">{c.chatTitle}</p>
                  <p className="text-xs text-dust-dim">
                    {t("connectedOn", {
                      date: new Date(c.connectedAt).toLocaleDateString(),
                    })}
                  </p>
                </div>
                <div className="flex items-center gap-2">
                  <select
                    value={c.language ?? ""}
                    onChange={(e) => setChannelLanguage(c.id, e.target.value)}
                    className="rounded-lg border border-line bg-ink-raised px-2 py-1 text-xs text-bone transition-all focus:border-registan-dim focus:outline-none focus:ring-2 focus:ring-registan/20"
                    aria-label={t("channelLanguageLabel")}
                  >
                    <option value="">{t("channelLanguageDefault")}</option>
                    <option value="uz">O&apos;zbekcha</option>
                    <option value="ru">Русский</option>
                    <option value="en">English</option>
                  </select>
                  <button
                    onClick={() => disconnectChannel(c.id)}
                    className="rounded-lg border border-pomegranate/35 px-3 py-1 text-xs font-medium text-pomegranate transition-colors hover:bg-pomegranate/[0.12]"
                  >
                    {t("disconnect")}
                  </button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>
    </main>
  );
}
