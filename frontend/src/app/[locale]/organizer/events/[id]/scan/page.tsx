"use client";

import { use, useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
import { Html5Qrcode } from "html5-qrcode";
import { api, ApiError } from "@/lib/api";

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
  const { id } = use(params);
  const [feedback, setFeedback] = useState<ScanFeedback | null>(null);
  const [cameraError, setCameraError] = useState<string | null>(null);
  const [count, setCount] = useState(0);
  // Debounce identical consecutive scans while the code sits in frame.
  const lastScanRef = useRef<{ value: string; at: number }>({ value: "", at: 0 });
  const busyRef = useRef(false);

  useEffect(() => {
    const scanner = new Html5Qrcode(READER_ID);
    let stopped = false;

    const onScan = async (qr: string) => {
      const now = Date.now();
      const last = lastScanRef.current;
      if (busyRef.current || (qr === last.value && now - last.at < 3000)) {
        return;
      }
      lastScanRef.current = { value: qr, at: now };
      busyRef.current = true;
      try {
        const result = await api<CheckInResult>("/checkin", {
          method: "POST",
          auth: true,
          body: { qr },
        });
        setFeedback({ kind: "success", result });
        setCount((c) => c + 1);
      } catch (e) {
        setFeedback({
          kind: "error",
          message: e instanceof ApiError ? e.message : t("checkInFailed"),
        });
      } finally {
        busyRef.current = false;
      }
    };

    scanner
      .start(
        { facingMode: "environment" },
        { fps: 8, qrbox: { width: 240, height: 240 } },
        onScan,
        () => undefined, // per-frame decode misses are normal
      )
      .catch(() => setCameraError(t("cameraError")));

    return () => {
      if (!stopped) {
        stopped = true;
        scanner.stop().catch(() => undefined);
      }
    };
    // Mount-once effect (opens the camera stream): intentionally excludes
    // `t` — re-running this on every render would restart the camera.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

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

      {cameraError ? (
        <p className="mt-4 rounded-lg border border-pomegranate/35 bg-pomegranate/[0.12] p-3 text-sm text-pomegranate">
          {cameraError}
        </p>
      ) : null}

      {feedback?.kind === "success" ? (
        <div className="mt-4 rounded-card border border-registan-dim bg-registan/[0.12] p-4 text-center">
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
        <div className="mt-4 rounded-card border border-pomegranate/35 bg-pomegranate/[0.12] p-4 text-center">
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
