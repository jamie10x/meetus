"use client";

import { useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { api } from "@/lib/api";

type Health = {
  worker: { status: "ready" | "stale" | "disabled"; lastProgressAt: string | null; revision: string | null };
  delivery: { pending: number; failed: number; oldestPendingAt: string | null };
};

export default function OperationsStatus() {
  const t = useTranslations("operations");
  const locale = useLocale();
  const [health, setHealth] = useState<Health | null>(null);
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    const controller = new AbortController();
    let timer: ReturnType<typeof setTimeout>;
    const load = async () => {
      try {
        const next = await api<Health>("/admin/operations", { auth: true, signal: controller.signal });
        if (!controller.signal.aborted) { setHealth(next); setFailed(false); }
      } catch { if (!controller.signal.aborted) setFailed(true); }
      if (!controller.signal.aborted) timer = setTimeout(load, 30_000);
    };
    void load();
    return () => { controller.abort(); clearTimeout(timer); };
  }, []);
  const date = (value: string | null) => value ? new Date(value).toLocaleString(locale) : t("none");
  return <section className="mb-8 rounded-card border border-line bg-ink-raised p-4" aria-label={t("title")}>
    <h2 className="mb-3 text-lg font-semibold text-bone">{t("title")}</h2>
    {failed ? <p role="alert" className="text-pomegranate">{t("unavailable")}</p> : !health ? <p role="status" className="text-dust">{t("loading")}</p> :
      <dl className="grid gap-3 text-sm sm:grid-cols-2">
        <div><dt className="text-dust">{t("worker")}</dt><dd className={health.worker.status === "stale" ? "text-pomegranate" : "text-bone"}>{t(health.worker.status)}</dd></div>
        <div><dt className="text-dust">{t("lastProgress")}</dt><dd className="text-bone">{date(health.worker.lastProgressAt)}</dd></div>
        <div><dt className="text-dust">{t("pending")}</dt><dd className="text-bone">{health.delivery.pending}</dd></div>
        <div><dt className="text-dust">{t("failed")}</dt><dd className={health.delivery.failed ? "text-pomegranate" : "text-bone"}>{health.delivery.failed}</dd></div>
        <div><dt className="text-dust">{t("oldest")}</dt><dd className="text-bone">{date(health.delivery.oldestPendingAt)}</dd></div>
      </dl>}
  </section>;
}
