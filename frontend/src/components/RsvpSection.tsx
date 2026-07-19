"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
import { api, ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";

type Ticket = {
  code: string;
  qr: string;
  checkedInAt: string | null;
};

type Props = {
  eventId: number;
  spotsLeft: number | null;
  isPast: boolean;
};

export default function RsvpSection({ eventId, spotsLeft, isPast }: Props) {
  const t = useTranslations("rsvp");
  const { user, loading } = useAuth();
  const [ticket, setTicket] = useState<Ticket | null>(null);
  const [checked, setChecked] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!user) {
      setChecked(true);
      return;
    }
    api<Ticket>(`/events/${eventId}/rsvp`, { auth: true })
      .then(setTicket)
      .catch(() => setTicket(null))
      .finally(() => setChecked(true));
  }, [user, eventId]);

  if (loading || !checked) return null;

  if (isPast) {
    return (
      <p className="mt-6 rounded-xl bg-zinc-100 p-4 text-center text-zinc-500 dark:bg-zinc-800">
        {t("eventStarted")}
      </p>
    );
  }

  if (!user) {
    return (
      <div className="mt-6 rounded-xl border border-sky-200 bg-sky-50 p-4 text-center dark:border-sky-900 dark:bg-sky-950">
        <Link href="/login" className="font-medium text-sky-600 hover:underline">
          {t("signInLink")}
        </Link>{" "}
        {t("signInSuffix")}
      </div>
    );
  }

  const join = async () => {
    setBusy(true);
    setError(null);
    try {
      setTicket(await api<Ticket>(`/events/${eventId}/rsvp`, {
        method: "POST",
        auth: true,
      }));
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("joinFailed"));
    } finally {
      setBusy(false);
    }
  };

  const leave = async () => {
    setBusy(true);
    setError(null);
    try {
      await api(`/events/${eventId}/rsvp`, { method: "DELETE", auth: true });
      setTicket(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("cancelFailed"));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="mt-6">
      {ticket ? (
        <div className="flex items-center justify-between rounded-xl border border-green-300 bg-green-50 p-4 dark:border-green-800 dark:bg-green-950">
          <p className="font-medium text-green-700 dark:text-green-300">
            {t("goingMessage")}{" "}
            <Link href="/tickets" className="underline">
              {t("viewTicket")}
            </Link>
          </p>
          <button
            onClick={leave}
            disabled={busy}
            className="text-sm text-zinc-500 hover:text-red-500 disabled:opacity-50"
          >
            {t("cancel")}
          </button>
        </div>
      ) : (
        <button
          onClick={join}
          disabled={busy || spotsLeft === 0}
          className="w-full rounded-xl bg-sky-500 px-6 py-3 text-lg font-semibold text-white hover:bg-sky-600 disabled:opacity-50"
        >
          {spotsLeft === 0 ? t("eventFull") : busy ? t("joining") : t("joinEvent")}
        </button>
      )}
      {error ? <p className="mt-2 text-sm text-red-600">{error}</p> : null}
    </div>
  );
}
