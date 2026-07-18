import Link from "next/link";

export default function HomePage() {
  return (
    <main className="mx-auto flex max-w-3xl flex-col items-center gap-6 px-4 py-24 text-center">
      <h1 className="text-4xl font-bold tracking-tight sm:text-5xl">
        Find your people in{" "}
        <span className="text-sky-500">Uzbekistan</span>
      </h1>
      <p className="max-w-xl text-lg text-zinc-500">
        Discover meetups, workshops, and communities near you. RSVP in one
        click, get your QR ticket, and show up.
      </p>
      <div className="flex gap-3">
        <Link
          href="/events"
          className="rounded-lg bg-sky-500 px-5 py-2.5 font-medium text-white hover:bg-sky-600"
        >
          Explore events
        </Link>
        <Link
          href="/login"
          className="rounded-lg border border-zinc-300 px-5 py-2.5 font-medium hover:border-sky-500 hover:text-sky-500 dark:border-zinc-700"
        >
          Sign in
        </Link>
      </div>
    </main>
  );
}
