import { getTranslations } from "next-intl/server";
import { Link } from "@/i18n/navigation";
import TrendingSection from "@/components/TrendingSection";

export default async function HomePage() {
  const t = await getTranslations("home");

  return (
    <main className="mx-auto max-w-3xl px-4 py-16">
      <div className="flex flex-col items-center gap-6 py-8 text-center">
        <h1 className="text-4xl font-bold tracking-tight sm:text-5xl">
          {t("titlePart1")}{" "}
          <span className="text-sky-500">{t("titleHighlight")}</span>
        </h1>
        <p className="max-w-xl text-lg text-zinc-500">{t("subtitle")}</p>
        <div className="flex gap-3">
          <Link
            href="/events"
            className="rounded-lg bg-sky-500 px-5 py-2.5 font-medium text-white hover:bg-sky-600"
          >
            {t("exploreEvents")}
          </Link>
          <Link
            href="/login"
            className="rounded-lg border border-zinc-300 px-5 py-2.5 font-medium hover:border-sky-500 hover:text-sky-500 dark:border-zinc-700"
          >
            {t("signIn")}
          </Link>
        </div>
      </div>

      <div className="mt-8">
        <TrendingSection />
      </div>
    </main>
  );
}
