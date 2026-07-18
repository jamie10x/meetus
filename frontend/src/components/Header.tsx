"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth-context";

export default function Header() {
  const { user, loading, logout } = useAuth();
  const router = useRouter();

  return (
    <header className="border-b border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-950">
      <div className="mx-auto flex h-14 max-w-5xl items-center justify-between px-4">
        <Link href="/" className="text-lg font-bold tracking-tight">
          meetus<span className="text-sky-500">.uz</span>
        </Link>

        <nav className="flex items-center gap-4 text-sm">
          <Link href="/events" className="hover:text-sky-500">
            Explore
          </Link>
          {loading ? null : user ? (
            <>
              <Link href="/tickets" className="hover:text-sky-500">
                Tickets
              </Link>
              <Link href="/organizer" className="hover:text-sky-500">
                Organize
              </Link>
              {user.isAdmin ? (
                <Link href="/admin" className="hover:text-sky-500">
                  Admin
                </Link>
              ) : null}
              <Link
                href="/profile"
                className="flex items-center gap-2 hover:text-sky-500"
              >
                {user.avatarUrl ? (
                  // eslint-disable-next-line @next/next/no-img-element
                  <img
                    src={user.avatarUrl}
                    alt=""
                    className="h-7 w-7 rounded-full"
                  />
                ) : null}
                {user.name}
              </Link>
              <button
                onClick={async () => {
                  await logout();
                  router.push("/");
                }}
                className="text-zinc-500 hover:text-red-500"
              >
                Log out
              </button>
            </>
          ) : (
            <Link
              href="/login"
              className="rounded-lg bg-sky-500 px-3 py-1.5 font-medium text-white hover:bg-sky-600"
            >
              Sign in
            </Link>
          )}
        </nav>
      </div>
    </header>
  );
}
