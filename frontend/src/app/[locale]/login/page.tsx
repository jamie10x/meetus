"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { useRouter } from "@/i18n/navigation";
import TelegramLoginButton from "@/components/TelegramLoginButton";
import { useAuth } from "@/lib/auth-context";
import { ApiError } from "@/lib/api";
import type { TelegramAuthFields } from "@/lib/types";

export default function LoginPage() {
  const t = useTranslations("login");
  const { user, loading, loginWithTelegram } = useAuth();
  const router = useRouter();
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!loading && user) router.replace("/");
  }, [loading, user, router]);

  const handleAuth = useCallback(
    async (fields: TelegramAuthFields) => {
      setSubmitting(true);
      setError(null);
      try {
        await loginWithTelegram(fields);
        router.replace("/");
      } catch (e) {
        setError(e instanceof ApiError ? e.message : t("signInFailed"));
      } finally {
        setSubmitting(false);
      }
    },
    [loginWithTelegram, router, t],
  );

  // Avoids flashing the Login Widget while a Mini App silent auto-login
  // (AuthProvider) is still in flight — resolves quickly either way.
  if (loading) {
    return (
      <main className="mx-auto flex max-w-md flex-col items-center gap-6 px-5 py-28 text-center">
        <p className="text-sm text-dust">{t("signingIn")}</p>
      </main>
    );
  }

  return (
    <main className="mx-auto flex max-w-md flex-col items-center gap-6 px-5 py-28 text-center">
      <h1 className="font-display text-3xl font-black text-bone">{t("title")}</h1>
      <p className="text-dust">{t("subtitle")}</p>

      {submitting ? (
        <p className="text-sm text-dust">{t("signingIn")}</p>
      ) : (
        <TelegramLoginButton onAuth={handleAuth} />
      )}

      {error ? (
        <p className="rounded-card border border-pomegranate/35 bg-pomegranate/[0.12] p-3 text-sm text-pomegranate">
          {error}
        </p>
      ) : null}
    </main>
  );
}
