"use client";

import { errorMessage } from "@/lib/errorMessage";

import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
import { useLiveQuery } from "@/lib/useLiveQuery";
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
  onlineUrl: string | null;
};

type Props = {
  eventId: number;
  spotsLeft: number | null;
  isPast: boolean;
};

export default function RsvpSection({ eventId, spotsLeft, isPast }: Props) {
  const t = useTranslations("rsvp");
  const tErrors = useTranslations("errors");
  const { user, loading } = useAuth();
  const query = useCallback(async (signal: AbortSignal) => {
    try { return await api<RSVPState>(`/events/${eventId}/rsvp`, { auth: true, signal }); }
    catch (error) { if (error instanceof ApiError && error.status === 404) return null; throw error; }
  }, [eventId]);
  const { data: rsvp, error: syncFailed, refresh } = useLiveQuery(user ? `${user.id}:${eventId}` : null, query);
  const checked = !user || rsvp !== undefined;
  const [cancelStatus, setCancelStatus] = useState<RSVPState["status"] | null>(null);
  const keepButton = useRef<HTMLButtonElement>(null);
  useEffect(() => { if (cancelStatus) keepButton.current?.focus(); }, [cancelStatus]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [inMiniApp, setInMiniApp] = useState(false);

  useEffect(() => {
    setInMiniApp(isTelegramMiniApp());
  }, []);

  const isFull = spotsLeft === 0;

  const join = useCallback(async () => {
    setBusy(true);
    setError(null);
    try {
      await api<RSVPState>(`/events/${eventId}/rsvp`, {
        method: "POST",
        auth: true,
      });
      refresh();
    } catch (e) {
      setError(errorMessage(e, tErrors, t("joinFailed")));
    } finally {
      setBusy(false);
    }
  }, [eventId, refresh, t, tErrors]);

  // Inside Telegram, joining happens through the native MainButton instead
  // of the in-page button, so it feels like a first-class Telegram action.
  // Only shown once there's actually something to join — a full event
  // still gets the button, joining the waitlist instead of a confirmed spot.
  const canJoinViaMainButton =
    inMiniApp && checked && !loading && !!user && !rsvp && !isPast && !syncFailed;

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
    document.documentElement.dataset.telegramAction = "true";
    return () => {
      delete document.documentElement.dataset.telegramAction;
      tg.MainButton.offClick(join);
      tg.MainButton.hide();
      tg.MainButton.hideProgress();
      tg.MainButton.enable();
    };
  }, [canJoinViaMainButton, isFull, join, t]);

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

  if (loading || (!checked && !syncFailed)) return <p role="status" className="mt-8 rounded-card border border-line bg-ink-raised p-5 text-dust">{t("checking")}</p>;
  if (syncFailed) return <div role="alert" className="mt-8 rounded-card border border-line bg-ink-raised p-5"><p className="text-dust">{t("syncFailed")}</p><button onClick={refresh} className="btn btn-secondary mt-3">{t("retry")}</button></div>;

  if (isPast && !rsvp) {
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
          href={`/login?next=${encodeURIComponent(`/events/${eventId}`)}`}
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
      setCancelStatus(null);
      refresh();
    } catch (e) {
      setError(errorMessage(e, tErrors, t("cancelFailed")));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="mt-8" aria-live="polite" aria-busy={busy}>
      {rsvp?.status === "going" ? (
        <div className="rounded-card border border-registan-dim bg-registan/[0.1] p-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <p className="font-semibold text-registan-strong">
              {t("goingMessage")}{" "}
              <Link href="/tickets" className="underline">
                {t("viewTicket")}
              </Link>
            </p>
            <button
              onClick={() => setCancelStatus(rsvp.status)}
              disabled={busy || isPast}
              className="btn btn-danger-ghost btn-sm"
            >
              {t("cancel")}
            </button>
          </div>
          {rsvp.onlineUrl ? (
            <a
              href={rsvp.onlineUrl}
              target="_blank"
              rel="noreferrer"
              className="btn btn-primary btn-sm mt-3"
            >
              {t("joinCall")}
            </a>
          ) : null}
        </div>
      ) : rsvp?.status === "waitlisted" ? (
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-card border border-line bg-ink-raised p-4">
          <p className="font-semibold text-dust">{t("waitlistedMessage")}</p>
          <button
            onClick={() => setCancelStatus(rsvp.status)}
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
      {cancelStatus && cancelStatus === rsvp?.status ? <section role="group" aria-label={t("confirmTitle")} className="mt-4 rounded-card border border-pomegranate/35 bg-ink-raised p-5" onKeyDown={event => { if (event.key === "Escape" && !busy) setCancelStatus(null); }}>
        <h2 className="font-semibold text-bone">{t("confirmTitle")}</h2>
        <p className="mt-2 text-sm text-dust">{t(cancelStatus === "going" ? "cancelWarning" : "waitlistWarning")}</p>
        <div className="mt-4 flex flex-wrap gap-3">
          <button ref={keepButton} onClick={() => setCancelStatus(null)} disabled={busy} className="btn btn-secondary">{t("keepPlace")}</button>
          <button onClick={leave} disabled={busy} className="btn btn-danger-ghost">{t("confirmCancel")}</button>
        </div>
      </section> : null}
      {error ? <p role="alert" className="mt-2.5 text-sm text-pomegranate">{error}</p> : null}
    </div>
  );
}
