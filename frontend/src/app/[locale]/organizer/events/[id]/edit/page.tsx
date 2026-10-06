"use client";

import { errorMessage } from "@/lib/errorMessage";

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
  const tErrors = useTranslations("errors");
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
    const controller = new AbortController();
    setNotFound(false);
    api<EventItem>(`/events/${id}`, { auth: true, signal: controller.signal })
      .then(setEvent)
      .catch(error => {
        if (controller.signal.aborted) return;
        if (error instanceof ApiError && error.status === 404) setNotFound(true);
        else setActionError(errorMessage(error, tErrors, t("actionFailed")));
      });
    api<Channel[]>("/organizers/me/channels", { auth: true, signal: controller.signal })
      .then(setChannels)
      .catch(() => undefined);
    return () => controller.abort();
  }, [id, t, tErrors]);

  if (notFound) {
    return <main className="p-8 text-center text-dust">{t("notFound")}</main>;
  }
  if (!event && actionError) return <main role="alert" className="p-8 text-center text-pomegranate">{actionError}</main>;
  if (!event) {
    return <main className="p-8 text-center text-dust">{t("loading")}</main>;
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
      setActionError(errorMessage(e, tErrors, t("actionFailed")));
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
      setActionError(errorMessage(e, tErrors, t("deleteFailed")));
    }
  };

  const btn = "btn btn-outline";
  const statusStyle: Record<string, string> = {
    draft: "border border-atlas/35 bg-atlas/[0.12] text-atlas",
    published: "border border-registan-dim bg-registan/[0.12] text-registan-strong",
    canceled: "border border-pomegranate/35 bg-pomegranate/[0.12] text-pomegranate",
    finished: "border border-line bg-ink-raised text-dust",
  };
  const statusLabel: Record<string, string> = {
    draft: tStatus("statusDraft"),
    published: tStatus("statusPublished"),
    canceled: tStatus("statusCanceled"),
    finished: tStatus("statusFinished"),
  };

  return (
    <main className="mx-auto max-w-2xl px-4 py-10">
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-bold text-bone">{t("title")}</h1>
        <span className={`rounded-full px-3 py-1 text-xs font-medium ${statusStyle[event.status]}`}>
          {statusLabel[event.status]}
        </span>
      </div>

      <div className="mb-6 flex flex-wrap gap-2">
        {event.status === "draft" ? (
          <>
            <button
              onClick={() => doAction("publish")}
              className={`${btn} btn-outline-accent`}
            >
              {t("publish")}
            </button>
            <button
              onClick={remove}
              className={`${btn} btn-outline-danger`}
            >
              {t("deleteDraft")}
            </button>
          </>
        ) : null}
        {event.status === "published" ? (
          <>
            <Link
              href={`/organizer/events/${event.id}/scan`}
              className={`${btn} btn-outline-accent`}
            >
              {t("scanTickets")}
            </Link>
            <Link
              href={`/organizer/events/${event.id}/attendees`}
              className={`${btn} btn-outline-neutral`}
            >
              {t("attendees", { count: event.goingCount })}
            </Link>
            <button
              onClick={() => doAction("unpublish")}
              className={`${btn} btn-outline-neutral`}
            >
              {t("unpublish")}
            </button>
            <button
              onClick={() => doAction("cancel")}
              className={`${btn} btn-outline-danger`}
            >
              {t("cancelEvent")}
            </button>
          </>
        ) : null}
      </div>

      {actionError ? (
        <p className="mb-4 text-sm text-pomegranate">{actionError}</p>
      ) : null}

      {event.status === "draft" && channels.length > 0 ? (
        <p className="mb-6 text-sm text-dust">
          {t("autoAnnounceHint", { count: channels.length })}
        </p>
      ) : null}

      {event.status === "published" && channels.length > 0 ? (
        <div className="mb-6 rounded-card border border-line bg-ink-raised p-4">
          <h2 className="mb-3 text-sm font-semibold text-bone">{t("announceHeading")}</h2>
          <ul className="flex flex-col gap-2">
            {channels.map((ch) => {
              const state = announceState[ch.id];
              return (
                <li key={ch.id} className="flex items-center justify-between gap-3">
                  <span className="text-sm text-bone">{ch.chatTitle}</span>
                  <div className="flex items-center gap-2">
                    {state === "sent" ? (
                      <span className="text-xs text-registan-strong">
                        {t("announceSent", { channel: ch.chatTitle })}
                      </span>
                    ) : state === "failed" ? (
                      <span className="text-xs text-pomegranate">{t("announceFailed")}</span>
                    ) : null}
                    <button
                      onClick={() => announce(ch.id)}
                      disabled={state === "sending"}
                      className={`${btn} btn-outline-accent`}
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
        <p className="text-dust">
          {t("cannotEdit", { status: statusLabel[event.status] })}
        </p>
      )}
    </main>
  );
}
