"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import QRCode from "qrcode";
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
  const [qrDataUrl, setQrDataUrl] = useState<string | null>(null);

  useEffect(() => {
    QRCode.toDataURL(ticket.qr, { width: 220, margin: 1 })
      .then(setQrDataUrl)
      .catch(() => setQrDataUrl(null));
  }, [ticket.qr]);

  return (
    <div className="flex flex-col items-center gap-4 rounded-2xl border border-zinc-200 p-6 sm:flex-row dark:border-zinc-800">
      <div className="shrink-0 rounded-xl bg-white p-2">
        {qrDataUrl ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={qrDataUrl} alt={`Ticket QR for ${ticket.eventTitle}`} />
        ) : (
          <div className="flex h-[220px] w-[220px] items-center justify-center text-sm text-zinc-400">
            QR unavailable
          </div>
        )}
      </div>
      <div className="text-center sm:text-left">
        <Link
          href={`/events/${ticket.eventId}`}
          className="text-xl font-semibold hover:text-sky-500"
        >
          {ticket.eventTitle}
        </Link>
        <p className="mt-1 text-sm text-zinc-500">
          {formatEventDate(ticket.startsAt)}
        </p>
        <p className="text-sm text-zinc-500">
          {ticket.isOnline
            ? "Online"
            : (ticket.locationName ?? ticket.citySlug ?? "")}
        </p>
        <p className="mt-2 font-mono text-xs text-zinc-400">{ticket.code}</p>
        {ticket.checkedInAt ? (
          <p className="mt-2 inline-block rounded-full bg-green-100 px-3 py-1 text-xs font-medium text-green-700 dark:bg-green-900 dark:text-green-300">
            Checked in ✓
          </p>
        ) : ticket.eventStatus === "canceled" ? (
          <p className="mt-2 inline-block rounded-full bg-red-100 px-3 py-1 text-xs font-medium text-red-700 dark:bg-red-900 dark:text-red-300">
            Event canceled
          </p>
        ) : (
          <p className="mt-2 text-xs text-zinc-400">
            Show this QR at the entrance
          </p>
        )}
      </div>
    </div>
  );
}

export default function TicketsPage() {
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
    return <main className="p-8 text-center text-zinc-500">Loading…</main>;
  }

  return (
    <main className="mx-auto max-w-2xl px-4 py-10">
      <h1 className="mb-6 text-2xl font-bold">My tickets</h1>
      {tickets.length === 0 ? (
        <p className="rounded-lg border border-dashed border-zinc-300 p-10 text-center text-zinc-500 dark:border-zinc-700">
          No tickets yet.{" "}
          <Link href="/events" className="text-sky-500 hover:underline">
            Explore events
          </Link>{" "}
          and join one!
        </p>
      ) : (
        <div className="flex flex-col gap-4">
          {tickets.map((t) => (
            <TicketCard key={t.code} ticket={t} />
          ))}
        </div>
      )}
    </main>
  );
}
