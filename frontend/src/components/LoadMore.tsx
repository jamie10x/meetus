"use client";
import { useEffect, useRef, useState, type Dispatch, type SetStateAction } from "react";
import { useTranslations } from "next-intl";
import { api } from "@/lib/api";

export default function LoadMore<T extends { id: number }>({ path, rows, setRows }: {
  path: string; rows: T[]; setRows: Dispatch<SetStateAction<T[]>>;
}) {
  const t = useTranslations("common");
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);
  const [end, setEnd] = useState<number | null>(null);
  const pending = useRef<AbortController | null>(null);
  useEffect(() => () => pending.current?.abort(), [path]);
  const last = rows.at(-1)?.id;
  if (!last || rows.length % 50 !== 0 || end === last) return null;
  const load = async () => {
    if (pending.current) return;
    const controller = new AbortController(); pending.current = controller;
    setBusy(true); setFailed(false);
    try {
      const next = await api<T[]>(`${path}${path.includes("?") ? "&" : "?"}beforeId=${last}`, { auth: true, signal: controller.signal });
      if (controller.signal.aborted) return;
      if (next.length === 0) setEnd(last);
      setRows(current => current.at(-1)?.id === last && current[0]?.id === rows[0]?.id
        ? [...current, ...next.filter(item => !current.some(existing => existing.id === item.id))] : current);
    } catch { if (!controller.signal.aborted) setFailed(true); }
    finally { pending.current = null; setBusy(false); }
  };
  return <div className="mt-3 text-center">
    {failed && <p role="alert" className="text-pomegranate">{t("loadFailed")}</p>}
    <button disabled={busy} onClick={load} className="rounded-lg border border-line px-4 py-2 text-bone disabled:opacity-50">{busy ? t("loading") : t("loadMore")}</button>
  </div>;
}
