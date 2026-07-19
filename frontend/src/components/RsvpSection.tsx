"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
import { api, ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { getTelegramWebApp, isTelegramMiniApp } from "@/lib/telegram-webapp";

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
  const [inMiniApp, setInMiniApp] = useState(false);

  useEffect(() => {
    setInMiniApp(isTelegramMiniApp());
  }, []);

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

  // Inside Telegram, joining happens through the native MainButton instead
  // of the in-page button, so it feels like a first-class Telegram action.
  // Only shown once there's actually something to join.
  const canJoinViaMainButton =
    inMiniApp && checked && !loading && !!user && !ticket && !isPast && spotsLeft !== 0;

  useEffect(() => {
    const tg = getTelegramWebApp();
    if (!tg) return;
    if (!canJoinViaMainButton) {
      tg.MainButton.hide();
      return;
    }
    tg.MainButton.setText(t("joinEvent"));
    tg.MainButton.onClick(join);
    tg.MainButton.show();
    return () => {
      tg.MainButton.offClick(join);
    };
    // join is intentionally omitted — it's recreated every render but does
    // the same thing each time, and re-subscribing on every render would
    // thrash Telegram's native button.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [canJoinViaMainButton]);

  useEffect(() => {
    const tg = getTelegramWebApp();
    if (!tg || !canJoinViaMainButton) return;
    if (busy) {
      tg.MainButton.showProgress(false);
      tg.MainButton.disable();
    } else {
      tg.MainButton.hideProgress();
      tg.MainButton.enable();
    }
  }, [busy, canJoinViaMainButton]);

  if (loading || !checked) return null;

  if (isPast) {
    return (
      <p className="mt-8 rounded-card border border-line bg-ink-raised p-4 text-center text-dust">
        {t("eventStarted")}
      </p>
    );
  }

  if (!user) {
    return (
      <div className="mt-8 rounded-card border border-registan-dim bg-registan/[0.08] p-4 text-center">
        <Link
          href="/login"
          className="font-semibold text-registan-strong hover:underline"
        >
          {t("signInLink")}
        </Link>{" "}
        <span className="text-dust">{t("signInSuffix")}</span>
      </div>
    );
  }

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
    <div className="mt-8">
      {ticket ? (
        <div className="flex items-center justify-between rounded-card border border-registan-dim bg-registan/[0.1] p-4">
          <p className="font-semibold text-registan-strong">
            {t("goingMessage")}{" "}
            <Link href="/tickets" className="underline">
              {t("viewTicket")}
            </Link>
          </p>
          <button
            onClick={leave}
            disabled={busy}
            className="text-sm text-dust transition-colors hover:text-pomegranate disabled:opacity-50"
          >
            {t("cancel")}
          </button>
        </div>
      ) : canJoinViaMainButton ? (
        busy ? (
          <p className="text-center text-sm text-dust">{t("joining")}</p>
        ) : null
      ) : (
        <button
          onClick={join}
          disabled={busy || spotsLeft === 0}
          className="w-full rounded-full bg-registan px-6 py-3.5 text-lg font-bold text-[#0A2320] shadow-[0_8px_22px_-8px_rgba(24,173,160,0.55)] transition-colors hover:bg-registan-strong disabled:cursor-not-allowed disabled:opacity-50 disabled:shadow-none"
        >
          {spotsLeft === 0 ? t("eventFull") : busy ? t("joining") : t("joinEvent")}
        </button>
      )}
      {error ? <p className="mt-2.5 text-sm text-pomegranate">{error}</p> : null}
    </div>
  );
}
