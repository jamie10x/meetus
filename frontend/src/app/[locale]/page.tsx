import { getTranslations } from "next-intl/server";
import { Link } from "@/i18n/navigation";
import TrendingSection from "@/components/TrendingSection";
import TicketPreview from "@/components/TicketPreview";
import { API_URL } from "@/lib/api";
import { fetchTrending } from "@/lib/server-api";
import type { MetaItem } from "@/lib/types";

async function fetchCityCount(): Promise<number> {
  try {
    const res = await fetch(`${API_URL}/api/meta/cities`, { cache: "no-store" });
    if (!res.ok) return 0;
    const body = await res.json();
    return (body.data as MetaItem[]).length;
  } catch {
    return 0;
  }
}

export default async function HomePage() {
  const t = await getTranslations("home");
  const [cityCount, [featured]] = await Promise.all([
    fetchCityCount(),
    fetchTrending(1),
  ]);

  return (
    <main>
      <section className="relative overflow-hidden py-14 sm:py-20">
        <div
          className="pointer-events-none absolute -inset-x-[10%] -top-[20%] -z-10 h-[640px] blur-sm"
          style={{
            background:
              "radial-gradient(480px 380px at 12% 20%, rgba(24,173,160,0.24), transparent 65%)," +
              "radial-gradient(420px 340px at 88% 10%, rgba(242,167,59,0.16), transparent 65%)",
          }}
        />
        <div className="mx-auto grid max-w-6xl grid-cols-1 items-center gap-12 px-5 lg:grid-cols-[1.15fr_0.85fr]">
          <div>
            <p className="flex items-center gap-2 font-mono text-xs font-medium uppercase tracking-[0.14em] text-registan-strong">
              <span className="h-1.5 w-1.5 rounded-full bg-registan-strong shadow-[0_0_0_3px_rgba(24,173,160,0.16)]" />
              {t("eyebrow")}
            </p>
            <h1 className="mt-4 text-[clamp(2.5rem,4.6vw+1rem,4.35rem)] font-black leading-[0.98] tracking-tight text-bone">
              {t("titlePart1")}{" "}
              <em className="not-italic italic text-registan-strong">
                {t("titleHighlight")}
              </em>
            </h1>
            <p className="mt-5 max-w-[46ch] text-lg leading-relaxed text-dust">
              {t("subtitle")}
            </p>
            <div className="mt-8 flex flex-wrap gap-3.5">
              <Link
                href="/events"
                className="rounded-full bg-registan px-6 py-3 text-base font-bold text-[#0A2320] shadow-[0_8px_22px_-8px_rgba(24,173,160,0.55)] transition-colors hover:bg-registan-strong"
              >
                {t("exploreEvents")}
              </Link>
              <Link
                href="/login"
                className="rounded-full border border-line px-6 py-3 text-base font-bold text-bone transition-colors hover:border-registan-strong hover:text-registan-strong"
              >
                {t("signIn")}
              </Link>
            </div>
            <div className="mt-10 flex flex-wrap gap-8 border-t border-bone/[0.09] pt-7">
              {cityCount > 0 ? (
                <div>
                  <p className="font-mono text-2xl font-medium tabular-nums text-bone">
                    {cityCount}+
                  </p>
                  <p className="mt-0.5 text-xs text-dust-dim">
                    {t("statCities", { count: cityCount })}
                  </p>
                </div>
              ) : null}
              <div>
                <p className="font-mono text-2xl font-medium text-bone">🎟️</p>
                <p className="mt-0.5 text-xs text-dust-dim">{t("statFree")}</p>
              </div>
            </div>
          </div>

          {featured ? (
            <TicketPreview event={featured} />
          ) : null}
        </div>
      </section>

      <div className="mx-auto max-w-6xl px-5">
        <TrendingSection />

        <section className="mb-16 border-t border-bone/[0.09] pt-10">
          <h2 className="text-2xl font-extrabold text-bone sm:text-3xl">
            {t("howItWorksHeading")}
          </h2>
          <div className="mt-6 grid grid-cols-1 gap-px overflow-hidden rounded-2xl border border-line bg-line sm:grid-cols-3">
            {[1, 2, 3].map((n) => (
              <div key={n} className="bg-ink-raised p-7">
                <p className="font-mono text-xs text-registan-strong">
                  0{n}
                </p>
                <h3 className="mt-2.5 text-lg font-bold text-bone">
                  {t(`step${n}Title`)}
                </h3>
                <p className="mt-2 text-sm leading-relaxed text-dust">
                  {t(`step${n}Body`)}
                </p>
              </div>
            ))}
          </div>
        </section>
      </div>
    </main>
  );
}
