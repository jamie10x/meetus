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
    return <main className="p-8 text-center text-zinc-500">{t("loadFailed")}</main>;
  }
  if (attendees === null) {
    return <main className="p-8 text-center text-zinc-500">{t("loading")}</main>;
  }

  const checkedIn = attendees.filter((a) => a.checkedInAt).length;

  return (
    <main className="mx-auto max-w-2xl px-4 py-8">
      <div className="mb-4 flex items-center justify-between">
        <h1 className="text-xl font-bold">{t("title")}</h1>
        <Link
          href={`/organizer/events/${id}/edit`}
          className="text-sm text-zinc-500 hover:text-sky-500"
        >
          {t("back")}
        </Link>
      </div>

      <div className="mb-4 flex items-center justify-between">
        <p className="text-sm text-zinc-500">
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
            className="rounded-lg border border-zinc-300 px-3 py-1 text-xs font-medium hover:border-sky-500 hover:text-sky-500 dark:border-zinc-700"
          >
            {t("exportCsv")}
          </button>
        ) : null}
      </div>

      {attendees.length === 0 ? (
        <p className="rounded-lg border border-dashed border-zinc-300 p-10 text-center text-zinc-500 dark:border-zinc-700">
          {t("noRsvps")}
        </p>
      ) : (
        <ul className="divide-y divide-zinc-200 rounded-xl border border-zinc-200 dark:divide-zinc-800 dark:border-zinc-800">
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
                <div className="flex h-9 w-9 items-center justify-center rounded-full bg-zinc-200 text-sm font-medium dark:bg-zinc-700">
                  {a.name[0]}
                </div>
              )}
              <div className="min-w-0 flex-1">
                <p className="truncate font-medium">{a.name}</p>
                {a.username ? (
                  <p className="truncate text-xs text-zinc-500">@{a.username}</p>
                ) : null}
              </div>
              {a.checkedInAt ? (
                <span className="rounded-full bg-green-100 px-2.5 py-0.5 text-xs font-medium text-green-700 dark:bg-green-900 dark:text-green-300">
                  {t("checkedIn")}
                </span>
              ) : (
                <span className="rounded-full bg-zinc-100 px-2.5 py-0.5 text-xs text-zinc-500 dark:bg-zinc-800">
                  {t("going")}
                </span>
              )}
            </li>
          ))}
        </ul>
      )}

      {comments.length > 0 ? (
        <section className="mt-8">
          <h2 className="mb-3 text-lg font-semibold">
            {t("commentsHeading", { count: comments.length })}
          </h2>
          <ul className="flex flex-col gap-3">
            {comments.map((c, i) => (
              <li
                key={i}
                className="rounded-lg border border-zinc-200 p-3 dark:border-zinc-800"
              >
                <div className="mb-1 flex items-center justify-between">
                  <span className="text-sm font-medium">{c.userName}</span>
                  <span className="text-xs text-amber-500">
                    {"★".repeat(c.rating)}
                    <span className="text-zinc-300 dark:text-zinc-700">
                      {"★".repeat(5 - c.rating)}
                    </span>
                  </span>
                </div>
                <p className="text-sm text-zinc-600 dark:text-zinc-300">{c.comment}</p>
              </li>
            ))}
          </ul>
        </section>
      ) : null}
    </main>
  );
}
