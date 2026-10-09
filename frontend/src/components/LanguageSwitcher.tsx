"use client";

import { useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { usePathname, useRouter } from "@/i18n/navigation";
import { routing } from "@/i18n/routing";
import { useAuth } from "@/lib/auth-context";
import { confirmNavigation } from "@/lib/useUnsavedChanges";
import { api } from "@/lib/api";
import type { User } from "@/lib/types";

export default function LanguageSwitcher() {
  const t = useTranslations("languageSwitcher");
  const locale = useLocale();
  const pathname = usePathname();
  const router = useRouter();
  const { user, setUser } = useAuth();
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);
  const change = async (language: string) => {
    if (!confirmNavigation()) return;
    setBusy(true); setFailed(false);
    try {
      if (user) setUser(await api<User>("/me", { method: "PATCH", auth: true, body: { language } }));
      router.replace(`${pathname}${window.location.search}${window.location.hash}`, { locale: language });
    } catch { setFailed(true); }
    finally { setBusy(false); }
  };
  return <div className="relative">
    <select value={locale} disabled={busy} onChange={event => void change(event.target.value)} aria-label={t("label")}
      className="min-h-11 rounded-full border border-line bg-ink-raised px-2.5 text-xs font-medium text-dust">
      {routing.locales.map(language => <option key={language} value={language}>{t(language)}</option>)}
    </select>
    {failed ? <p role="alert" className="absolute right-0 top-full mt-2 w-56 rounded-xl border border-line bg-ink-raised p-3 text-sm text-pomegranate">{t("saveFailed")}</p> : null}
  </div>;
}
