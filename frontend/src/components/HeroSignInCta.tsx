"use client";

import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
import { useAuth } from "@/lib/auth-context";

/** The hero's secondary CTA — only makes sense to show "Sign in" to a
 * signed-out visitor; the Header already covers navigation once logged in. */
export default function HeroSignInCta() {
  const t = useTranslations("home");
  const { user, loading } = useAuth();

  if (loading || user) return null;

  return (
    <Link
      href="/login"
      className="rounded-full border border-line px-6 py-3 text-base font-bold text-bone transition-colors hover:border-registan-strong hover:text-registan-strong"
    >
      {t("signIn")}
    </Link>
  );
}
