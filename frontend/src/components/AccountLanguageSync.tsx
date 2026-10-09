"use client";

import { useEffect } from "react";
import { usePathname, useRouter } from "@/i18n/navigation";
import { useAuth } from "@/lib/auth-context";
import { hasUnsavedChanges } from "@/lib/useUnsavedChanges";
import { api } from "@/lib/api";
import type { User } from "@/lib/types";

/** Pick up explicit /language changes made in the bot when returning here. */
export default function AccountLanguageSync() {
  const { user, setUser } = useAuth();
  const router = useRouter();
  const pathname = usePathname();
  useEffect(() => {
    if (!user) return;
    const controller = new AbortController();
    let pending = false;
    const sync = async () => {
      if (hasUnsavedChanges() || pending || document.visibilityState === "hidden") return;
      pending = true;
      try {
        const current = await api<User>("/me", { auth: true, signal: controller.signal });
        if (!controller.signal.aborted && current.id === user.id && current.language !== user.language) {
          setUser(current);
          router.replace(`${pathname}${window.location.search}${window.location.hash}`, { locale: current.language });
        }
      } catch { /* Attendance and ticket screens expose their own retry state. */ }
      finally { pending = false; }
    };
    window.addEventListener("focus", sync);
    document.addEventListener("visibilitychange", sync);
    return () => { controller.abort(); window.removeEventListener("focus", sync); document.removeEventListener("visibilitychange", sync); };
  }, [user, pathname, router, setUser]);
  return null;
}
