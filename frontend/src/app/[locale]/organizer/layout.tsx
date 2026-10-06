"use client";

import { Fragment, useEffect, type ReactNode } from "react";
import { useTranslations } from "next-intl";
import { usePathname, useRouter } from "@/i18n/navigation";
import { useAuth } from "@/lib/auth-context";

export default function OrganizerLayout({ children }: { children: ReactNode }) {
  const { user, loading } = useAuth();
  const router = useRouter();
  const pathname = usePathname();
  const t = useTranslations("common");
  useEffect(() => {
    if (!loading && !user) router.replace(`/login?next=${encodeURIComponent(pathname)}`);
  }, [loading, user, router, pathname]);
  if (loading || !user) return <main role="status" className="p-8 text-center text-dust">{t("loading")}</main>;
  // Account switches must discard the previous organizer's local form state.
  return <Fragment key={user.id}>{children}</Fragment>;
}
