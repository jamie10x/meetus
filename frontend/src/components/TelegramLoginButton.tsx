"use client";

import { useEffect, useRef } from "react";
import { useTranslations } from "next-intl";
import type { TelegramAuthFields } from "@/lib/types";

declare global {
  interface Window {
    onTelegramAuth?: (user: Record<string, unknown>) => void;
  }
}

type Props = {
  onAuth: (fields: TelegramAuthFields) => void;
};

const BOT_USERNAME = process.env.NEXT_PUBLIC_TELEGRAM_BOT_USERNAME ?? "";

/**
 * Renders the official Telegram Login Widget. The widget calls the global
 * onTelegramAuth callback with the signed user payload, which we forward
 * to the backend for verification.
 */
export default function TelegramLoginButton({ onAuth }: Props) {
  const t = useTranslations("telegramLogin");
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!BOT_USERNAME || !containerRef.current) return;

    window.onTelegramAuth = (user) => {
      const fields: TelegramAuthFields = {};
      for (const [key, value] of Object.entries(user)) {
        fields[key] = String(value);
      }
      onAuth(fields);
    };

    const script = document.createElement("script");
    script.src = "https://telegram.org/js/telegram-widget.js?22";
    script.async = true;
    script.setAttribute("data-telegram-login", BOT_USERNAME);
    script.setAttribute("data-size", "large");
    script.setAttribute("data-radius", "12");
    script.setAttribute("data-onauth", "onTelegramAuth(user)");
    script.setAttribute("data-request-access", "write");

    const container = containerRef.current;
    container.appendChild(script);
    return () => {
      container.innerHTML = "";
      delete window.onTelegramAuth;
    };
  }, [onAuth]);

  if (!BOT_USERNAME) {
    return (
      <p className="rounded-lg border border-amber-300 bg-amber-50 p-4 text-sm text-amber-800">
        {t("notConfigured", {
          envVar: "NEXT_PUBLIC_TELEGRAM_BOT_USERNAME",
          envFile: "frontend/.env.local",
        })}
      </p>
    );
  }

  return <div ref={containerRef} className="flex justify-center" />;
}
