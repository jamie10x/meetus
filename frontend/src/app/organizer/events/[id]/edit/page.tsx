"use client";

import { use, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import EventForm from "@/components/EventForm";
import { api, ApiError } from "@/lib/api";
import type { EventInput, EventItem } from "@/lib/types";

export default function EditEventPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const router = useRouter();

  const [event, setEvent] = useState<EventItem | null>(null);
  const [notFound, setNotFound] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  useEffect(() => {
    api<EventItem[]>("/events/mine", { auth: true })
      .then((events) => {
        const found = events.find((e) => e.id === Number(id));
        if (found) setEvent(found);
        else setNotFound(true);
      })
      .catch(() => setNotFound(true));
  }, [id]);

  if (notFound) {
    return (
      <main className="p-8 text-center text-zinc-500">Event not found.</main>
    );
  }
  if (!event) {
    return <main className="p-8 text-center text-zinc-500">Loading…</main>;
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
      setActionError(e instanceof ApiError ? e.message : "Action failed.");
    }
  };

  const remove = async () => {
    if (!window.confirm("Delete this draft? This cannot be undone.")) return;
    setActionError(null);
    try {
      await api(`/events/${event.id}`, { method: "DELETE", auth: true });
      router.push("/organizer");
    } catch (e) {
      setActionError(e instanceof ApiError ? e.message : "Delete failed.");
    }
  };

  const btn =
    "rounded-lg border px-3 py-1.5 text-sm font-medium transition-colors";

  return (
    <main className="mx-auto max-w-2xl px-4 py-10">
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-bold">Edit event</h1>
        <span className="rounded-full bg-zinc-100 px-3 py-1 text-xs font-medium text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300">
          {event.status}
        </span>
      </div>

      <div className="mb-6 flex flex-wrap gap-2">
        {event.status === "draft" ? (
          <>
            <button
              onClick={() => doAction("publish")}
              className={`${btn} border-green-500 text-green-600 hover:bg-green-50 dark:hover:bg-green-950`}
            >
              Publish
            </button>
            <button
              onClick={remove}
              className={`${btn} border-red-500 text-red-600 hover:bg-red-50 dark:hover:bg-red-950`}
            >
              Delete draft
            </button>
          </>
        ) : null}
        {event.status === "published" ? (
          <>
            <button
              onClick={() => doAction("unpublish")}
              className={`${btn} border-zinc-400 text-zinc-600 hover:bg-zinc-50 dark:hover:bg-zinc-900`}
            >
              Unpublish
            </button>
            <button
              onClick={() => doAction("cancel")}
              className={`${btn} border-red-500 text-red-600 hover:bg-red-50 dark:hover:bg-red-950`}
            >
              Cancel event
            </button>
          </>
        ) : null}
      </div>

      {actionError ? (
        <p className="mb-4 text-sm text-red-600">{actionError}</p>
      ) : null}

      {event.status === "draft" || event.status === "published" ? (
        <EventForm initial={event} submitLabel="Save changes" onSubmit={save} />
      ) : (
        <p className="text-zinc-500">
          This event is {event.status} and can no longer be edited.
        </p>
      )}
    </main>
  );
}
