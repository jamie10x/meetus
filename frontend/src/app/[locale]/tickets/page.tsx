"use client";

import { useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import QRCode from "qrcode";
import { Link, useRouter } from "@/i18n/navigation";
import { api } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { formatEventDate } from "@/components/EventCard";

type MyTicket = {
  code: string;
  qr: string;
  checkedInAt: string | null;
  eventId: number;
  eventTitle: string;
  eventStatus: string;
  startsAt: string;
  isOnline: boolean;
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
        {qrDataUrl ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={qrDataUrl} alt={`Ticket QR for ${ticket.eventTitle}`} />
        ) : (
          <div className="flex h-[220px] w-[220px] items-center justify-center text-sm text-ink/50">
            {t("qrUnavailable")}
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
      </div>
    </div>
  );
}

export default function TicketsPage() {
  const t = useTranslations("tickets");
  const { user, loading } = useAuth();
  const router = useRouter();
  const [tickets, setTickets] = useState<MyTicket[] | null>(null);

  useEffect(() => {
    if (!loading && !user) router.replace("/login");
  }, [loading, user, router]);

  useEffect(() => {
    if (!user) return;
    api<MyTicket[]>("/me/tickets", { auth: true })
      .then(setTickets)
      .catch(() => setTickets([]));
  }, [user]);

  if (loading || !user || tickets === null) {
    return <main className="p-8 text-center text-dust">{t("loading")}</main>;
  }

  return (
    <main className="mx-auto max-w-2xl px-5 py-12">
      <h1 className="mb-6 font-display text-2xl font-black text-bone">{t("title")}</h1>
      {tickets.length === 0 ? (
        <p className="rounded-card border border-dashed border-line p-10 text-center text-dust">
          {t("empty")}{" "}
          <Link href="/events" className="text-registan-strong hover:underline">
            {t("exploreLink")}
          </Link>{" "}
          {t("andJoinOne")}
        </p>
      ) : (
        <div className="flex flex-col gap-4">
          {tickets.map((ticket) => (
            <TicketCard key={ticket.code} ticket={ticket} />
          ))}
        </div>
      )}
    </main>
  );
}
