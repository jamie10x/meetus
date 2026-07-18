"use client";

import { use, useEffect, useRef, useState } from "react";
import Link from "next/link";
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
          message: e instanceof ApiError ? e.message : "Check-in failed.",
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
      .catch(() =>
        setCameraError(
          "Camera unavailable. Allow camera access and reload the page.",
        ),
      );

    return () => {
      if (!stopped) {
        stopped = true;
        scanner.stop().catch(() => undefined);
      }
    };
  }, []);

  return (
    <main className="mx-auto max-w-md px-4 py-8">
      <div className="mb-4 flex items-center justify-between">
        <h1 className="text-xl font-bold">Check-in scanner</h1>
        <Link
          href={`/organizer/events/${id}/edit`}
          className="text-sm text-zinc-500 hover:text-sky-500"
        >
          ← Back to event
        </Link>
      </div>

      <div
        id={READER_ID}
        className="overflow-hidden rounded-2xl border border-zinc-200 dark:border-zinc-800"
      />

      {cameraError ? (
        <p className="mt-4 rounded-lg bg-red-50 p-3 text-sm text-red-700 dark:bg-red-950 dark:text-red-300">
          {cameraError}
        </p>
      ) : null}

      {feedback?.kind === "success" ? (
        <div className="mt-4 rounded-xl bg-green-50 p-4 text-center dark:bg-green-950">
          <p className="text-2xl">✅</p>
          <p className="font-semibold text-green-700 dark:text-green-300">
            {feedback.result.attendeeName}
          </p>
          <p className="text-sm text-green-600 dark:text-green-400">
            checked in to {feedback.result.eventTitle}
          </p>
        </div>
      ) : null}
      {feedback?.kind === "error" ? (
        <div className="mt-4 rounded-xl bg-red-50 p-4 text-center dark:bg-red-950">
          <p className="text-2xl">❌</p>
          <p className="text-sm font-medium text-red-700 dark:text-red-300">
            {feedback.message}
          </p>
        </div>
      ) : null}

      <p className="mt-4 text-center text-sm text-zinc-500">
        {count} checked in this session
      </p>
    </main>
  );
}
