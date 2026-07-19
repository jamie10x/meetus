"use client";

import { useEffect } from "react";
import { usePathname, useRouter } from "@/i18n/navigation";
import { getTelegramWebApp } from "./telegram-webapp";

/**
 * Wires Telegram's native BackButton to in-app navigation — the Mini App
 * WebView has no browser chrome of its own, so without this there's no way
 * to go back. Hidden on the home page since there's nowhere to go back to.
 */
export function useTelegramBackButton() {
  const pathname = usePathname();
  const router = useRouter();

  useEffect(() => {
    const tg = getTelegramWebApp();
    if (!tg) return;

    if (pathname === "/") {
      tg.BackButton.hide();
      return;
    }

    const onClick = () => router.back();
    tg.BackButton.onClick(onClick);
    tg.BackButton.show();

    return () => {
      tg.BackButton.offClick(onClick);
    };
  }, [pathname, router]);
}
