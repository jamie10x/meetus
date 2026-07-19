import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";

type Props = { params: Promise<{ locale: string }> };

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { locale } = await params;
  const t = await getTranslations({ locale, namespace: "privacy" });
  return { title: t("title") };
}

type Section = { heading: string; body: string };

export default async function PrivacyPage({ params }: Props) {
  const { locale } = await params;
  const t = await getTranslations({ locale, namespace: "privacy" });
  const sections = t.raw("sections") as Section[];

  return (
    <main className="mx-auto max-w-3xl px-5 py-16">
      <p className="font-mono text-xs font-medium uppercase tracking-[0.14em] text-registan-strong">
        {t("updated")}
      </p>
      <h1 className="mt-4 font-display text-4xl font-black tracking-tight text-bone sm:text-5xl">
        {t("title")}
      </h1>
      <p className="mt-6 max-w-2xl text-lg leading-relaxed text-dust">{t("intro")}</p>

      <div className="mt-12 flex flex-col gap-10 border-t border-bone/[0.09] pt-10">
        {sections.map((section, i) => (
          <section key={i}>
            <h2 className="font-display text-xl font-bold text-bone">{section.heading}</h2>
            <p className="mt-3 leading-relaxed text-dust">{section.body}</p>
          </section>
        ))}
      </div>

      <section className="mt-10 rounded-card border border-line bg-ink-raised p-6">
        <h2 className="font-display text-xl font-bold text-bone">{t("contactHeading")}</h2>
        <p className="mt-3 leading-relaxed text-dust">{t("contactBody")}</p>
      </section>
    </main>
  );
}
