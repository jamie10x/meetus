"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import TelegramLoginButton from "@/components/TelegramLoginButton";
import { useAuth } from "@/lib/auth-context";
import { ApiError } from "@/lib/api";
import type { TelegramAuthFields } from "@/lib/types";

export default function LoginPage() {
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
        setError(
          e instanceof ApiError ? e.message : "Sign in failed. Please retry.",
        );
      } finally {
        setSubmitting(false);
      }
    },
    [loginWithTelegram, router],
  );

  return (
    <main className="mx-auto flex max-w-md flex-col items-center gap-6 px-4 py-24 text-center">
      <h1 className="text-3xl font-bold">Sign in to Meetus</h1>
      <p className="text-zinc-500">
        Use your Telegram account — no passwords, no OTP codes.
      </p>

      {submitting ? (
        <p className="text-sm text-zinc-500">Signing you in…</p>
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
