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
      <main className="mx-auto flex max-w-md flex-col items-center gap-6 px-4 py-24 text-center">
        <p className="text-sm text-zinc-500">{t("signingIn")}</p>
      </main>
    );
  }

  return (
    <main className="mx-auto flex max-w-md flex-col items-center gap-6 px-4 py-24 text-center">
      <h1 className="text-3xl font-bold">{t("title")}</h1>
      <p className="text-zinc-500">{t("subtitle")}</p>

      {submitting ? (
        <p className="text-sm text-zinc-500">{t("signingIn")}</p>
      ) : (
        <TelegramLoginButton onAuth={handleAuth} />
      )}

      {error ? (
        <p className="rounded-lg border border-red-300 bg-red-50 p-3 text-sm text-red-700">
          {error}
        </p>
      ) : null}
    </main>
  );
}
