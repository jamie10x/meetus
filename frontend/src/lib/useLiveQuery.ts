"use client";

import { useCallback, useEffect, useState } from "react";

/** Refresh shared server state after returning from the bot, reconnecting,
 * and while visible. Scope keys prevent an old account's data flashing. */
export function useLiveQuery<T>(key: string | null, query: (signal: AbortSignal) => Promise<T>) {
  const [version, setVersion] = useState(0);
  const [result, setResult] = useState<{ key: string; data?: T; error: boolean }>({ key: "", error: false });
  const refresh = useCallback(() => setVersion(value => value + 1), []);
  useEffect(() => {
    if (!key) return;
    const controller = new AbortController();
    let pending = false;
    const load = async () => {
      if (pending || document.visibilityState === "hidden") return;
      pending = true;
      try {
        const data = await query(controller.signal);
        if (!controller.signal.aborted) setResult({ key, data, error: false });
      } catch {
        if (!controller.signal.aborted) setResult(previous => ({ key, data: previous.key === key ? previous.data : undefined, error: true }));
      } finally { pending = false; }
    };
    void load();
    window.addEventListener("focus", load);
    window.addEventListener("online", load);
    document.addEventListener("visibilitychange", load);
    const timer = setInterval(load, 15_000);
    return () => {
      controller.abort(); clearInterval(timer);
      window.removeEventListener("focus", load);
      window.removeEventListener("online", load);
      document.removeEventListener("visibilitychange", load);
    };
  }, [key, query, version]);
  return { data: result.key === key ? result.data : undefined, error: result.key === key && result.error, refresh };
}
