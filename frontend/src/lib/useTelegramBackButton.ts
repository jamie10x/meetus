"use client";

import { useEffect, useRef } from "react";
import { usePathname, useRouter } from "@/i18n/navigation";
import { confirmNavigation } from "./useUnsavedChanges";
import { getTelegramWebApp } from "./telegram-webapp";

/**
 * Wires Telegram's native BackButton to in-app navigation — the Mini App
 * WebView has no browser chrome of its own, so without this there's no way
 * to go back. Hidden on the home page since there's nowhere to go back to.
 */
export function useTelegramBackButton() {
  const pathname = usePathname();
  const router = useRouter();
  const initialPath = useRef(pathname);

  useEffect(() => {
    const tg = getTelegramWebApp();
    if (!tg) return;

    if (pathname === "/") {
      tg.BackButton.hide();
      return;
    }

    const onClick = () => {
      if (!confirmNavigation()) return;
      if (pathname === initialPath.current) router.replace("/events");
      else router.back();
    };
    tg.BackButton.onClick(onClick);
    tg.BackButton.show();

    return () => {
      tg.BackButton.offClick(onClick);
      tg.BackButton.hide();
    };
  }, [pathname, router]);
}
