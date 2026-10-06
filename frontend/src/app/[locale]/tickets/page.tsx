"use client";

import { useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import QRCode from "qrcode";
import { Link, useRouter } from "@/i18n/navigation";
import { useLiveQuery } from "@/lib/useLiveQuery";
import { api } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { formatEventDate } from "@/components/EventCard";
import AddToCalendar from "@/components/AddToCalendar";

type MyTicket = {
  code: string;
  qr: string;
  checkedInAt: string | null;
  eventId: number;
  eventTitle: string;
  eventStatus: string;
  startsAt: string;
  isOnline: boolean;
  onlineUrl: string | null;
  locationName: string | null;
  citySlug: string | null;
  coverUrl: string | null;
};

function TicketCard({ ticket }: { ticket: MyTicket }) {
  const t = useTranslations("tickets");
  const locale = useLocale();
  const [qrDataUrl, setQrDataUrl] = useState<string | null>(null);

  useEffect(() => {
    QRCode.toDataURL(ticket.qr, { width: 220, margin: 1 })
      .then(setQrDataUrl)
      .catch(() => setQrDataUrl(null));
  }, [ticket.qr]);

  return (
    <div className="flex flex-col items-center gap-5 rounded-card border border-line bg-ink-raised p-6 shadow-card sm:flex-row">
      <div className="shrink-0 rounded-xl bg-bone p-2">
        {ticket.eventStatus === "published" && qrDataUrl ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={qrDataUrl} alt={t("qrAlt", { title: ticket.eventTitle })} />
        ) : (
          <div className="flex h-[220px] w-[220px] items-center justify-center text-sm text-ink/50">
            {ticket.eventStatus !== "published" ? t("inactive") : t("qrUnavailable")}
          </div>
        )}
      </div>
      <div className="text-center sm:text-left">
        <Link
          href={`/events/${ticket.eventId}`}
          className="font-display text-xl font-bold text-bone hover:text-registan-strong"
        >
          {ticket.eventTitle}
        </Link>
        <p className="mt-1.5 font-mono text-sm text-registan-strong">
          {formatEventDate(ticket.startsAt, locale)}
        </p>
        <p className="text-sm text-dust">
          {ticket.isOnline
            ? t("online")
            : (ticket.locationName ?? ticket.citySlug ?? "")}
        </p>
        <p className="mt-2 font-mono text-xs text-dust-dim">{ticket.code}</p>
        {ticket.eventStatus === "published" && ticket.isOnline && ticket.onlineUrl ? (
          <a
            href={ticket.onlineUrl}
            target="_blank"
            rel="noreferrer"
            className="btn btn-primary btn-sm mt-2.5"
          >
            {t("joinCall")}
          </a>
        ) : null}
        {ticket.checkedInAt ? (
          <p className="mt-2.5 inline-block rounded-full border border-registan-dim bg-registan/[0.12] px-3 py-1 text-xs font-semibold text-registan-strong">
            {t("checkedIn")}
          </p>
        ) : ticket.eventStatus === "canceled" ? (
          <p className="mt-2.5 inline-block rounded-full border border-pomegranate/35 bg-pomegranate/[0.12] px-3 py-1 text-xs font-semibold text-pomegranate">
            {t("eventCanceled")}
          </p>
        ) : (
          <p className="mt-2.5 text-xs text-dust-dim">{t("showAtEntrance")}</p>
        )}
        {ticket.eventStatus === "published" ? (
          <AddToCalendar
            className="mt-3"
            path={`/${locale}/events/${ticket.eventId}`}
            event={{
              id: ticket.eventId,
              title: ticket.eventTitle,
              description: "",
              startsAt: ticket.startsAt,
              endsAt: null,
              isOnline: ticket.isOnline,
              onlineUrl: ticket.onlineUrl,
              locationName: ticket.locationName,
              address: null,
              citySlug: ticket.citySlug,
            }}
          />
        ) : null}
      </div>
    </div>
  );
}

export default function TicketsPage() {
  const t = useTranslations("tickets");
  const { user, loading } = useAuth();
  const router = useRouter();
  const query = useCallback((signal: AbortSignal) => api<MyTicket[]>("/me/tickets", { auth: true, signal }), []);
  const { data: tickets, error, refresh } = useLiveQuery(user ? String(user.id) : null, query);
  const [offline, setOffline] = useState(false);
  useEffect(() => {
    const sync = () => setOffline(!navigator.onLine);
    sync(); window.addEventListener("online", sync); window.addEventListener("offline", sync);
    return () => { window.removeEventListener("online", sync); window.removeEventListener("offline", sync); };
  }, []);

  useEffect(() => {
    if (!loading && !user) router.replace("/login?next=/tickets");
  }, [loading, user, router]);

  if (loading || !user || (tickets === undefined && !error)) {
    return <main className="p-8 text-center text-dust">{t("loading")}</main>;
  }

  const active = tickets?.filter(ticket => ticket.eventStatus === "published" && !ticket.checkedInAt) ?? [];
  const history = tickets?.filter(ticket => ticket.eventStatus !== "published" || !!ticket.checkedInAt) ?? [];

  return (
    <main className="mx-auto max-w-2xl px-5 py-12">
      <h1 className="mb-4 font-display text-2xl font-black text-bone">{t("title")}</h1>
      <div className="mb-5 flex flex-wrap items-center justify-between gap-3 rounded-card border border-line bg-ink-raised p-4">
        <p role="status" className="min-w-0 flex-1 text-sm text-dust">{offline ? t("offlineNotice") : t("syncNotice")}</p>
        <button onClick={refresh} disabled={offline} className="btn btn-secondary btn-sm">{t("refresh")}</button>
      </div>
      {error ? <p role="alert" className="mb-5 text-pomegranate">{t("loadFailed")}</p> : null}
      {tickets?.length === 0 ? (
        <p className="rounded-card border border-dashed border-line p-10 text-center text-dust">
          {t("empty")}{" "}
          <Link href="/events" className="text-registan-strong hover:underline">
            {t("exploreLink")}
          </Link>{" "}
          {t("andJoinOne")}
        </p>
      ) : (
        <div className="flex flex-col gap-4">
          {active.length ? <h2 className="font-display text-lg font-bold">{t("readyTickets")}</h2> : null}
          {active.map(ticket => <TicketCard key={ticket.code} ticket={ticket} />)}
          {history.length ? <details className="rounded-card border border-line p-4"><summary className="cursor-pointer py-2 font-semibold text-dust">{t("history", { count: history.length })}</summary><div className="mt-4 flex flex-col gap-4">{history.map(ticket => <TicketCard key={ticket.code} ticket={ticket} />)}</div></details> : null}
        </div>
      )}
    </main>
  );
}
