"use client";

import { useEffect } from "react";
import { useTelegramBackButton } from "@/lib/useTelegramBackButton";
import { getTelegramWebApp } from "@/lib/telegram-webapp";

// Matches the app's committed dark-first "ink" background (--color-ink in
// globals.css), so Telegram's own chrome — the header bar and the area
// behind the WebView — blends with the page instead of showing Telegram's
// default color. The app no longer follows Telegram's own colorScheme
// (or the OS light/dark preference) — the brand is dark-first everywhere.
const INK_BG = "#160f16";

/** Renders nothing — mounts the Mini App native-chrome side effects (back
 * button wiring, header/background color sync) once near the app root. */
export default function TelegramChrome() {
  useTelegramBackButton();

  useEffect(() => {
    const tg = getTelegramWebApp();
    if (!tg) return;
    tg.setHeaderColor(INK_BG);
    tg.setBackgroundColor(INK_BG);
  }, []);

  return null;
}
