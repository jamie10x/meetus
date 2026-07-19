"use client";

import { useEffect } from "react";
import { useTelegramBackButton } from "@/lib/useTelegramBackButton";
import { getTelegramWebApp } from "@/lib/telegram-webapp";

// Matches the light/dark backgrounds the rest of the app already uses
// (bg-white / dark:bg-zinc-950 in Header.tsx and elsewhere), so Telegram's
// own chrome — the header bar and the area behind the WebView — blends
// with the page instead of showing Telegram's default color. Deliberately
// not touching Tailwind's own dark-mode media query here: this only tells
// Telegram what our page looks like, it doesn't change what the page is.
const LIGHT_BG = "#ffffff";
const DARK_BG = "#09090b";

/** Renders nothing — mounts the Mini App native-chrome side effects (back
 * button wiring, header/background color sync) once near the app root. */
export default function TelegramChrome() {
  useTelegramBackButton();

  useEffect(() => {
    const tg = getTelegramWebApp();
    if (!tg) return;
    const bg = tg.colorScheme === "dark" ? DARK_BG : LIGHT_BG;
    tg.setHeaderColor(bg);
    tg.setBackgroundColor(bg);
  }, []);

  return null;
}
