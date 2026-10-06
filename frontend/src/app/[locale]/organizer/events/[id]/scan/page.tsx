"use client";

import { errorMessage } from "@/lib/errorMessage";

import { use, useCallback, useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
import { Html5Qrcode } from "html5-qrcode";
import { useAuth } from "@/lib/auth-context";
import { api } from "@/lib/api";

type CheckInResult = {
  attendeeName: string;
  eventTitle: string;
  checkedInAt: string;
};

type ScanFeedback =
  | { kind: "success"; result: CheckInResult }
  | { kind: "error"; message: string };

const READER_ID = "qr-reader";

export default function ScanPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const t = useTranslations("scan");
  const tErrors = useTranslations("errors");
  const { id } = use(params);
  const [feedback, setFeedback] = useState<ScanFeedback | null>(null);
  const [cameraError, setCameraError] = useState<string | null>(null);
  const [count, setCount] = useState(0);
  const [manual, setManual] = useState("");
  const { user, loading } = useAuth();
  const cameraQueue = useRef<Promise<void>>(Promise.resolve());
  const mounted = useRef(false);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  // Debounce identical consecutive scans while the code sits in frame.
  const lastScanRef = useRef<{ value: string; at: number }>({ value: "", at: 0 });
  const busyRef = useRef(false);

  const onScan = useCallback(async (qr: string) => {
    const now = Date.now();
    const last = lastScanRef.current;
    if (!user || busyRef.current || (qr === last.value && now - last.at < 3000)) return;
    lastScanRef.current = { value: qr, at: now };
    busyRef.current = true;
    try {
      const result = await api<CheckInResult>("/checkin", {
        method: "POST", auth: true, body: { qr, eventId: Number(id) },
      });
      if (mounted.current) { setFeedback({ kind: "success", result }); setCount(c => c + 1); }
    } catch (error) {
      if (mounted.current) setFeedback({ kind: "error", message: errorMessage(error, tErrors, t("checkInFailed")) });
    } finally { busyRef.current = false; }
  }, [id, user, t, tErrors]);

  useEffect(() => {
    if (loading || !user) return;
    let disposed = false;
    let scanner: Html5Qrcode | undefined;
    const start = cameraQueue.current.then(async () => {
      if (disposed) return;
      scanner = new Html5Qrcode(READER_ID);
      try {
        await scanner.start({ facingMode: "environment" }, { fps: 8, qrbox: { width: 240, height: 240 } }, qr => { if (!disposed) void onScan(qr); }, () => undefined);
      } catch { if (!disposed) setCameraError(t("cameraError")); }
    });
    cameraQueue.current = start;
    return () => {
      disposed = true;
      cameraQueue.current = start.then(async () => {
        if (scanner?.isScanning) await scanner.stop().catch(() => undefined);
        scanner?.clear();
      }).catch(() => undefined);
    };
  }, [loading, user, onScan, t]);

  return (
    <main className="mx-auto max-w-md px-4 py-8">
      <div className="mb-4 flex items-center justify-between">
        <h1 className="text-xl font-bold text-bone">{t("title")}</h1>
        <Link
          href={`/organizer/events/${id}/edit`}
          className="text-sm text-dust hover:text-registan-strong"
        >
          {t("back")}
        </Link>
      </div>

      <div
        id={READER_ID}
        className="overflow-hidden rounded-card border border-line"
      />

      <form className="mt-4 flex gap-2" onSubmit={event => { event.preventDefault(); if (manual.trim()) void onScan(manual.trim()); }}>
        <input aria-label={t("manualCode")} placeholder={t("manualCode")} value={manual} onChange={event => setManual(event.target.value)} maxLength={200} className="min-w-0 flex-1 rounded-lg border border-line bg-ink p-2 text-bone" />
        <button disabled={loading || !user || !manual.trim()} className="rounded-lg bg-registan px-3 text-ink disabled:opacity-50">{t("submitCode")}</button>
      </form>
      {cameraError ? (
        <p className="mt-4 rounded-lg border border-pomegranate/35 bg-pomegranate/[0.12] p-3 text-sm text-pomegranate">
          {cameraError}
        </p>
      ) : null}

      {feedback?.kind === "success" ? (
        <div role="status" className="mt-4 rounded-card border border-registan-dim bg-registan/[0.12] p-4 text-center">
          <p className="text-2xl">✅</p>
          <p className="font-semibold text-registan-strong">
            {feedback.result.attendeeName}
          </p>
          <p className="text-sm text-registan-strong">
            {t("checkedInTo", { eventTitle: feedback.result.eventTitle })}
          </p>
        </div>
      ) : null}
      {feedback?.kind === "error" ? (
        <div role="status" className="mt-4 rounded-card border border-pomegranate/35 bg-pomegranate/[0.12] p-4 text-center">
          <p className="text-2xl">❌</p>
          <p className="text-sm font-medium text-pomegranate">
            {feedback.message}
          </p>
        </div>
      ) : null}

      <p className="mt-4 text-center text-sm text-dust">
        {t("sessionCount", { count })}
      </p>
    </main>
  );
}
