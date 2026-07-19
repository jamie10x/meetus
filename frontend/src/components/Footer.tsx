import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";

const CITIES = [
  "Tashkent",
  "Samarkand",
  "Bukhara",
  "Andijan",
  "Namangan",
  "Fergana",
  "Khiva",
];

export default function Footer() {
  const t = useTranslations("footer");
  const year = new Date().getFullYear();

  return (
    <footer className="relative z-[1] border-t border-bone/[0.09]">
      <div className="mx-auto max-w-6xl px-5 py-10">
        <div className="grid grid-cols-2 gap-10 sm:grid-cols-4">
          <div className="col-span-2">
            <Link
              href="/"
              className="flex items-center whitespace-nowrap font-display text-lg font-black italic text-bone"
            >
              <span className="mr-1.5 not-italic text-registan-strong">✳</span>
              Meetus
            </Link>
            <p className="mt-3 max-w-xs text-sm text-dust">{t("tagline")}</p>
            <div className="mt-5 flex flex-wrap gap-x-3 gap-y-1.5">
              {CITIES.map((city) => (
                <span key={city} className="font-mono text-xs text-dust-dim">
                  {city}
                </span>
              ))}
            </div>
          </div>

          <div>
            <h2 className="font-mono text-xs uppercase tracking-wider text-dust-dim">
              {t("exploreHeading")}
            </h2>
            <ul className="mt-4 flex flex-col gap-2.5 text-sm">
              <li>
                <Link href="/events" className="text-dust transition-colors hover:text-bone">
                  {t("exploreLink")}
                </Link>
              </li>
              <li>
                <Link href="/tickets" className="text-dust transition-colors hover:text-bone">
                  {t("ticketsLink")}
                </Link>
              </li>
              <li>
                <Link
                  href="/organizer"
                  className="text-dust transition-colors hover:text-bone"
                >
                  {t("organizeLink")}
                </Link>
              </li>
            </ul>
          </div>

          <div>
            <h2 className="font-mono text-xs uppercase tracking-wider text-dust-dim">
              {t("legalHeading")}
            </h2>
            <ul className="mt-4 flex flex-col gap-2.5 text-sm">
              <li>
                <Link href="/privacy" className="text-dust transition-colors hover:text-bone">
                  {t("privacyLink")}
                </Link>
              </li>
              <li>
                <a
                  href="https://t.me/meetusuz_bot"
                  target="_blank"
                  rel="noreferrer"
                  className="text-dust transition-colors hover:text-bone"
                >
                  {t("botLink")}
                </a>
              </li>
            </ul>
          </div>
        </div>

        <div className="mt-10 flex flex-col-reverse items-start gap-3 border-t border-bone/[0.09] pt-6 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-xs text-dust-dim">{t("copyright", { year })}</p>
          <p className="font-mono text-xs text-dust-dim">{t("builtFor")}</p>
        </div>
      </div>
    </footer>
  );
}
