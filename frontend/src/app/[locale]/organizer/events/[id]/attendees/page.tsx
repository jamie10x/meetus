"use client";

import { use, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
import { api, API_URL, getAccessToken } from "@/lib/api";

type Attendee = {
  userId: number;
  name: string;
  username: string | null;
  avatarUrl: string | null;
  rsvpAt: string;
  checkedInAt: string | null;
};

type FeedbackSummary = {
  count: number;
  average: number;
};

type FeedbackComment = {
  userName: string;
  rating: number;
  comment: string;
  createdAt: string;
};

export default function AttendeesPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const t = useTranslations("attendees");
  const { id } = use(params);
  const [attendees, setAttendees] = useState<Attendee[] | null>(null);
  const [feedback, setFeedback] = useState<FeedbackSummary | null>(null);
  const [comments, setComments] = useState<FeedbackComment[]>([]);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    api<Attendee[]>(`/events/${id}/attendees`, { auth: true })
      .then(setAttendees)
      .catch(() => setFailed(true));
    api<FeedbackSummary>(`/events/${id}/feedback`, { auth: true })
      .then(setFeedback)
      .catch(() => setFeedback(null));
    api<FeedbackComment[]>(`/events/${id}/feedback/comments`, { auth: true })
      .then(setComments)
      .catch(() => setComments([]));
  }, [id]);

  if (failed) {
    return <main className="p-8 text-center text-dust">{t("loadFailed")}</main>;
  }
  if (attendees === null) {
    return <main className="p-8 text-center text-dust">{t("loading")}</main>;
  }

  const checkedIn = attendees.filter((a) => a.checkedInAt).length;

  return (
    <main className="mx-auto max-w-2xl px-4 py-8">
      <div className="mb-4 flex items-center justify-between">
        <h1 className="text-xl font-bold text-bone">{t("title")}</h1>
        <Link
          href={`/organizer/events/${id}/edit`}
          className="text-sm text-dust hover:text-registan-strong"
        >
          {t("back")}
        </Link>
      </div>

      <div className="mb-4 flex items-center justify-between">
        <p className="text-sm text-dust">
          {t("summary", { going: attendees.length, checkedIn })}
          {feedback && feedback.count > 0 ? (
            <>
              {" · "}
              {t("ratings", {
                average: feedback.average.toFixed(1),
                count: feedback.count,
              })}
            </>
          ) : null}
        </p>
        {attendees.length > 0 ? (
          <button
            onClick={async () => {
              const res = await fetch(
                `${API_URL}/api/events/${id}/attendees.csv`,
                { headers: { Authorization: `Bearer ${getAccessToken()}` } },
              );
              if (!res.ok) return;
              const blob = await res.blob();
              const url = URL.createObjectURL(blob);
              const a = document.createElement("a");
              a.href = url;
              a.download = `attendees-event-${id}.csv`;
              a.click();
              URL.revokeObjectURL(url);
            }}
            className="rounded-lg border border-line px-3 py-1 text-xs font-medium text-dust transition-colors hover:border-registan-strong hover:text-registan-strong"
          >
            {t("exportCsv")}
          </button>
        ) : null}
      </div>

      {attendees.length === 0 ? (
        <p className="rounded-card border border-dashed border-line p-10 text-center text-dust">
          {t("noRsvps")}
        </p>
      ) : (
        <ul className="divide-y divide-line rounded-card border border-line bg-ink-raised">
          {attendees.map((a) => (
            <li key={a.userId} className="flex items-center gap-3 p-3">
              {a.avatarUrl ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img
                  src={a.avatarUrl}
                  alt=""
                  className="h-9 w-9 rounded-full"
                />
              ) : (
                <div className="flex h-9 w-9 items-center justify-center rounded-full bg-ink-overlay text-sm font-medium text-bone">
                  {a.name[0]}
                </div>
              )}
              <div className="min-w-0 flex-1">
                <p className="truncate font-medium text-bone">{a.name}</p>
                {a.username ? (
                  <p className="truncate text-xs text-dust-dim">@{a.username}</p>
                ) : null}
              </div>
              {a.checkedInAt ? (
                <span className="rounded-full border border-registan-dim bg-registan/[0.12] px-2.5 py-0.5 text-xs font-medium text-registan-strong">
                  {t("checkedIn")}
                </span>
              ) : (
                <span className="rounded-full border border-line bg-ink-raised px-2.5 py-0.5 text-xs text-dust">
                  {t("going")}
                </span>
              )}
            </li>
          ))}
        </ul>
      )}

      {comments.length > 0 ? (
        <section className="mt-8">
          <h2 className="mb-3 text-lg font-semibold text-bone">
            {t("commentsHeading", { count: comments.length })}
          </h2>
          <ul className="flex flex-col gap-3">
            {comments.map((c, i) => (
              <li
                key={i}
                className="rounded-card border border-line bg-ink-raised p-3"
              >
                <div className="mb-1 flex items-center justify-between">
                  <span className="text-sm font-medium text-bone">{c.userName}</span>
                  <span className="text-xs text-atlas">
                    {"★".repeat(c.rating)}
                    <span className="text-dust-dim">
                      {"★".repeat(5 - c.rating)}
                    </span>
                  </span>
                </div>
                <p className="text-sm text-dust">{c.comment}</p>
              </li>
            ))}
          </ul>
        </section>
      ) : null}
    </main>
  );
}
