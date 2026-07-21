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

type RSVPState = {
  status: "going" | "waitlisted";
  ticket: Ticket | null;
};

type Props = {
  eventId: number;
  spotsLeft: number | null;
  isPast: boolean;
};

export default function RsvpSection({ eventId, spotsLeft, isPast }: Props) {
  const t = useTranslations("rsvp");
  const { user, loading } = useAuth();
  const [rsvp, setRsvp] = useState<RSVPState | null>(null);
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
    api<RSVPState>(`/events/${eventId}/rsvp`, { auth: true })
      .then(setRsvp)
      .catch(() => setRsvp(null))
      .finally(() => setChecked(true));
  }, [user, eventId]);

  const isFull = spotsLeft === 0;

  const join = async () => {
    setBusy(true);
    setError(null);
    try {
      setRsvp(await api<RSVPState>(`/events/${eventId}/rsvp`, {
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
  // Only shown once there's actually something to join — a full event
  // still gets the button, joining the waitlist instead of a confirmed spot.
  const canJoinViaMainButton =
    inMiniApp && checked && !loading && !!user && !rsvp && !isPast;

  useEffect(() => {
    const tg = getTelegramWebApp();
    if (!tg) return;
    if (!canJoinViaMainButton) {
      tg.MainButton.hide();
      return;
    }
    tg.MainButton.setText(isFull ? t("joinWaitlist") : t("joinEvent"));
    tg.MainButton.onClick(join);
    tg.MainButton.show();
    return () => {
      tg.MainButton.offClick(join);
    };
    // join is intentionally omitted — it's recreated every render but does
    // the same thing each time, and re-subscribing on every render would
    // thrash Telegram's native button.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [canJoinViaMainButton, isFull]);

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
      setRsvp(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("cancelFailed"));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="mt-8">
      {rsvp?.status === "going" ? (
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
            className="btn btn-danger-ghost btn-sm"
          >
            {t("cancel")}
          </button>
        </div>
      ) : rsvp?.status === "waitlisted" ? (
        <div className="flex items-center justify-between rounded-card border border-line bg-ink-raised p-4">
          <p className="font-semibold text-dust">{t("waitlistedMessage")}</p>
          <button
            onClick={leave}
            disabled={busy}
            className="btn btn-danger-ghost btn-sm"
          >
            {t("leaveWaitlist")}
          </button>
        </div>
      ) : canJoinViaMainButton ? (
        busy ? (
          <p className="text-center text-sm text-dust">{t("joining")}</p>
        ) : null
      ) : (
        <button
          onClick={join}
          disabled={busy}
          className="btn btn-primary w-full text-lg"
        >
          {busy ? t("joining") : isFull ? t("joinWaitlist") : t("joinEvent")}
        </button>
      )}
      {error ? <p className="mt-2.5 text-sm text-pomegranate">{error}</p> : null}
    </div>
  );
}
