"use client";

import { useTranslations } from "next-intl";
import { Link, usePathname } from "@/i18n/navigation";
import { useAuth } from "@/lib/auth-context";

export default function MobileNavigation() {
  const t = useTranslations("nav");
  const pathname = usePathname();
  const { user } = useAuth();
  const links = [
    { href: "/events", label: t("explore"), icon: "⌕" },
    { href: "/tickets", label: t("tickets"), icon: "▣" },
    { href: "/organizer", label: t("organize"), icon: "+" },
    { href: user ? "/profile" : "/login", label: user ? t("profile") : t("signIn"), icon: "○" },
  ];
  return <nav aria-label={t("mobileNavigation")} className="mobile-navigation fixed inset-x-0 bottom-0 z-40 grid grid-cols-4 border-t border-line bg-ink/95 px-2 pt-2 backdrop-blur sm:hidden" style={{ paddingBottom: "max(0.5rem, env(safe-area-inset-bottom))" }}>
    {links.map(link => {
      const active = pathname === link.href || pathname.startsWith(`${link.href}/`);
      return <Link key={link.href} href={link.href} aria-current={active ? "page" : undefined} className={`flex min-h-12 flex-col items-center justify-center gap-1 rounded-xl px-1 text-xs font-semibold ${active ? "bg-registan/15 text-registan-strong" : "text-dust"}`}>
        <span aria-hidden="true" className="text-xl leading-none">{link.icon}</span>{link.label}
      </Link>;
    })}
  </nav>;
}
