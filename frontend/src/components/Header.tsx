"use client";

import { useTranslations } from "next-intl";
import { Link, useRouter } from "@/i18n/navigation";
import { useAuth } from "@/lib/auth-context";
import LanguageSwitcher from "./LanguageSwitcher";

export default function Header() {
  const t = useTranslations("nav");
  const { user, loading, logout } = useAuth();
  const router = useRouter();

  return (
    <header className="sticky top-0 z-50 border-b border-bone/[0.09] bg-ink/95 backdrop-blur">
      <div className="mx-auto flex h-16 max-w-6xl items-center justify-between gap-6 px-5">
        <Link
          href="/"
          className="flex items-center whitespace-nowrap font-display text-xl font-black tracking-tight text-bone"
        >
          <span className="mr-1.5 text-registan-strong">✳</span>
          <span className="italic">Meetus</span>
          <span className="ml-1 hidden font-mono text-xs font-medium text-atlas sm:inline">
            .uz
          </span>
        </Link>

        <nav className="flex items-center gap-5 text-sm">
          <Link href="/events" className="text-dust transition-colors hover:text-bone">
            {t("explore")}
          </Link>
          {loading ? null : user ? (
            <>
              <Link
                href="/tickets"
                className="hidden text-dust transition-colors hover:text-bone sm:inline"
              >
                {t("tickets")}
              </Link>
              <Link
                href="/organizer"
                className="hidden text-dust transition-colors hover:text-bone sm:inline"
              >
                {t("organize")}
              </Link>
              {user.isAdmin ? (
                <Link
                  href="/admin"
                  className="hidden text-dust transition-colors hover:text-bone sm:inline"
                >
                  {t("admin")}
                </Link>
              ) : null}
              <Link
                href="/profile"
                className="flex items-center gap-2 text-dust transition-colors hover:text-bone"
              >
                {user.avatarUrl ? (
                  // eslint-disable-next-line @next/next/no-img-element
                  <img
                    src={user.avatarUrl}
                    alt=""
                    className="h-7 w-7 rounded-full border border-line"
                  />
                ) : (
                  <span className="flex h-7 w-7 items-center justify-center rounded-full bg-ink-raised text-xs font-semibold text-bone">
                    {user.name[0]}
                  </span>
                )}
                <span className="hidden md:inline">{user.name}</span>
              </Link>
              <button
                onClick={async () => {
                  await logout();
                  router.push("/");
                }}
                className="hidden text-dust transition-colors hover:text-pomegranate sm:inline"
              >
                {t("logOut")}
              </button>
            </>
          ) : (
            <Link
              href="/login"
              className="rounded-full bg-registan px-4 py-2 text-sm font-bold text-[#0A2320] shadow-[0_8px_22px_-8px_rgba(24,173,160,0.55)] transition-colors hover:bg-registan-strong"
            >
              {t("signIn")}
            </Link>
          )}
          <LanguageSwitcher />
        </nav>
      </div>
    </header>
  );
}
