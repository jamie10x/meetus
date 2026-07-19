"use client";

import { use, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { Link, useRouter } from "@/i18n/navigation";
import EventForm from "@/components/EventForm";
import { api, ApiError } from "@/lib/api";
import type { Channel, EventInput, EventItem } from "@/lib/types";

export default function EditEventPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const t = useTranslations("eventEdit");
  const tStatus = useTranslations("organizer");
  const { id } = use(params);
  const router = useRouter();

  const [event, setEvent] = useState<EventItem | null>(null);
  const [notFound, setNotFound] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [channels, setChannels] = useState<Channel[]>([]);
  const [announceState, setAnnounceState] = useState<
    Record<number, "sending" | "sent" | "failed">
  >({});

  useEffect(() => {
    api<EventItem[]>("/events/mine", { auth: true })
      .then((events) => {
        const found = events.find((e) => e.id === Number(id));
        if (found) setEvent(found);
        else setNotFound(true);
      })
      .catch(() => setNotFound(true));
    api<Channel[]>("/organizers/me/channels", { auth: true })
      .then(setChannels)
      .catch(() => setChannels([]));
  }, [id]);

  if (notFound) {
    return <main className="p-8 text-center text-zinc-500">{t("notFound")}</main>;
  }
  if (!event) {
    return <main className="p-8 text-center text-zinc-500">{t("loading")}</main>;
  }

  const save = async (input: EventInput) => {
    await api<EventItem>(`/events/${event.id}`, {
      method: "PATCH",
      auth: true,
      body: input,
    });
    router.push("/organizer");
  };

  const doAction = async (action: "publish" | "unpublish" | "cancel") => {
    setActionError(null);
    try {
      const updated = await api<EventItem>(`/events/${event.id}/${action}`, {
        method: "POST",
        auth: true,
      });
      setEvent(updated);
    } catch (e) {
      setActionError(e instanceof ApiError ? e.message : t("actionFailed"));
    }
  };

  const announce = async (channelId: number) => {
    setAnnounceState((prev) => ({ ...prev, [channelId]: "sending" }));
    try {
      await api(`/events/${event.id}/announce`, {
        method: "POST",
        auth: true,
        body: { channelId },
      });
      setAnnounceState((prev) => ({ ...prev, [channelId]: "sent" }));
    } catch {
      setAnnounceState((prev) => ({ ...prev, [channelId]: "failed" }));
    }
  };

  const remove = async () => {
    if (!window.confirm(t("deleteConfirm"))) return;
    setActionError(null);
    try {
      await api(`/events/${event.id}`, { method: "DELETE", auth: true });
      router.push("/organizer");
    } catch (e) {
      setActionError(e instanceof ApiError ? e.message : t("deleteFailed"));
    }
  };

  const btn =
    "rounded-lg border px-3 py-1.5 text-sm font-medium transition-colors";
  const statusLabel: Record<string, string> = {
    draft: tStatus("statusDraft"),
    published: tStatus("statusPublished"),
    canceled: tStatus("statusCanceled"),
    finished: tStatus("statusFinished"),
  };

  return (
    <main className="mx-auto max-w-2xl px-4 py-10">
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-bold">{t("title")}</h1>
        <span className="rounded-full bg-zinc-100 px-3 py-1 text-xs font-medium text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300">
          {statusLabel[event.status]}
        </span>
      </div>

      <div className="mb-6 flex flex-wrap gap-2">
        {event.status === "draft" ? (
          <>
            <button
              onClick={() => doAction("publish")}
              className={`${btn} border-green-500 text-green-600 hover:bg-green-50 dark:hover:bg-green-950`}
            >
              {t("publish")}
            </button>
            <button
              onClick={remove}
              className={`${btn} border-red-500 text-red-600 hover:bg-red-50 dark:hover:bg-red-950`}
            >
              {t("deleteDraft")}
            </button>
          </>
        ) : null}
        {event.status === "published" ? (
          <>
            <Link
              href={`/organizer/events/${event.id}/scan`}
              className={`${btn} border-sky-500 text-sky-600 hover:bg-sky-50 dark:hover:bg-sky-950`}
            >
              {t("scanTickets")}
            </Link>
            <Link
              href={`/organizer/events/${event.id}/attendees`}
              className={`${btn} border-zinc-400 text-zinc-600 hover:bg-zinc-50 dark:hover:bg-zinc-900`}
            >
              {t("attendees", { count: event.goingCount })}
            </Link>
            <button
              onClick={() => doAction("unpublish")}
              className={`${btn} border-zinc-400 text-zinc-600 hover:bg-zinc-50 dark:hover:bg-zinc-900`}
            >
              {t("unpublish")}
            </button>
            <button
              onClick={() => doAction("cancel")}
              className={`${btn} border-red-500 text-red-600 hover:bg-red-50 dark:hover:bg-red-950`}
            >
              {t("cancelEvent")}
            </button>
          </>
        ) : null}
      </div>

      {actionError ? (
        <p className="mb-4 text-sm text-red-600">{actionError}</p>
      ) : null}

      {event.status === "published" && channels.length > 0 ? (
        <div className="mb-6 rounded-lg border border-zinc-200 p-4 dark:border-zinc-800">
          <h2 className="mb-3 text-sm font-semibold">{t("announceHeading")}</h2>
          <ul className="flex flex-col gap-2">
            {channels.map((ch) => {
              const state = announceState[ch.id];
              return (
                <li key={ch.id} className="flex items-center justify-between gap-3">
                  <span className="text-sm">{ch.chatTitle}</span>
                  <div className="flex items-center gap-2">
                    {state === "sent" ? (
                      <span className="text-xs text-green-600 dark:text-green-400">
                        {t("announceSent", { channel: ch.chatTitle })}
                      </span>
                    ) : state === "failed" ? (
                      <span className="text-xs text-red-600">{t("announceFailed")}</span>
                    ) : null}
                    <button
                      onClick={() => announce(ch.id)}
                      disabled={state === "sending"}
                      className={`${btn} border-sky-500 text-sky-600 hover:bg-sky-50 disabled:opacity-50 dark:hover:bg-sky-950`}
                    >
                      {state === "sending" ? t("announcing") : t("announce")}
                    </button>
                  </div>
                </li>
              );
            })}
          </ul>
        </div>
      ) : null}

      {event.status === "draft" || event.status === "published" ? (
        <EventForm initial={event} submitLabel={t("saveChanges")} onSubmit={save} />
      ) : (
        <p className="text-zinc-500">
          {t("cannotEdit", { status: statusLabel[event.status] })}
        </p>
      )}
    </main>
  );
}
